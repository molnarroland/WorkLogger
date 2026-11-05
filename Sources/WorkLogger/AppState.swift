import EventKit
import Foundation
import SwiftUI
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

@MainActor
@Observable
final class AppState {
    let db: Database
    var settings: AppSettings {
        didSet {
            guard settings != oldValue else { return }
            try? db.saveSettings(settings)
            if settings.excludedFolders != oldValue.excludedFolders { removeHiddenRepos() }
            if settings.modules != oldValue.modules || settings.chatGapMinutes != oldValue.chatGapMinutes { reloadRange() }
        }
    }

    var from: Date
    var to: Date
    var worklogs: [Worklog] = [] {
        didSet { updateTempoMatch() }
    }
    /// For the status bar: entries without a ticket, and ticket-days already in Tempo. Kept up to date with
    /// `worklogs` instead of being recomputed on every redraw.
    private(set) var tempoProgress: (missing: Int, entered: Int, total: Int) = (0, 0, 0)
    /// Your worklogs already in Tempo for the range (read when a Tempo token is set; empty otherwise).
    var tempoLogs: [TempoWorklog] = [] {
        didSet { updateTempoMatch() }
    }
    /// A Tempo token is set and the last read worked.
    var tempoConnected = false
    /// "KEY|day" ticket-days whose time Tempo already has.
    private(set) var tempoCovered: Set<String> = []
    var repos: [Repo] = []
    var projects: [ProjectMapping] = []
    var meetings: [Meeting] = []
    var sessions: [WorkSession] = []
    var dayRecords: [String: DayRecord] = [:]
    var dirty: Set<Int64> = []
    var status = String(localized: "Ready")
    var chats: [ChatDay] = []
    /// Commits per repository in the range, for the Repositories list.
    var commitCounts: [Int64: Int] = [:]
    /// Set when a background fetch found the site signed out: the window opens for sign-in and the fetch resumes.
    var signInNeeded: WebSession.Source?
    /// A page the menu bar item asked the main window to show.
    var requestedPane: Pane?
    /// Show only days with missing reports (Review lists them, the Timesheet dims the rest).
    var onlyMissingDays = false
    var isBusy = false
    var googleConnected: Bool
    var calendarAccess: MacCalendarStore.Access = .notDetermined
    var macCalendars: [MacCalendarInfo] = []

    @ObservationIgnored let services: AppDependencies
    @ObservationIgnored var googleAccount: (any GoogleCalendarAccount)?
    @ObservationIgnored private var calendarObserver: NSObjectProtocol?

    init(db: Database, services: AppDependencies) {
        self.db = db
        self.services = services
        var loaded = db.loadSettings()
        let added = Set(Module.allCases).subtracting(loaded.knownModules)
        if !added.isEmpty {  // modules added in a later version start switched on
            loaded.modules.formUnion(added)
            loaded.knownModules = Set(Module.allCases)
        }
        settings = loaded
        googleConnected = services.secrets.get(SecretKey.googleRefresh) != nil
        let month = DateRange.month(containing: services.now())!
        from = month.from
        to = month.to
        moveLegacyTokens()
        reloadAll()
        refreshCalendars()
        // New accounts and calendars sync in the background; refresh the list when macOS reports a change.
        calendarObserver = NotificationCenter.default.addObserver(forName: .EKEventStoreChanged, object: nil, queue: .main) {
            [weak self] _ in
            MainActor.assumeIsolated { self?.refreshCalendars() }
        }
    }

    /// The app's state: the database in Application Support (or `databaseURL`, e.g. a throwaway file for
    /// previews) and the real system services.
    convenience init(databaseURL: URL? = nil) {
        let dir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("WorkLogger", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let database: Database
        do {
            var url = databaseURL ?? dir.appendingPathComponent("worklogs.sqlite")
            #if DEBUG
                if databaseURL == nil, let path = UserDefaults.standard.string(forKey: "WLDatabase") {
                    url = URL(fileURLWithPath: path)
                }
            #endif
            database = try Database(url: url)
        } catch {
            fatalError("Cannot open the database: \(error.localizedDescription)")
        }
        self.init(db: database, services: .live)
    }

    // MARK: Range

    var range: DateRange { DateRange(from: from, to: to) }
    var fromDay: String { range.fromDay }
    var toDay: String { range.toDay }
    var rangeStart: Date { range.start }
    var rangeEnd: Date { range.end }

    func setRange(_ r: QuickRange) {
        guard let new = DateRange.quick(r, now: services.now()) else { return }
        apply(new)
    }

    /// The toolbar arrows: by months for a whole month, otherwise by the range's length.
    func shiftRange(_ delta: Int) {
        guard let new = range.shifted(by: delta) else { return }
        apply(new)
    }

    /// Sets the range picked in the toolbar calendar.
    func setRange(from: Date, to: Date) {
        apply(DateRange(from: min(from, to), to: max(from, to)))
    }

    /// Workdays up to today that are short of their expected hours or have entries without a ticket
    /// (the "Only days with missing reports" preset).
    var missingDays: Set<String> {
        let today = DayKey.key(services.now())
        let builder = makeBuilder()
        let byDay = Dictionary(grouping: worklogs, by: \.day)
        return Set(
            range.days.filter { day in
                guard day <= today else { return false }
                let logs = byDay[day] ?? []
                if logs.contains(where: \.needsIssue) { return true }
                let expected =
                    target(day: day, builder: builder)
                    ?? (builder.isWorkday(day, info: dayRecords[day]) ? Int(settings.fixedDailyHours * 3600) : 0)
                return expected > 0 && logs.totalDuration < expected
            })
    }

    /// Moves the range by whole months (the timesheet's ‹ › buttons).
    func shiftMonth(_ delta: Int) {
        guard let new = range.shiftedByMonths(delta) else { return }
        apply(new)
    }

    private func apply(_ new: DateRange) {
        from = new.from
        to = new.to
        reloadRange()
    }

    // MARK: Loading

    func reloadAll() {
        attempt("Loading") {
            repos = try db.repos()
            projects = try db.projects()
        }
        reloadRange()
    }

    func reloadRange() {
        attempt("Loading") {
            worklogs = try db.worklogs(fromDay: fromDay, toDay: toDay)
            let on = settings.modules
            chats = try db.chatDays(from: rangeStart, to: rangeEnd, gap: TimeInterval(settings.chatGapMinutes * 60))
            commitCounts = try db.commitCounts(from: rangeStart, to: rangeEnd)
            meetings = on.contains(.calendar) ? try db.meetings(from: rangeStart, to: rangeEnd) : []
            sessions = on.contains(.workTimes) ? try db.sessions(fromDay: fromDay, toDay: toDay) : []
            dayRecords =
                on.contains(.workTimes)
                ? Dictionary(uniqueKeysWithValues: try db.dayRecords(fromDay: fromDay, toDay: toDay).map { ($0.day, $0) }) : [:]
        }
        dirty.removeAll()
    }

    var days: [String] { Array(Set(worklogs.map(\.day))).sorted() }

    func total(day: String) -> Int {
        worklogs.filter { $0.day == day }.totalDuration
    }

    func makeBuilder() -> WorklogBuilder {
        WorklogBuilder(
            settings: settings,
            repos: Dictionary(uniqueKeysWithValues: repos.map { ($0.id, $0) }),
            projects: Dictionary(uniqueKeysWithValues: projects.map { ($0.key, $0) }))
    }

    /// The hours a day should add up to under the current rounding mode, if any.
    func target(day: String, builder: WorklogBuilder? = nil) -> Int? {
        let builder = builder ?? makeBuilder()
        let input = DayInput(
            day: day, sessions: sessions.filter { $0.day == day }, commits: [], meetings: [],
            kept: [], info: dayRecords[day])
        let (windows, fromSessions) = builder.workWindows(input)
        return builder.dailyTarget(day: day, info: dayRecords[day], windows: windows, fromSessions: fromSessions)
    }

    func isWorkday(_ day: String) -> Bool { makeBuilder().isWorkday(day, info: dayRecords[day]) }

    /// What the timesheet expects for a day: the rounding target, or the hours per day on a workday.
    func expected(day: String) -> Int {
        if let t = target(day: day) { return t }
        return isWorkday(day) ? Int(settings.fixedDailyHours * 3600) : 0
    }

    // MARK: Helpers

    var databasePath: String { db.path }

    /// Which ticket-days Tempo already has, and the status-bar counts that depend on it.
    private func updateTempoMatch() {
        tempoCovered = TempoWorklog.coverage(of: worklogs, in: tempoLogs)
        tempoProgress = Self.progress(of: worklogs, inTempo: tempoCovered)
    }

    /// Runs a database step; a failure ends up in the status bar.
    func attempt(_ label: String.LocalizationValue, _ body: () throws -> Void) {
        do { try body() } catch {
            let step = String(localized: label)
            status = String(localized: "\(step) failed: \(error.localizedDescription)")
            Log.app.error("\(step, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
        }
    }
}
