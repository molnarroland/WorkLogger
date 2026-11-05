import Foundation
import Testing
@testable import WorkLoggerTempo
import WorkLoggerTestSupport
import WorkLoggerCore
import WorkLoggerStore

@Suite struct TempoExportTests {
    init() { Fixture.pinTimeZone() }

    var logs: [Worklog] {
        var quoted = Fixture.worklog(day: "2026-03-03", "09:00", duration: 5400, issue: "TAO-1")
        quoted.description = "Fix \"quoted\"\nthing"
        return [quoted,
                Fixture.worklog(day: "2026-03-03", "11:00", duration: 1800, issue: nil),
                Fixture.worklog(day: "2026-03-03", "12:00", duration: 0, issue: "TAO-2")]
    }

    @Test func onlyEntriesWithIssueAndTimeAreExported() {
        #expect(TempoExport.exportable(logs).map(\.issueKey) == ["TAO-1"])
    }

    @Test func csvUsesUTCAndQuotes() {
        // 09:00 in Budapest is 08:00 UTC.
        #expect(TempoExport.csv(logs) == """
        Issue Key,Date Started,Time Spent,Comment
        TAO-1,2026-03-03 08:00:00Z,1h 30m,"Fix ""quoted"" thing"

        """)
    }

    @Test func payloadsUseLocalDateAndTime() throws {
        let p = try #require(TempoExport.payloads(logs, accountId: "acc-1").first)
        #expect(p.issueKey == "TAO-1")
        #expect(p.issueId == nil)
        #expect(p.authorAccountId == "acc-1")
        #expect(p.startDate == "2026-03-03")
        #expect(p.startTime == "09:00:00")
        #expect(p.timeSpentSeconds == 5400)
    }

    @Test(arguments: [nil, ""] as [String?])
    func missingAccountIdIsLeftOut(accountId: String?) {
        #expect(TempoExport.payloads(logs, accountId: accountId).first?.authorAccountId == nil)
    }

    @Test func jsonDecodesToThePayloads() throws {
        let data = try TempoExport.json(logs, accountId: "acc-1")
        let decoded = try JSONDecoder().decode([TempoWorklogPayload].self, from: data)
        #expect(decoded.map(\.issueKey) == ["TAO-1"])
        #expect(decoded.first?.startTime == "09:00:00")
    }

    /// Moved from the HRmaster parser tests when the parser went to WorkLoggerCore (the export stays in the app).
    @Test func markdownExportListsEntriesPerDay() {
        let w = Worklog(id: 1, day: "2026-03-03", start: DayKey.at("2026-03-03", hm: "09:00")!, duration: 5400, issueKey: "TAO-1",
                        description: "Fix | pipe", source: .git, sourceRef: "", repoId: nil, estimated: 5400,
                        status: .draft, locked: false, tempoId: nil)
        var noIssue = w
        noIssue.issueKey = nil
        let md = TempoExport.markdown([w, noIssue], from: "2026-03-01", to: "2026-03-31", jiraBaseURL: "https://acme.atlassian.net/")
        #expect(md.contains("https://acme.atlassian.net/plugins/servlet/ac/io.tempo.jira/tempo-app"))
        #expect(md.contains("| 1 | TAO-1 | 09:00 | 1h 30m | Fix \\| pipe |"))
        #expect(md.contains("1 worklogs"))
    }
}
