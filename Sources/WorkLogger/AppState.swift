import Foundation
import SwiftUI
import UniformTypeIdentifiers
import EventKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
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
            if settings.modules != oldValue.modules { reloadRange() }
        }
    }

    var from: Date
    var to: Date
    var worklogs: [Worklog] = [] {
        didSet { tempoProgress = Self.progress(of: worklogs) }
    }
    /// For the status bar: entries without a ticket, and ticket-days already in Tempo. Kept up to date with
    /// `worklogs` instead of being recomputed on every redraw.
    private(set) var tempoProgress: (missing: Int, entered: Int, total: Int) = (0, 0, 0)
    var repos: [Repo] = []
    var projects: [ProjectMapping] = []
    var meetings: [Meeting] = []
    var sessions: [WorkSession] = []
    var dayRecords: [String: DayRecord] = [:]
    var dirty: Set<Int64> = []
    var status = "Ready"
    var chats: [ChatDay] = []
    /// Commits per repository in the range, for the Repositories list.
    var commitCounts: [Int64: Int] = [:]
    /// Set when a background fetch found the site signed out: the window opens for sign-in and the fetch resumes.
    var signInNeeded: WebSession.Source?
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
        if !added.isEmpty {   // modules added in a later version start switched on
            loaded.modules.formUnion(added)
            loaded.knownModules = Set(Module.allCases)
        }
        settings = loaded
        googleConnected = services.secrets.get(SecretKey.googleRefresh) != nil
        let month = DateRange.month(containing: services.now())!
        from = month.from
        to = month.to
        reloadAll()
        refreshCalendars()
        // New accounts and calendars sync in the background; refresh the list when macOS reports a change.
        calendarObserver = NotificationCenter.default.addObserver(forName: .EKEventStoreChanged, object: nil, queue: .main) { [weak self] _ in
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
            if databaseURL == nil, let path = UserDefaults.standard.string(forKey: "WLDatabase") { url = URL(fileURLWithPath: path) }
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
            chats = try db.chatDays(from: rangeStart, to: rangeEnd)
            commitCounts = try db.commitCounts(from: rangeStart, to: rangeEnd)
            meetings = on.contains(.calendar) ? try db.meetings(from: rangeStart, to: rangeEnd) : []
            sessions = on.contains(.workTimes) ? try db.sessions(fromDay: fromDay, toDay: toDay) : []
            dayRecords = on.contains(.workTimes)
                ? Dictionary(uniqueKeysWithValues: try db.dayRecords(fromDay: fromDay, toDay: toDay).map { ($0.day, $0) }) : [:]
        }
        dirty.removeAll()
    }

    var days: [String] { Array(Set(worklogs.map(\.day))).sorted() }

    func total(day: String) -> Int {
        worklogs.filter { $0.day == day }.totalDuration
    }

    func makeBuilder() -> WorklogBuilder {
        WorklogBuilder(settings: settings,
                       repos: Dictionary(uniqueKeysWithValues: repos.map { ($0.id, $0) }),
                       projects: Dictionary(uniqueKeysWithValues: projects.map { ($0.key, $0) }))
    }

    /// The hours a day should add up to under the current rounding mode, if any.
    func target(day: String, builder: WorklogBuilder? = nil) -> Int? {
        let builder = builder ?? makeBuilder()
        let input = DayInput(day: day, sessions: sessions.filter { $0.day == day }, commits: [], meetings: [],
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

    /// Runs a database step; a failure ends up in the status bar.
    func attempt(_ label: String, _ body: () throws -> Void) {
        do { try body() } catch { status = "\(label) failed: \(error.localizedDescription)" }
    }
}
