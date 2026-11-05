import Foundation
import SwiftUI
import UniformTypeIdentifiers
import EventKit

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
    var worklogs: [Worklog] = []
    var repos: [Repo] = []
    var projects: [ProjectMapping] = []
    var meetings: [Meeting] = []
    var sessions: [WorkSession] = []
    var dayRecords: [String: DayRecord] = [:]
    var dirty: Set<Int64> = []
    var status = "Ready"
    var chats: [ChatDay] = []
    /// Set when a background fetch found the site signed out: the window opens for sign-in and the fetch resumes.
    var signInNeeded: WebSession.Source?
    var isBusy = false
    var googleConnected: Bool
    var calendarAccess: MacCalendarStore.Access = .notDetermined
    var macCalendars: [MacCalendarInfo] = []

    @ObservationIgnored let services: AppDependencies
    @ObservationIgnored private var googleAccount: (any GoogleCalendarAccount)?
    @ObservationIgnored private var calendarObserver: NSObjectProtocol?

    init(db: Database, services: AppDependencies) {
        self.db = db
        self.services = services
        settings = db.loadSettings()
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

    // MARK: Collect

    /// Reads git history and the calendar for the selected range, then rebuilds the drafts.
    func collect() async {
        guard !isBusy else { return }
        isBusy = true
        defer { isBusy = false }

        let useGit = settings.modules.contains(.git)
        var authors = settings.authorEmails
        if useGit, authors.isEmpty, let email = services.git.globalUserEmail() {
            authors = [email]
            settings.authorEmails = [email]
        }
        guard !useGit || !authors.isEmpty else {
            status = "Add your commit email address under Repositories so only your commits are counted."
            return
        }

        let enabled = useGit ? repos.filter(\.enabled) : []
        let start = rangeStart, end = rangeEnd
        let ignored = settings.ignoredPathPatterns
        let git = services.git
        status = "Reading git history of \(enabled.count) repositories…"
        let (results, errors) = await Task.detached(priority: .userInitiated) { () -> ([(Int64, [GitCommit])], [String]) in
            var ok: [(Int64, [GitCommit])] = []
            var failed: [String] = []
            for repo in enabled {
                do {
                    ok.append((repo.id, try git.log(repoPath: repo.path, since: start, until: end, authors: authors)))
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
            status = "Reading meetings from your calendars…"
            do { meetingCount = try await fetchMeetings() } catch { status = "Calendar: \(error.localizedDescription)" }
        }

        rebuild()
        var parts: [String] = []
        if useGit { parts.append("\(commitCount) commits") }
        if canReadMeetings { parts.append("\(meetingCount) meetings") }
        var summary = parts.isEmpty ? "Nothing to collect" : "Collected " + parts.joined(separator: " and ")
        summary += " for \(fromDay) – \(toDay)."
        if !errors.isEmpty { summary += " Skipped: " + errors.joined(separator: "; ") }
        status = summary
    }

    /// Rebuilds unlocked drafts from stored commits, meetings and sessions (no network or git access).
    func rebuild() {
        saveDirty()
        attempt("Building worklogs") {
            try DraftRebuilder(db: db, builder: makeBuilder()).rebuild(range)
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
        guard let url = services.files.chooseFolder() else { return }
        settings.workspaceRoot = url.path
        scanWorkspace()
    }

    func scanWorkspace() {
        guard !settings.workspaceRoot.isEmpty else { status = "Choose your workspace folder first."; return }
        let root = URL(fileURLWithPath: (settings.workspaceRoot as NSString).expandingTildeInPath)
        let found = services.git.findRepos(root: root, excluded: settings.excludedFolders)
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

    /// Reads the HRmaster work-time calendar for each month of the range in the background, with progress in the
    /// status bar. Returns true when you need to sign in first.
    @discardableResult
    func fetchHRmaster() async -> Bool {
        guard !isBusy else { return false }
        guard let base = HRmasterCollector.normalize(settings.hrmasterURL) else {
            status = "Enter your company's HRmaster address on the Work sessions page first."
            return false
        }
        isBusy = true
        defer { isBusy = false }
        do {
            let pages = try await HRmasterCollector(session: .hrmaster, base: base)
                .collect(HRmasterCollector.months(from: rangeStart, to: rangeEnd)) { self.status = $0 }
            var days = 0
            attempt("Importing work sessions") {
                for (m, html) in pages {
                    let parsed = HRmasterParser(fallbackYear: m.year, fallbackMonth: m.month).parse(html)
                    try db.importTimesheet(parsed)
                    days += parsed.dayKeys.count
                }
                status = "HRmaster: imported \(days) days. Rebuild drafts to apply them."
            }
            reloadRange()
        } catch WebSessionError.notSignedIn {
            status = "HRmaster is signed out. Sign in in the HRmaster window; fetching continues after."
            signInNeeded = .hrmaster
            return true
        } catch {
            status = error.localizedDescription
        }
        return false
    }

    func clearSessionsInRange() {
        attempt("Clearing sessions") { try db.deleteSessions(fromDay: fromDay, toDay: toDay) }
        reloadRange()
    }

    // MARK: Google Calendar

    private func google() -> any GoogleCalendarAccount {
        let secret = services.secrets.get(SecretKey.googleSecret) ?? ""
        if let a = googleAccount, a.clientId == settings.googleClientId, a.clientSecret == secret { return a }
        let a = services.makeGoogle(settings.googleClientId, secret)
        googleAccount = a
        return a
    }

    func connectGoogle() async {
        isBusy = true
        defer { isBusy = false }
        status = "Finish signing in to Google in your browser…"
        do {
            try await google().signIn()
            googleConnected = true
            status = "Google Calendar connected."
        } catch {
            status = error.localizedDescription
        }
    }

    func disconnectGoogle() {
        google().signOut()
        googleConnected = false
        status = "Google Calendar disconnected."
    }

    // MARK: macOS calendars (any online account via Internet Accounts)

    var canReadMeetings: Bool {
        settings.modules.contains(.calendar)
            && (googleConnected || (calendarAccess == .granted && !settings.macCalendarIds.isEmpty))
    }

    private var googleApiCalendarId: String {
        settings.googleCalendarId.isEmpty ? "primary" : settings.googleCalendarId
    }

    func refreshCalendars() {
        calendarAccess = services.calendars.access
        macCalendars = calendarAccess == .granted ? services.calendars.calendars() : []
    }

    func openInternetAccounts() {
        MacCalendarStore.openInternetAccounts()
        status = "In Internet Accounts choose Add Account…, sign in and keep Calendars switched on."
    }

    func requestCalendarAccess() async {
        do {
            let granted = try await services.calendars.requestAccess()
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
        let macEvents = services.calendars.meetings(calendarIds: macIds, from: rangeStart, to: rangeEnd)
        try db.syncMeetings(from: rangeStart, to: rangeEnd, calendarIds: macIds, activeCalendarIds: active, events: macEvents)
        count += macEvents.count

        if googleConnected {
            let events = try await google().meetings(calendarId: googleApiCalendarId, from: rangeStart, to: rangeEnd)
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
        services.files.copyToPasteboard(markdown(logs))
        status = "Copied the Tempo checklist for \(TempoExport.exportable(logs).count) worklogs. Paste it to Claude with Claude in Chrome."
    }

    private func save(_ data: Data, name: String, type: UTType) {
        guard let url = services.files.saveLocation(suggestedName: name, type: type) else { return }
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
            let client = try services.makeTempo(settings)
            if settings.jiraAccountId.isEmpty {
                status = "Reading your Jira account…"
                settings.jiraAccountId = try await client.accountId()
            }
            let outcome = await TempoExporter(db: db, client: client).push(logs, accountId: settings.jiraAccountId) {
                self.status = "Sending \($0) of \($1) to Tempo…"
            }
            reloadRange()
            status = "Sent \(outcome.sent) worklogs to Tempo."
                + (outcome.failures.isEmpty ? "" : " Failed: " + outcome.failures.joined(separator: " | "))
        } catch {
            status = error.localizedDescription
        }
    }

    // MARK: Helpers

    var databasePath: String { db.path }

    // MARK: Chats

    /// Imports a pasted chat export; returns false (with the reason in the status) when it can't be read.
    func importChats(_ pasted: String) -> Bool {
        do {
            let export = try ChatExport.parse(pasted)
            let added = try db.importChats(export)
            reloadRange()
            status = "Imported \(added) new messages from \(export.conversations.count) conversations."
            return true
        } catch {
            status = "Chat import failed: \(error.localizedDescription)"
            return false
        }
    }

    func chatLines(_ c: ChatDay) -> [ChatLine] {
        (try? db.chatLines(conversationId: c.conversationId, day: c.day)) ?? []
    }

    /// The worklog a conversation-day was logged to, if any.
    func chatWorklog(_ c: ChatDay) -> Worklog? { worklogs.first { $0.source == .chat && $0.sourceRef == c.ref } }

    /// Logs a conversation-day's active time to an issue (or re-targets the existing entry); locked so rebuilds keep it.
    func logChat(_ c: ChatDay, issue: String) {
        guard let key = JiraKey.normalize(issue), c.activeSeconds > ChatDay.minLoggable else { return }
        saveDirty()
        let minutes = (c.activeSeconds + 59) / 60
        attempt("Logging chat") {
            if var w = chatWorklog(c) {
                guard w.status != .exported else { return }
                w.issueKey = key
                try db.updateWorklog(w)
            } else {
                try db.insertWorklog(Worklog(id: 0, day: c.day, start: c.times[0], duration: minutes * 60, issueKey: key,
                                             description: "Chat: \(c.name)", source: .chat, sourceRef: c.ref, repoId: nil,
                                             estimated: c.activeSeconds, status: .draft, locked: true, tempoId: nil))
            }
            status = "Logged \(DurationFormat.string(minutes * 60)) of “\(c.name)” to \(key)."
        }
        reloadRange()
    }

    /// Reads your Google Chat messages for the range in the background (the signed-in Google Chat page, no window)
    /// and imports them, with progress in the status bar. Returns true when you need to sign in first.
    @discardableResult
    func fetchGoogleChat() async -> Bool {
        guard !isBusy else { return false }
        let email = settings.chatEmail.isEmpty ? (settings.authorEmails.first ?? "") : settings.chatEmail
        guard !email.isEmpty else { status = "Enter your Google Chat email on the Chats page first."; return false }
        isBusy = true
        defer { isBusy = false }
        do {
            let export = try await GoogleChatCollector(session: .googleChat, email: email)
                .collect(from: rangeStart, to: rangeEnd) { self.status = $0 }
            let added = try db.importChats(export)
            reloadRange()
            status = "Google Chat: \(added) new messages in \(export.conversations.count) conversations."
        } catch WebSessionError.notSignedIn {
            status = "Google Chat is signed out. Sign in in the Google Chat window; fetching continues after."
            signInNeeded = .googleChat
            return true
        } catch {
            status = "Google Chat: \(error.localizedDescription)"
        }
        return false
    }

    func clearChatsInRange() {
        attempt("Clearing chats") { try db.deleteChats(from: rangeStart, to: rangeEnd) }
        reloadRange()
    }

    private func attempt(_ label: String, _ body: () throws -> Void) {
        do { try body() } catch { status = "\(label) failed: \(error.localizedDescription)" }
    }
}
