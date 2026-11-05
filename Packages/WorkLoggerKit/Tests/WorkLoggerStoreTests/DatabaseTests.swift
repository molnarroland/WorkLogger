import Foundation
import SQLite3
import Testing
import WorkLoggerCore
import WorkLoggerTestSupport

@testable import WorkLoggerStore

@Suite struct DatabaseTests {
    let db: Database

    init() throws {
        Fixture.pinTimeZone()
        db = try Database.inMemory()
    }

    func addRepo(_ name: String = "billing") throws -> Repo {
        try db.addRepo(path: "/tmp/\(name)", name: name)
        return try #require(try db.repos().first { $0.name == name })
    }

    func count(_ table: String) throws -> Int {
        try db.query("SELECT COUNT(*) AS n FROM \(table)").first?.intValue("n") ?? -1
    }

    // MARK: Schema & transactions

    @Test func migratesToTheLatestVersion() throws {
        #expect(try db.query("PRAGMA user_version").first?.intValue("user_version") == Database.schemaVersion)
        #expect(Database.schemaVersion == 5)
        #expect(db.path == ":memory:")
    }

    /// A database left at an older version (here 2: before chats) is brought up to date and keeps its data.
    @Test func upgradesAnOlderDatabaseAndKeepsItsData() throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("worklogger-\(UUID().uuidString).sqlite")
        defer { try? FileManager.default.removeItem(at: url) }
        var handle: OpaquePointer?
        #expect(sqlite3_open(url.path, &handle) == SQLITE_OK)
        for (i, sql) in Database.migrations.prefix(2).enumerated() {
            #expect(sqlite3_exec(handle, sql + "\nPRAGMA user_version = \(i + 1);", nil, nil, nil) == SQLITE_OK)
        }
        #expect(
            sqlite3_exec(handle, "INSERT INTO repos(path, name) VALUES('/w/billing', 'billing');", nil, nil, nil) == SQLITE_OK)
        sqlite3_close(handle)

        let upgraded = try Database(url: url)
        #expect(try upgraded.query("PRAGMA user_version").first?.intValue("user_version") == Database.schemaVersion)
        #expect(try upgraded.repos().map(\.name) == ["billing"])
        #expect(try upgraded.importChats(ChatExport(source: "t", conversations: [])) == 0)  // chat tables exist
    }

    @Test func failedTransactionRollsBack() throws {
        struct Boom: Error {}
        #expect(throws: Boom.self) {
            try db.transaction {
                try db.addRepo(path: "/tmp/x", name: "x")
                throw Boom()
            }
        }
        #expect(try db.repos().isEmpty)
    }

    // MARK: Settings

    @Test func settingsDefaultThenRoundTrip() throws {
        #expect(db.loadSettings() == AppSettings())
        var s = AppSettings()
        s.fixedDailyHours = 6
        try db.saveSettings(s)
        s.jiraAccounts = [JiraAccount(baseURL: "https://acme.atlassian.net", email: "me@x.hu")]
        try db.saveSettings(s)  // upsert
        #expect(db.loadSettings() == s)
    }

    // MARK: Repositories & projects

    @Test func reposAreUniqueByPathAndUpdatable() throws {
        try db.addRepo(path: "/tmp/billing", name: "billing")
        var repo = try addRepo("billing")
        #expect(try db.repos().count == 1)
        repo.issueKey = "TAO-1"
        repo.enabled = false
        try db.updateRepo(repo)
        #expect(try db.repos() == [repo])
    }

    @Test func deletingARepoDeletesItsCommits() throws {
        let repo = try addRepo()
        try db.saveCommits([Fixture.commit(day: "2026-03-03", "10:00", repoId: repo.id)])
        try db.deleteRepo(repo.id)
        #expect(try count("commits") == 0)
    }

    @Test func projectsUpsertAndDelete() throws {
        try db.upsertProject(ProjectMapping(key: "TAO", defaultIssue: "TAO-1"))
        try db.upsertProject(ProjectMapping(key: "TAO", defaultIssue: "TAO-2"))
        #expect(try db.projects() == [ProjectMapping(key: "TAO", defaultIssue: "TAO-2")])
        try db.deleteProject("TAO")
        #expect(try db.projects().isEmpty)
    }

    // MARK: Commits

    @Test func commitsOfADayFromEnabledReposOnly() throws {
        var off = try addRepo("off")
        let on = try addRepo("on")
        off.enabled = false
        try db.updateRepo(off)
        try db.saveCommits([
            Fixture.commit("a", day: "2026-03-03", "10:00", repoId: on.id),
            Fixture.commit("b", day: "2026-03-03", "11:00", repoId: off.id),
            Fixture.commit("c", day: "2026-03-04", "00:00", repoId: on.id),
        ])
        #expect(try db.commits(day: "2026-03-03").map(\.hash) == ["a"])
    }

    @Test func savingACommitAgainUpdatesItsStats() throws {
        let repo = try addRepo()
        var c = Fixture.commit("a", day: "2026-03-03", "10:00", repoId: repo.id)
        try db.saveCommits([c])
        c.insertions = 99
        c.ref = "refs/heads/feature"
        try db.saveCommits([c])
        #expect(try db.commits(day: "2026-03-03") == [c])
    }

    // MARK: Sessions & day records

    @Test func importReplacesTheImportedDays() throws {
        let day = "2026-03-03"
        try db.importTimesheet(
            ParsedTimesheet(sessions: [
                Fixture.session(day: day, "08:00", "12:00"),
                Fixture.session(day: "2026-03-04", "08:00", "16:00"),
            ]))
        let record = DayRecord(day: day, status: .leave, label: "Szabadság", reportedWorked: 8 * 3600, reportedBreaks: nil)
        try db.importTimesheet(ParsedTimesheet(days: [record]))  // leave: no sessions, clears the day

        #expect(try db.sessions(day: day).isEmpty)
        #expect(try db.sessions(day: "2026-03-04").count == 1)
        #expect(try db.dayRecord(day: day) == record)
    }

    @Test func clearingSessionsClearsDayRecords() throws {
        let day = "2026-03-03"
        try db.importTimesheet(
            ParsedTimesheet(
                sessions: [Fixture.session(day: day, "08:00", "12:00")],
                days: [DayRecord(day: day, status: .workday, label: "", reportedWorked: nil, reportedBreaks: nil)]))
        try db.deleteSessions(fromDay: day, toDay: day)
        #expect(try db.sessions(fromDay: day, toDay: day).isEmpty)
        #expect(try db.dayRecords(fromDay: day, toDay: day).isEmpty)
    }

    // MARK: Meetings

    @Test func syncKeepsSkippedAndRemovesStaleOrInactiveEvents() throws {
        let day = "2026-03-03"
        let (from, to) = DayKey.bounds(day)
        let kept = Fixture.meeting("kept", day: day, "09:00", "10:00", calendarId: "work")
        let stale = Fixture.meeting("stale", day: day, "11:00", "12:00", calendarId: "work")
        let other = Fixture.meeting("other", day: day, "13:00", "14:00", calendarId: "google")
        let inactive = Fixture.meeting("inactive", day: day, "15:00", "16:00", calendarId: "old")
        let all: Set<String> = ["work", "google", "old"]
        try db.syncMeetings(from: from, to: to, calendarIds: all, activeCalendarIds: all, events: [kept, stale, other, inactive])
        try db.setMeetingSkipped("kept", true)

        var renamed = kept
        renamed.title = "Renamed"
        try db.syncMeetings(from: from, to: to, calendarIds: ["work"], activeCalendarIds: ["work", "google"], events: [renamed])

        let meetings = try db.meetings(day: day)
        #expect(meetings.map(\.id) == ["kept", "other"])
        #expect(meetings.first?.title == "Renamed")
        #expect(meetings.first?.skipped == true)
    }

    // MARK: Worklogs

    @Test func worklogRoundTripAndUpdate() throws {
        var w = Fixture.worklog(day: "2026-03-03", "09:00", duration: 5400, issue: nil, ref: "a,b")
        w.id = try db.insertWorklog(w)
        #expect(try db.worklogs(day: "2026-03-03") == [w])

        w.issueKey = "TAO-1"
        w.status = .exported
        w.locked = true
        w.tempoId = 42
        w.day = "2026-03-04"
        try db.updateWorklog(w)
        #expect(try db.worklogs(fromDay: "2026-03-01", toDay: "2026-03-31") == [w])

        try db.deleteWorklog(w.id)
        #expect(try count("worklogs") == 0)
    }

    @Test func onlyUnlockedGeneratedDraftsAreRegeneratable() throws {
        let day = "2026-03-03"
        let entries: [(String, Worklog)] = [
            ("git draft", Fixture.worklog(day: day, "09:00", duration: 60, source: .git)),
            ("meeting draft", Fixture.worklog(day: day, "09:00", duration: 60, source: .meeting)),
            ("locked", Fixture.worklog(day: day, "09:00", duration: 60, source: .git, locked: true)),
            ("reviewed", Fixture.worklog(day: day, "09:00", duration: 60, source: .git, status: .reviewed)),
            ("manual", Fixture.worklog(day: day, "09:00", duration: 60, source: .manual)),
            ("outside", Fixture.worklog(day: "2026-03-10", "09:00", duration: 60, source: .git)),
        ]
        for (label, var w) in entries {
            w.description = label
            try db.insertWorklog(w)
        }
        try db.deleteRegeneratable(fromDay: "2026-03-02", toDay: "2026-03-06")
        let left = try db.worklogs(fromDay: "2026-03-01", toDay: "2026-03-31").map(\.description)
        #expect(Set(left) == ["locked", "reviewed", "manual", "outside"])
    }

    // MARK: Issue cache

    @Test func issueIdCache() throws {
        let a = "https://a.atlassian.net"
        let b = "https://b.atlassian.net"
        #expect(db.cachedIssueId("TAO-1", site: a) == nil)
        try db.cacheIssueId("TAO-1", 10001, site: a)
        try db.cacheIssueId("TAO-1", 10002, site: a)
        try db.cacheIssueId("TAO-1", 7, site: b)  // the same key on another site is another issue
        #expect(db.cachedIssueId("TAO-1", site: a) == 10002)
        #expect(db.cachedIssueId("TAO-1", site: b) == 7)
        #expect(db.cachedIssueKey(10002, site: a) == "TAO-1")
        #expect(db.cachedIssueKey(10002, site: b) == nil)
    }
}
