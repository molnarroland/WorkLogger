import Foundation
import SwiftUI
import AppKit
import UniformTypeIdentifiers
import EventKit

enum QuickRange: String, CaseIterable {
    case today = "Today", thisWeek = "This week", lastWeek = "Last week", thisMonth = "This month", lastMonth = "Last month"
}

@MainActor
@Observable
final class AppState {
    let db: Database
    var settings: AppSettings {
        didSet {
            guard settings != oldValue else { return }
            try? db.saveSettings(settings)
            if settings.excludedFolders != oldValue.excludedFolders { removeHiddenRepos() }
        }
    }

    var from: Date
    var to: Date
    var worklogs: [Worklog] = []
    var repos: [Repo] = []
    var projects: [ProjectMapping] = []
    var meetings: [Meeting] = []
    var sessions: [WorkSession] = []
    var dayRecords: [String: DayRecord] = [:]
    var dirty: Set<Int64> = []
    var status = "Ready"
    var isBusy = false
    var googleConnected = GoogleAuth.isSignedIn
    var calendarAccess: MacCalendarStore.Access = .notDetermined
    var macCalendars: [MacCalendarInfo] = []

    @ObservationIgnored private var googleAuth: GoogleAuth?
    @ObservationIgnored let macCalendarStore = MacCalendarStore()
    @ObservationIgnored private var calendarObserver: NSObjectProtocol?

    /// `databaseURL` defaults to Application Support; previews pass a throwaway file.
    init(databaseURL: URL? = nil) {
        let dir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("WorkLogger", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let database: Database
        do {
            database = try Database(url: databaseURL ?? dir.appendingPathComponent("worklogs.sqlite"))
        } catch {
            fatalError("Cannot open the database: \(error.localizedDescription)")
        }
        db = database
        settings = database.loadSettings()
        let month = DayKey.calendar.dateInterval(of: .month, for: Date())!
        from = month.start
        to = month.end.addingTimeInterval(-1)
        reloadAll()
        refreshCalendars()
        // New accounts and calendars sync in the background; refresh the list when macOS reports a change.
        calendarObserver = NotificationCenter.default.addObserver(forName: .EKEventStoreChanged, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.refreshCalendars() }
        }
    }

    // MARK: Range

    var fromDay: String { DayKey.key(min(from, to)) }
    var toDay: String { DayKey.key(max(from, to)) }
    var rangeStart: Date { DayKey.startOfDay(min(from, to)) }
    var rangeEnd: Date { DayKey.dayAfter(max(from, to)) }

    func setRange(_ r: QuickRange) {
        let cal = DayKey.calendar
        let now = Date()
        let interval: DateInterval?
        switch r {
        case .today: interval = cal.dateInterval(of: .day, for: now)
        case .thisWeek: interval = cal.dateInterval(of: .weekOfYear, for: now)
        case .lastWeek: interval = cal.dateInterval(of: .weekOfYear, for: cal.date(byAdding: .day, value: -7, to: now)!)
        case .thisMonth: interval = cal.dateInterval(of: .month, for: now)
        case .lastMonth: interval = cal.dateInterval(of: .month, for: cal.date(byAdding: .month, value: -1, to: now)!)
        }
        guard let interval else { return }
        from = interval.start
        to = interval.end.addingTimeInterval(-1)
        reloadRange()
    }

    /// Moves the range by whole months (the timesheet's ‹ › buttons).
    func shiftMonth(_ delta: Int) {
        let cal = DayKey.calendar
        guard let month = cal.dateInterval(of: .month, for: cal.date(byAdding: .month, value: delta, to: rangeStart)!) else { return }
        from = month.start
        to = month.end.addingTimeInterval(-1)
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
            meetings = try db.meetings(from: rangeStart, to: rangeEnd)
            sessions = try db.sessions(fromDay: fromDay, toDay: toDay)
            dayRecords = Dictionary(uniqueKeysWithValues: try db.dayRecords(fromDay: fromDay, toDay: toDay).map { ($0.day, $0) })
        }
        dirty.removeAll()
    }

    var days: [String] { Array(Set(worklogs.map(\.day))).sorted() }

    func total(day: String) -> Int {
        worklogs.filter { $0.day == day }.reduce(0) { $0 + $1.duration }
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

    // MARK: Collect

    /// Reads git history and the calendar for the selected range, then rebuilds the drafts.
    func collect() async {
        guard !isBusy else { return }
        isBusy = true
        defer { isBusy = false }

        var authors = settings.authorEmails
        if authors.isEmpty, let email = GitScanner.globalUserEmail() {
            authors = [email]
            settings.authorEmails = [email]
        }
        guard !authors.isEmpty else {
            status = "Add your commit email address under Repositories so only your commits are counted."
            return
        }

        let enabled = repos.filter(\.enabled)
        let start = rangeStart, end = rangeEnd
        let ignored = settings.ignoredPathPatterns
        status = "Reading git history of \(enabled.count) repositories…"
        let (results, errors) = await Task.detached(priority: .userInitiated) { () -> ([(Int64, [GitCommit])], [String]) in
            var ok: [(Int64, [GitCommit])] = []
            var failed: [String] = []
            for repo in enabled {
                do {
                    ok.append((repo.id, try GitLogReader.log(repoPath: repo.path, since: start, until: end, authors: authors)))
                } catch {
                    failed.append("\(repo.name): \(error.localizedDescription)")
                }
            }
            return (ok, failed)
        }.value

        var commitCount = 0
        attempt("Saving commits") {
            try db.transaction {
                for (repoId, commits) in results {
                    commitCount += commits.count
                    try db.saveCommits(commits.map { StoredCommit($0, repoId: repoId, ignoredPaths: ignored) })
                }
            }
        }

        var meetingCount = 0
        if canReadMeetings {
            status = "Reading meetings from Google Calendar…"
            do { meetingCount = try await fetchMeetings() } catch { status = "Calendar: \(error.localizedDescription)" }
        }

        rebuild()
        var summary = "Collected \(commitCount) commits"
        if canReadMeetings { summary += " and \(meetingCount) meetings" }
        summary += " for \(fromDay) – \(toDay)."
        if !errors.isEmpty { summary += " Skipped: " + errors.joined(separator: "; ") }
        status = summary
    }

    /// Rebuilds unlocked drafts from stored commits, meetings and sessions (no network or git access).
    func rebuild() {
        saveDirty()
        let builder = makeBuilder()
        attempt("Building worklogs") {
            try db.transaction {
                try db.deleteRegeneratable(fromDay: fromDay, toDay: toDay)
                // Commits made on leave days and on rest days without work sessions go to the closest workday,
                // which may lie just outside the range.
                let cal = DayKey.calendar
                func shift(_ d: Date, _ n: Int) -> Date { cal.date(byAdding: .day, value: n, to: d)! }
                let padFrom = shift(rangeStart, -31), padTo = shift(rangeEnd, 31)
                // Twice the padding, so the closest-workday search from a padded leave day sees real day types.
                let records = Dictionary(uniqueKeysWithValues: try db.dayRecords(
                    fromDay: DayKey.key(shift(rangeStart, -62)), toDay: DayKey.key(shift(rangeEnd, 62))).map { ($0.day, $0) })
                let workedDays = Set(try db.sessions(fromDay: DayKey.key(padFrom), toDay: DayKey.key(padTo))
                    .filter { $0.kind == .work }.map(\.day))
                let rangeDays = DayKey.days(from: from, to: to)
                let inRange = Set(rangeDays)
                var commitsByDay: [String: [StoredCommit]] = [:]
                for day in DayKey.days(from: padFrom, to: padTo) {
                    let target = builder.commitDay(for: day, info: { records[$0] }, worked: { workedDays.contains($0) })
                    guard inRange.contains(day) || target != day else { continue }
                    commitsByDay[target, default: []] += try db.commits(day: day)
                }
                // Entries that survived the delete (locked, moved, reviewed) own their commits wherever they now sit.
                let claimed = Set(try db.worklogs(fromDay: DayKey.key(padFrom), toDay: DayKey.key(padTo)).flatMap(\.refs))
                for (day, list) in commitsByDay { commitsByDay[day] = list.filter { !claimed.contains($0.hash) } }
                for day in rangeDays {
                    let input = DayInput(day: day, sessions: try db.sessions(day: day), commits: commitsByDay[day] ?? [],
                                         meetings: try db.meetings(day: day).filter { !claimed.contains($0.id) },
                                         kept: try db.worklogs(day: day),
                                         info: try db.dayRecord(day: day))
                    for w in builder.build(input) { try db.insertWorklog(w) }
                }
            }
            status = "Drafts rebuilt for \(fromDay) – \(toDay)."
        }
        reloadRange()
    }

    // MARK: Editing

    func markDirty(_ id: Int64) { dirty.insert(id) }

    func saveDirty() {
        guard !dirty.isEmpty else { return }
        attempt("Saving") {
            try db.transaction {
                for w in worklogs where dirty.contains(w.id) { try db.updateWorklog(w) }
            }
            status = "Saved \(dirty.count) changes."
        }
        dirty.removeAll()
    }

    func delete(_ w: Worklog) {
        attempt("Deleting") { try db.deleteWorklog(w.id) }
        worklogs.removeAll { $0.id == w.id }
        dirty.remove(w.id)
    }

    func addManual(day: String) {
        saveDirty()
        let lastEnd = worklogs.filter { $0.day == day }.map(\.end).max()
        let start = lastEnd ?? DayKey.at(day, hm: settings.defaultDayStart) ?? DayKey.date(day)
        let w = Worklog(id: 0, day: day, start: start, duration: 30 * 60, issueKey: nil, description: "",
                        source: .manual, sourceRef: "", repoId: nil, estimated: 0, status: .draft, locked: true, tempoId: nil)
        attempt("Adding") { try db.insertWorklog(w) }
        reloadRange()
    }

    /// Drag & drop on the timesheet: moves entries to another day, after its last entry, and locks them so a rebuild keeps them.
    func move(_ ids: Set<Int64>, to day: String) {
        saveDirty()
        var cursor = worklogs.filter { $0.day == day }.map(\.end).max()
            ?? DayKey.at(day, hm: settings.defaultDayStart) ?? DayKey.date(day)
        var moved = 0
        attempt("Moving") {
            try db.transaction {
                for var w in worklogs where ids.contains(w.id) && w.day != day && w.status != .exported {
                    w.day = day
                    w.start = cursor
                    w.locked = true
                    cursor = w.end
                    try db.updateWorklog(w)
                    moved += 1
                }
            }
            status = "Moved \(moved) \(moved == 1 ? "entry" : "entries") to \(DayKey.title(day))."
        }
        reloadRange()
    }

    /// A day can take dropped entries when it is a workday or has imported work sessions.
    func acceptsWork(_ day: String) -> Bool {
        isWorkday(day) || sessions.contains { $0.day == day && $0.kind == .work }
    }

    func setStatus(_ status: WorklogStatus, forAllIn day: String) {
        for i in worklogs.indices where worklogs[i].day == day && worklogs[i].status != .exported {
            worklogs[i].status = status
            worklogs[i].locked = true
            dirty.insert(worklogs[i].id)
        }
        saveDirty()
    }

    // MARK: Repositories

    func chooseWorkspace() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.prompt = "Use folder"
        if panel.runModal() == .OK, let url = panel.url {
            settings.workspaceRoot = url.path
            scanWorkspace()
        }
    }

    func scanWorkspace() {
        guard !settings.workspaceRoot.isEmpty else { status = "Choose your workspace folder first."; return }
        let root = URL(fileURLWithPath: (settings.workspaceRoot as NSString).expandingTildeInPath)
        let found = GitScanner.findRepos(root: root, excluded: settings.excludedFolders)
        attempt("Scanning") {
            for url in found { try db.addRepo(path: url.path, name: url.lastPathComponent) }
        }
        reloadAll()
        removeHiddenRepos()
        status = "Found \(found.count) repositories in \(root.lastPathComponent)."
    }

    /// Drops listed repositories that sit inside a hidden folder.
    func removeHiddenRepos() {
        let hidden = repos.filter { GitScanner.isExcluded($0.path, by: settings.excludedFolders) }
        guard !hidden.isEmpty else { return }
        attempt("Hiding repositories") {
            for repo in hidden { try db.deleteRepo(repo.id) }
            repos = try db.repos()
        }
    }

    func update(_ repo: Repo) {
        var r = repo
        r.issueKey = JiraKey.normalize(r.issueKey)
        r.projectKey = JiraKey.normalize(r.projectKey)
        attempt("Saving repository") { try db.updateRepo(r) }
    }

    func removeRepo(_ repo: Repo) {
        attempt("Removing repository") { try db.deleteRepo(repo.id) }
        reloadAll()
    }

    func saveProject(_ p: ProjectMapping) {
        guard let key = JiraKey.normalize(p.key) else { return }
        attempt("Saving project") { try db.upsertProject(ProjectMapping(key: key, defaultIssue: JiraKey.normalize(p.defaultIssue))) }
        projects = (try? db.projects()) ?? projects
    }

    func removeProject(_ p: ProjectMapping) {
        attempt("Removing project") { try db.deleteProject(p.key) }
        projects = (try? db.projects()) ?? projects
    }

    // MARK: Work sessions

    func importTimesheet(_ t: ParsedTimesheet) {
        attempt("Importing work sessions") { try db.importTimesheet(t) }
        status = "Imported \(t.sessions.count) sessions and breaks for \(t.dayKeys.count) days. Rebuild drafts to apply them."
        reloadRange()
    }

    func clearSessionsInRange() {
        attempt("Clearing sessions") { try db.deleteSessions(fromDay: fromDay, toDay: toDay) }
        reloadRange()
    }

    // MARK: Google Calendar

    private func auth() -> GoogleAuth {
        let secret = Keychain.get(SecretKey.googleSecret) ?? ""
        if let a = googleAuth, a.clientId == settings.googleClientId, a.clientSecret == secret { return a }
        let a = GoogleAuth(clientId: settings.googleClientId, clientSecret: secret)
        googleAuth = a
        return a
    }

    func connectGoogle() async {
        isBusy = true
        defer { isBusy = false }
        status = "Finish signing in to Google in your browser…"
        do {
            try await auth().signIn()
            googleConnected = true
            status = "Google Calendar connected."
        } catch {
            status = error.localizedDescription
        }
    }

    func disconnectGoogle() {
        auth().signOut()
        googleConnected = false
        status = "Google Calendar disconnected."
    }

    // MARK: macOS calendars (Google via Internet Accounts)

    var canReadMeetings: Bool {
        googleConnected || (calendarAccess == .granted && !settings.macCalendarIds.isEmpty)
    }

    private var googleApiCalendarId: String {
        settings.googleCalendarId.isEmpty ? "primary" : settings.googleCalendarId
    }

    func refreshCalendars() {
        calendarAccess = macCalendarStore.access
        macCalendars = calendarAccess == .granted ? macCalendarStore.calendars() : []
    }

    func signInWithGoogle() {
        MacCalendarStore.openInternetAccounts()
        status = "In Internet Accounts choose Add Account… → Google, sign in and keep Calendars switched on."
    }

    func requestCalendarAccess() async {
        do {
            let granted = try await macCalendarStore.requestAccess()
            refreshCalendars()
            status = granted ? "Calendar access allowed. Tick the calendars to read."
                             : "Calendar access was not allowed. You can change this in System Settings → Privacy & Security → Calendars."
        } catch {
            status = "Calendar access failed: \(error.localizedDescription)"
        }
    }

    func isCalendarSelected(_ id: String) -> Bool { settings.macCalendarIds.contains(id) }

    func setCalendar(_ id: String, selected: Bool) {
        if selected {
            if !settings.macCalendarIds.contains(id) { settings.macCalendarIds.append(id) }
        } else {
            settings.macCalendarIds.removeAll { $0 == id }
        }
    }

    /// Reads meetings from the selected macOS calendars and, if connected, the Google Calendar API.
    @discardableResult
    func fetchMeetings() async throws -> Int {
        let macIds = calendarAccess == .granted ? Set(settings.macCalendarIds) : []
        var active = macIds
        if googleConnected { active.insert(googleApiCalendarId) }

        var count = 0
        let macEvents = macCalendarStore.meetings(calendarIds: macIds, from: rangeStart, to: rangeEnd)
        try db.syncMeetings(from: rangeStart, to: rangeEnd, calendarIds: macIds, activeCalendarIds: active, events: macEvents)
        count += macEvents.count

        if googleConnected {
            let events = try await GoogleCalendarClient(auth: auth())
                .meetings(calendarId: googleApiCalendarId, from: rangeStart, to: rangeEnd)
            try db.syncMeetings(from: rangeStart, to: rangeEnd, calendarIds: [googleApiCalendarId],
                                activeCalendarIds: active, events: events)
            count += events.count
        }
        meetings = try db.meetings(from: rangeStart, to: rangeEnd)
        return count
    }

    func refreshMeetings() async {
        isBusy = true
        defer { isBusy = false }
        do {
            let n = try await fetchMeetings()
            status = "Read \(n) meetings. Rebuild drafts to apply them."
        } catch {
            status = "Calendar: \(error.localizedDescription)"
        }
    }

    func setSkipped(_ meeting: Meeting, _ skipped: Bool) {
        attempt("Saving meeting") { try db.setMeetingSkipped(meeting.id, skipped) }
        if let i = meetings.firstIndex(where: { $0.id == meeting.id }) { meetings[i].skipped = skipped }
    }

    // MARK: Export

    func exportCSV(_ logs: [Worklog]) {
        save(TempoExport.csv(logs).data(using: .utf8) ?? Data(), name: "tempo-\(fromDay)_\(toDay).csv", type: .commaSeparatedText)
    }

    func exportJSON(_ logs: [Worklog]) {
        attempt("Exporting") {
            save(try TempoExport.json(logs, accountId: settings.jiraAccountId), name: "tempo-\(fromDay)_\(toDay).json", type: .json)
        }
    }

    func markdown(_ logs: [Worklog]) -> String {
        TempoExport.markdown(logs, from: fromDay, to: toDay, jiraBaseURL: settings.jiraBaseURL)
    }

    func exportMarkdown(_ logs: [Worklog]) {
        save(Data(markdown(logs).utf8), name: "tempo-\(fromDay)_\(toDay).md",
             type: UTType(filenameExtension: "md") ?? .plainText)
    }

    func copyMarkdown(_ logs: [Worklog]) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(markdown(logs), forType: .string)
        status = "Copied the Tempo checklist for \(TempoExport.exportable(logs).count) worklogs. Paste it to Claude with Claude in Chrome."
    }

    private func save(_ data: Data, name: String, type: UTType) {
        let panel = NSSavePanel()
        panel.nameFieldStringValue = name
        panel.allowedContentTypes = [type]
        guard panel.runModal() == .OK, let url = panel.url else { return }
        attempt("Writing file") {
            try data.write(to: url, options: .atomic)
            status = "Saved \(url.lastPathComponent)."
        }
    }

    func markExported(_ logs: [Worklog]) {
        let ids = Set(logs.map(\.id))
        for i in worklogs.indices where ids.contains(worklogs[i].id) {
            worklogs[i].status = .exported
            worklogs[i].locked = true
            dirty.insert(worklogs[i].id)
        }
        saveDirty()
    }

    /// Creates the worklogs in Tempo, one request per entry, and marks the successful ones as exported.
    func pushToTempo(_ logs: [Worklog]) async {
        isBusy = true
        defer { isBusy = false }
        saveDirty()
        do {
            let client = try TempoClient(settings: settings)
            if settings.jiraAccountId.isEmpty {
                status = "Reading your Jira account…"
                settings.jiraAccountId = try await client.accountId()
            }
            var done = 0
            var failures: [String] = []
            let payloads = TempoExport.payloads(logs, accountId: settings.jiraAccountId)
            let pending = TempoExport.exportable(logs)
            for (log, payload) in zip(pending, payloads) {
                status = "Sending \(done + 1) of \(pending.count) to Tempo…"
                do {
                    var p = payload
                    if let cached = db.cachedIssueId(p.issueKey) {
                        p.issueId = cached
                    } else {
                        let id = try await client.issueId(for: p.issueKey)
                        try db.cacheIssueId(p.issueKey, id)
                        p.issueId = id
                    }
                    let tempoId = try await client.create(p)
                    var w = log
                    w.tempoId = tempoId
                    w.status = .exported
                    w.locked = true
                    try db.updateWorklog(w)
                    done += 1
                } catch {
                    failures.append("\(payload.issueKey) \(payload.startDate): \(error.localizedDescription)")
                }
            }
            reloadRange()
            status = "Sent \(done) worklogs to Tempo." + (failures.isEmpty ? "" : " Failed: " + failures.joined(separator: " | "))
        } catch {
            status = error.localizedDescription
        }
    }

    // MARK: Helpers

    var databasePath: String { db.url.path }

    private func attempt(_ label: String, _ body: () throws -> Void) {
        do { try body() } catch { status = "\(label) failed: \(error.localizedDescription)" }
    }
}
