import Foundation
import SwiftUI
import Testing
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

@testable import WorkLogger

/// AppState on an in-memory database with fake services; "now" is Wed 2026-03-04 12:00 in Budapest.
@MainActor
@Suite struct AppStateTests {
    func gitCommit(_ hash: String, _ day: String, _ hm: String, subject: String = "TAO-1 Fix") -> GitCommit {
        GitCommit(
            hash: hash, date: Fixture.at(day, hm), author: "Me", email: "me@example.com", subject: subject,
            body: "", ref: "", files: [FileStat(added: 10, deleted: 0, path: "a.swift")])
    }

    // MARK: Range

    @Test func startsOnTheCurrentMonth() throws {
        let app = try AppHarness()
        let state = app.state
        #expect(state.fromDay == "2026-03-01")
        #expect(state.toDay == "2026-03-31")
    }

    @Test func quickRangesAndMonthShiftsUseTheInjectedClock() throws {
        let app = try AppHarness()
        let state = app.state
        state.setRange(.lastWeek)
        #expect([state.fromDay, state.toDay] == ["2026-02-23", "2026-03-01"])
        state.setRange(.thisMonth)
        state.shiftMonth(1)
        #expect([state.fromDay, state.toDay] == ["2026-04-01", "2026-04-30"])
    }

    @Test func expectedHours() throws {
        let app = try AppHarness()
        let state = app.state
        #expect(state.expected(day: "2026-03-03") == 8 * 3600)
        #expect(state.expected(day: "2026-03-07") == 0)
        #expect(state.isWorkday("2026-03-03"))
        #expect(!state.acceptsWork("2026-03-07"))
    }

    @Test func settingsArePersisted() throws {
        let app = try AppHarness()
        app.state.settings.fixedDailyHours = 6
        #expect(app.db.loadSettings().fixedDailyHours == 6)
    }

    // MARK: Collect

    @Test func collectNeedsAnAuthorEmail() async throws {
        let app = try AppHarness()
        await app.state.collect()
        #expect(app.state.status.hasPrefix("Add your commit email address"))
        #expect(app.git.logCalls.isEmpty)
        #expect(!app.state.isBusy)
    }

    @Test func collectReadsGitWithTheGlobalEmailAndBuildsDrafts() async throws {
        let app = try AppHarness()
        app.git.email = "me@example.com"
        let repo = try app.addRepo()
        app.git.commits[repo.path] = [gitCommit("a", "2026-03-03", "10:00")]

        await app.state.collect()

        #expect(app.state.settings.authorEmails == ["me@example.com"])
        #expect(app.git.logCalls.map(\.authors) == [["me@example.com"]])
        let entry = try #require(app.state.worklogs.first)
        #expect(app.state.worklogs.count == 1)
        #expect(entry.day == "2026-03-03")
        #expect(entry.issueKey == "TAO-1")
        #expect(entry.duration == 8 * 3600)
        #expect(app.state.status == "Collected 1 commits for 2026-03-01 – 2026-03-31.")
    }

    @Test func collectSkipsFailingAndDisabledRepositories() async throws {
        var settings = AppSettings()
        settings.authorEmails = ["me@example.com"]
        let app = try AppHarness(settings: settings)
        let broken = try app.addRepo("broken")
        var off = try app.addRepo("off")
        off.enabled = false
        app.state.update(off)
        app.state.reloadAll()
        app.git.failingPaths = [broken.path]

        await app.state.collect()

        #expect(app.git.logCalls.map(\.path) == [broken.path])
        #expect(app.state.status.hasSuffix("Skipped: broken: not a git repository"))
    }

    @Test func collectAlsoReadsSelectedCalendars() async throws {
        var settings = AppSettings()
        settings.authorEmails = ["me@example.com"]
        settings.macCalendarIds = ["work"]
        let app = try AppHarness(settings: settings)
        app.calendars.access = .granted
        app.calendars.events = [Fixture.meeting("m1", day: "2026-03-03", "09:00", "10:00", calendarId: "work")]
        app.state.refreshCalendars()

        await app.state.collect()

        #expect(app.state.meetings.map(\.id) == ["m1"])
        #expect(app.state.worklogs.map(\.source) == [.meeting])
        #expect(app.state.status == "Collected 0 commits and 1 meetings for 2026-03-01 – 2026-03-31.")
    }

    @Test func disabledModulesAreNeitherReadNorBuilt() async throws {
        var settings = AppSettings()
        settings.macCalendarIds = ["work"]
        settings.modules = [.calendar]
        let app = try AppHarness(settings: settings)
        let repo = try app.addRepo()
        app.git.commits[repo.path] = [gitCommit("a", "2026-03-03", "10:00")]
        app.calendars.access = .granted
        app.calendars.events = [Fixture.meeting("m1", day: "2026-03-03", "09:00", "10:00", calendarId: "work")]
        app.state.refreshCalendars()

        await app.state.collect()  // no author email needed without git
        #expect(app.git.logCalls.isEmpty)
        #expect(app.state.worklogs.map(\.source) == [.meeting])
        #expect(app.state.status == "Collected 1 meetings for 2026-03-01 – 2026-03-31.")

        app.state.settings.modules = []
        #expect(app.state.meetings.isEmpty)
        app.state.rebuild()
        #expect(!app.state.worklogs.contains { $0.source == .meeting })
    }

    @Test func chatsAreLoggedToAnIssueOnceAndSurviveRebuilds() throws {
        let app = try AppHarness()
        let json = """
            {"source":"google-chat","conversations":[{"id":"dm/B","type":"dm","name":"Bea","messages":[
              {"time":"2026-03-03T09:00:00Z","sender":"me@x","me":true,"text":"second"},
              {"time":"2026-03-03T08:58:00Z","sender":null,"me":false,"text":"first"}]}]}
            """
        #expect(app.state.importChats(json))
        let chat = try #require(app.state.chats.first)
        #expect(app.state.chatLines(chat).map(\.text) == ["first", "second"])

        app.state.logChat(chat, issue: "tao-7")
        app.state.logChat(chat, issue: "TAO-8")
        app.state.rebuild()
        let logs = app.state.worklogs.filter { $0.source == .chat }
        #expect(logs.map(\.issueKey) == ["TAO-8"])
        #expect(logs.first?.duration == 120)
    }

    @Test func chatGapSettingAndAlwaysRules() throws {
        let app = try AppHarness()
        let json = """
            {"source":"google-chat","conversations":[{"id":"space/T","type":"space","name":"TAO","messages":[
              {"time":"2026-03-03T08:00:00Z","sender":"me@x","me":true,"text":"a"},
              {"time":"2026-03-03T08:14:00Z","sender":"me@x","me":true,"text":"b"},
              {"time":"2026-03-04T08:00:00Z","sender":"me@x","me":true,"text":"c"}]}]}
            """
        #expect(app.state.importChats(json))
        let tuesday = try #require(app.state.chats.first { $0.day == "2026-03-03" })
        #expect(tuesday.activeSeconds == 120)  // 14 min apart: two one-minute stretches
        app.state.settings.chatGapMinutes = 15
        #expect(app.state.chats.first { $0.day == "2026-03-03" }?.activeSeconds == 14 * 60)  // now one stretch

        app.state.setChatRule(tuesday, issue: "tao-9")
        #expect(app.state.settings.chatRules.map(\.issueKey) == ["TAO-9"])
        #expect(app.state.worklogs.filter { $0.source == .chat }.map(\.day) == ["2026-03-03", "2026-03-04"])
        app.state.rebuild()  // rules don't log a day twice
        #expect(app.state.worklogs.filter { $0.source == .chat }.count == 2)

        app.state.setChatRule(tuesday, issue: nil)
        #expect(app.state.settings.chatRules.isEmpty)
    }

    @Test func missingDaysAndRangeArrows() throws {
        let app = try AppHarness()  // "now" is Wed 2026-03-04, the range is March
        for (day, hours, issue) in [("2026-03-02", 8, "TAO-1"), ("2026-03-03", 4, "TAO-1"), ("2026-03-04", 8, nil)]
            as [(String, Int, String?)]
        {
            _ = try app.db.insertWorklog(Fixture.worklog(day: day, "09:00", duration: hours * 3600, issue: issue))
        }
        app.state.reloadRange()
        // Short (Tue) and without a ticket (Wed); complete Monday, the weekend and days after today are left out.
        #expect(app.state.missingDays == ["2026-03-03", "2026-03-04"])

        app.state.shiftRange(1)
        #expect([app.state.fromDay, app.state.toDay] == ["2026-04-01", "2026-04-30"])
        app.state.setRange(from: Fixture.at("2026-04-08", "00:00"), to: Fixture.at("2026-04-06", "00:00"))
        #expect([app.state.fromDay, app.state.toDay] == ["2026-04-06", "2026-04-08"])
        app.state.shiftRange(-1)
        #expect([app.state.fromDay, app.state.toDay] == ["2026-04-03", "2026-04-05"])
    }

    /// Settings with one Jira account (Tempo).
    static var withAccount: AppSettings {
        var s = AppSettings()
        s.jiraAccounts = [JiraAccount(name: "Work", baseURL: "https://acme.atlassian.net", email: "me@x.hu")]
        return s
    }

    @Test func tempoWorklogsAreReadAndMatched() async throws {
        let app = try AppHarness(settings: Self.withAccount)
        app.tempo.projects = ["TAO", "INT"]
        _ = try app.db.insertWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "TAO-1"))
        _ = try app.db.insertWorklog(Fixture.worklog(day: "2026-03-03", "10:00", duration: 1800, issue: "INT-12"))
        app.state.reloadRange()
        app.tempo.keysById = [10001: "TAO-1"]
        app.tempo.existing = [
            TempoWorklog(id: 1, issueId: 10001, day: "2026-03-03", startTime: "09:00:00", seconds: 3600, description: "Fix")
        ]

        await app.state.refreshTempo()

        #expect(app.state.tempoConnected)
        #expect(app.state.tempoLogs.map(\.issueKey) == ["TAO-1"])
        #expect(app.state.tempoSeconds(day: "2026-03-03") == 3600)
        let tao = try #require(app.state.worklogs.first { $0.issueKey == "TAO-1" })
        let int = try #require(app.state.worklogs.first { $0.issueKey == "INT-12" })
        #expect(app.state.isInTempo(tao))
        #expect(!app.state.isInTempo(int))
        #expect(app.state.tempoProgress.entered == 1 && app.state.tempoProgress.total == 2)
        #expect(app.state.settings.jiraAccounts[0].accountId == "acc-1")
        #expect(app.state.settings.jiraAccounts[0].projectKeys == ["INT", "TAO"])  // read once, sorted

        app.tempoError = TempoError.missing("the Tempo API token")  // no token: nothing shown, no error
        await app.state.refreshTempo()
        #expect(!app.state.tempoConnected)
        #expect(app.state.tempoLogs.isEmpty)
    }

    // MARK: Editing

    @Test func addManualStartsAfterTheLastEntry() throws {
        let app = try AppHarness()
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        app.state.addManual(day: "2026-03-03")
        let manual = try #require(app.state.worklogs.first { $0.source == .manual })
        #expect(DayKey.hm(manual.start) == "10:00")
        #expect(manual.duration == 30 * 60)
        #expect(manual.locked)
    }

    @Test func addManualOnAnEmptyDayStartsAtTheDefaultStart() throws {
        let app = try AppHarness()
        app.state.addManual(day: "2026-03-05")
        #expect(app.state.worklogs.map { DayKey.hm($0.start) } == ["09:00"])
    }

    @Test func moveAppendsLocksAndKeepsExportedEntries() throws {
        let app = try AppHarness()
        let moving = try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        let exported = try app.addWorklog(Fixture.worklog(day: "2026-03-03", "10:00", duration: 3600, status: .exported))
        try app.addWorklog(Fixture.worklog(day: "2026-03-04", "09:00", duration: 5400))

        app.state.move([moving.id, exported.id], to: "2026-03-04")

        let moved = try #require(app.state.worklogs.first { $0.id == moving.id })
        #expect(moved.day == "2026-03-04")
        #expect(DayKey.hm(moved.start) == "10:30")
        #expect(moved.locked)
        #expect(app.state.worklogs.first { $0.id == exported.id }?.day == "2026-03-03")
        // "Moved 1 entry …" in English: the catalog's singular form.
        #expect(app.state.status == String(localized: "Moved \(1) entries to \(DayKey.title("2026-03-04"))."))
    }

    @Test func editsAreSavedWhenMarkedDirty() throws {
        let app = try AppHarness()
        let w = try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        app.state.worklogs[0].description = "Edited"
        app.state.markDirty(w.id)
        app.state.saveDirty()
        #expect(try app.db.worklogs(day: "2026-03-03").first?.description == "Edited")
        #expect(app.state.dirty.isEmpty)
    }

    @Test func reviewingADayLeavesExportedEntriesAlone() throws {
        let app = try AppHarness()
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "10:00", duration: 3600, status: .exported))
        app.state.setStatus(.reviewed, forAllIn: "2026-03-03")
        #expect(try app.db.worklogs(day: "2026-03-03").map(\.status) == [.reviewed, .exported])
    }

    @Test func deleteRemovesTheEntry() throws {
        let app = try AppHarness()
        let w = try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        app.state.delete(w)
        #expect(app.state.worklogs.isEmpty)
        #expect(try app.db.worklogs(day: "2026-03-03").isEmpty)
    }

    // MARK: Repositories

    @Test func choosingAWorkspaceScansItAndDropsHiddenRepositories() throws {
        var settings = AppSettings()
        settings.excludedFolders = ["archive"]
        let app = try AppHarness(settings: settings)
        app.git.foundRepos = [URL(fileURLWithPath: "/w/api"), URL(fileURLWithPath: "/w/archive/old")]
        app.files.folder = URL(fileURLWithPath: "/w")

        app.state.chooseWorkspace()

        #expect(app.state.settings.workspaceRoot == "/w")
        #expect(app.state.repos.map(\.name) == ["api"])
        #expect(app.state.status == "Found 2 repositories in w.")
    }

    @Test func cancellingTheFolderPanelChangesNothing() throws {
        let app = try AppHarness()
        app.state.chooseWorkspace()
        #expect(app.state.settings.workspaceRoot.isEmpty)
    }

    @Test func repositoryKeysAreNormalized() throws {
        let app = try AppHarness()
        var repo = try app.addRepo()
        repo.issueKey = " tao-1 "
        repo.projectKey = ""
        app.state.update(repo)
        let saved = try #require(try app.db.repos().first)
        #expect(saved.issueKey == "TAO-1")
        #expect(saved.projectKey == nil)
    }

    // MARK: Calendars

    @Test func grantingCalendarAccessListsTheCalendars() async throws {
        let app = try AppHarness()
        app.calendars.list = [MacCalendarInfo(id: "work", title: "Work", account: "me@example.com", color: .blue)]
        #expect(app.state.calendarAccess == .notDetermined)
        await app.state.requestCalendarAccess()
        #expect(app.state.calendarAccess == .granted)
        #expect(app.state.macCalendars.map(\.id) == ["work"])
        #expect(app.state.status.hasPrefix("Calendar access allowed"))
    }

    @Test func fetchMeetingsReadsSelectedMacCalendarsAndGoogle() async throws {
        let app = try AppHarness()
        app.secrets.set("refresh", for: SecretKey.googleRefresh)
        app.calendars.access = .granted
        app.calendars.events = [
            Fixture.meeting("mac", day: "2026-03-03", "09:00", "10:00", calendarId: "work"),
            Fixture.meeting("private", day: "2026-03-03", "11:00", "12:00", calendarId: "home"),
        ]
        app.googleEvents = [Fixture.meeting("google", day: "2026-03-05", "14:00", "15:00", calendarId: "primary")]
        await app.state.unlock()
        app.state.refreshCalendars()
        app.state.setCalendar("work", selected: true)

        let count = try await app.state.fetchMeetings()

        #expect(app.state.googleConnected)
        #expect(count == 2)
        #expect(app.state.meetings.map(\.id) == ["mac", "google"])
    }

    @Test func connectAndDisconnectGoogle() async throws {
        var settings = AppSettings()
        settings.googleClientId = "client"
        let app = try AppHarness(settings: settings)
        app.secrets.set("secret", for: SecretKey.googleSecret)

        await app.state.connectGoogle()
        #expect(app.state.googleConnected)
        #expect(app.googleAccounts.map { [$0.clientId, $0.clientSecret] } == [["client", "secret"]])

        app.state.disconnectGoogle()
        #expect(!app.state.googleConnected)
        #expect(app.googleAccounts.count == 1)  // the same account is reused
        #expect(app.googleAccounts.first?.signedOut == true)
    }

    @Test func failedGoogleSignInShowsTheError() async throws {
        let app = try AppHarness()
        app.googleSignInError = FakeError(message: "denied")
        await app.state.connectGoogle()
        #expect(!app.state.googleConnected)
        #expect(app.state.status == "denied")
    }

    // MARK: Export

    @Test func exportCSVWritesTheChosenFile() throws {
        let app = try AppHarness()
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("tempo-\(UUID().uuidString).csv")
        defer { try? FileManager.default.removeItem(at: url) }
        app.files.saveURL = url

        app.state.exportCSV(app.state.worklogs)

        #expect(app.files.suggestedNames == ["tempo-2026-03-01_2026-03-31.csv"])
        #expect(try String(contentsOf: url, encoding: .utf8) == TempoExport.csv(app.state.worklogs))
        #expect(app.state.status == "Saved \(url.lastPathComponent).")
    }

    @Test func cancelledExportWritesNothing() throws {
        let app = try AppHarness()
        let before = app.state.status
        app.state.exportJSON([])
        #expect(app.state.status == before)
    }

    @Test func copyMarkdownUsesThePasteboard() throws {
        let app = try AppHarness()
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        app.state.copyMarkdown(app.state.worklogs)
        #expect(app.files.copied == [app.state.markdown(app.state.worklogs)])
    }

    @Test func pushToTempoResolvesTheAccountAndMarksEntriesExported() async throws {
        let app = try AppHarness(settings: Self.withAccount)
        app.tempo.issueIds = ["TAO-1": 10001]
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "10:00", duration: 1800, issue: nil))

        await app.state.pushToTempo(app.state.worklogs)

        #expect(app.state.settings.jiraAccounts[0].accountId == "acc-1")
        #expect(app.tempo.created.map(\.authorAccountId) == ["acc-1"])
        #expect(app.state.worklogs.map(\.status) == [.exported, .draft])
        #expect(app.state.worklogs.first?.tempoId == 100)
        #expect(app.state.status == String(localized: "Sent \(1) worklogs to Tempo."))  // "Sent 1 worklog …" in English
        #expect(!app.state.isBusy)
    }

    @Test func pushToTempoWithoutConfigurationShowsTheProblem() async throws {
        let app = try AppHarness(settings: Self.withAccount)
        app.tempoError = TempoError.missing("the Tempo API token")
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600))
        await app.state.pushToTempo(app.state.worklogs)
        #expect(app.state.status.contains(TempoError.missing("the Tempo API token").localizedDescription))
        #expect(app.state.worklogs.map(\.status) == [.draft])
    }

    @Test func pushSendsEachIssueToTheAccountOwningItsProject() async throws {
        var s = AppSettings()
        var work = JiraAccount(name: "Work", baseURL: "https://acme.atlassian.net", email: "me@x.hu")
        work.projectKeys = ["TAO"]
        var other = JiraAccount(name: "Client", baseURL: "https://client.atlassian.net", email: "me@x.hu", provider: .other)
        other.projectKeys = ["ACME"]
        s.jiraAccounts = [work, other]
        s.defaultJiraAccountId = work.id
        let app = try AppHarness(settings: s)
        app.tempo.issueIds = ["TAO-1": 1, "HR-2": 2]
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "TAO-1"))
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "10:00", duration: 3600, issue: "ACME-7"))
        try app.addWorklog(Fixture.worklog(day: "2026-03-03", "11:00", duration: 3600, issue: "HR-2"))  // → default

        await app.state.pushToTempo(app.state.worklogs)

        #expect(app.tempo.created.map(\.issueKey) == ["TAO-1", "HR-2"])
        #expect(app.state.worklogs.map(\.status) == [.exported, .draft, .exported])
        #expect(app.state.status.contains("Client"))  // ACME is logged in another tool: named, not sent
    }

    @Test func startsLockedAndUnlocksOnRequest() async throws {
        let app = try AppHarness()
        app.secrets.set("refresh", for: SecretKey.googleRefresh)
        #expect(app.state.locked && !app.state.googleConnected)  // nothing read before Unlock
        app.unlocks = false  // cancelled
        await app.state.unlock()
        #expect(app.state.locked)
        app.unlocks = true
        await app.state.unlock()
        #expect(!app.state.locked && app.state.googleConnected)
    }

    @Test func oldTokensMoveToTheMigratedAccount() async throws {
        let app = try AppHarness(settings: Self.withAccount)
        app.secrets.set("jira-t", for: SecretKey.legacyJiraToken)
        await app.state.unlock()
        let account = app.state.settings.jiraAccounts[0]
        #expect(app.secrets.get(account.jiraTokenKey) == "jira-t")
        #expect(app.secrets.get(SecretKey.legacyJiraToken) == nil)
    }

    // MARK: Sessions

    @Test func importAndClearSessions() throws {
        let app = try AppHarness()
        let day = "2026-03-03"
        app.state.importTimesheet(ParsedTimesheet(sessions: [Fixture.session(day: day, "08:00", "16:00")]))
        #expect(app.state.sessions.count == 1)
        #expect(app.state.status.hasPrefix("Imported 1 sessions and breaks for 1 days."))
        app.state.clearSessionsInRange()
        #expect(app.state.sessions.isEmpty)
    }
}
