import Foundation
import Testing
import WorkLoggerTestSupport

@testable import WorkLoggerCore

@Suite struct AppSettingsTests {
    @Test func missingFieldsFallBackToDefaults() throws {
        let s = try #require(AppSettings.decodeMerging(Data(#"{"fixedDailyHours": 6}"#.utf8)))
        #expect(s.fixedDailyHours == 6)
        #expect(s.granularityMinutes == AppSettings().granularityMinutes)
        #expect(s.jiraAccounts.isEmpty)
    }

    @Test func theOldSingleJiraSiteBecomesTheDefaultAccount() throws {
        let s = try #require(
            AppSettings.decodeMerging(
                Data(#"{"jiraBaseURL": "https://acme.atlassian.net/", "jiraEmail": "me@x.hu", "jiraAccountId": "a1"}"#.utf8)))
        let account = try #require(s.defaultJiraAccount)
        #expect(s.jiraAccounts.count == 1 && s.defaultJiraAccountId == account.id)
        #expect([account.site, account.email, account.accountId] == ["https://acme.atlassian.net", "me@x.hu", "a1"])
        #expect(account.tempoBaseURL == JiraAccount().tempoBaseURL && account.provider == .tempo)
        // Saved again, the account stays the same (not migrated a second time).
        #expect(AppSettings.decodeMerging(try JSONEncoder().encode(s)) == s)
    }

    @Test func issuesGoToTheAccountOwningTheirProject() {
        var s = AppSettings()
        var work = JiraAccount(name: "Work")
        work.projectKeys = ["TAO", "INT"]
        var client = JiraAccount(name: "Client", provider: .jira)
        client.projectKeys = ["ACME"]
        s.jiraAccounts = [work, client]
        s.defaultJiraAccountId = client.id
        #expect(s.jiraAccount(for: "TAO-12")?.id == work.id)
        #expect(s.jiraAccount(for: "acme-3")?.id == client.id)
        #expect(s.jiraAccount(for: "HR-1")?.id == client.id)  // unknown project: the default
        s.defaultJiraAccountId = nil
        #expect(s.jiraAccount(for: "HR-1")?.id == work.id)  // no default picked: the first
    }

    @Test func roundTrip() throws {
        var s = AppSettings()
        s.authorEmails = ["me@example.com"]
        s.periods = [ProjectPeriod(title: "Migration", issueKey: "TAO-200", from: "2026-03-02", to: "2026-03-13")]
        s.meetingRules = [MeetingRule(contains: "standup", issueKey: "INT-12")]
        let data = try JSONEncoder().encode(s)
        #expect(AppSettings.decodeMerging(data) == s)
    }

    @Test func legacyRoundingModeStillDecodes() {
        #expect(AppSettings.decodeMerging(Data(#"{"roundingMode": "sessions"}"#.utf8))?.roundingMode == .sessions)
    }

    @Test(arguments: ["not json", "[1, 2]", #"{"fixedDailyHours": "eight"}"#])
    func invalidDataGivesNil(json: String) {
        #expect(AppSettings.decodeMerging(Data(json.utf8)) == nil)
    }

    @Test func periodsAreInclusive() {
        let p = ProjectPeriod(title: "", issueKey: "", from: "2026-03-02", to: "2026-03-06")
        #expect(p.contains("2026-03-02"))
        #expect(p.contains("2026-03-06"))
        #expect(!p.contains("2026-03-07"))
        #expect(!p.contains("2026-03-01"))
    }
}
