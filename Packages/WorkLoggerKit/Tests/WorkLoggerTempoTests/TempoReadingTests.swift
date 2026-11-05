import Foundation
import Testing
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerTestSupport

@testable import WorkLoggerTempo

/// Reading worklogs back from Tempo: paging, issue keys, and which ticket-days are already covered.
@MainActor
@Suite struct TempoReadingTests {
    init() { Fixture.pinTimeZone() }

    let secrets = InMemorySecrets([SecretKey.jiraToken: "jira-token", SecretKey.tempoToken: "tempo-token"])
    var settings: AppSettings {
        var s = AppSettings()
        s.jiraBaseURL = "https://acme.atlassian.net"
        s.jiraEmail = "me@example.com"
        return s
    }

    @Test func worklogsFollowThePagesAndKeysAreLookedUpOnce() async throws {
        let http = FakeHTTP { r in
            let url = r.url!.absoluteString
            if url.contains("/rest/api/3/issue/10001") { return (200, #"{"id":"10001","key":"TAO-1"}"#) }
            if url.contains("offset=1") {
                return (
                    200,
                    #"{"metadata":{},"results":[{"tempoWorklogId":2,"issue":{"id":10001},"timeSpentSeconds":1800,"startDate":"2026-03-04","startTime":"10:00:00"}]}"#
                )
            }
            return (
                200,
                #"{"metadata":{"next":"https://api.tempo.io/4/worklogs/user/acc?offset=1"},"results":[{"tempoWorklogId":1,"issue":{"id":10001},"timeSpentSeconds":3600,"startDate":"2026-03-03","startTime":"09:00:00","description":"Fix"}]}"#
            )
        }
        let client = try TempoClient(settings: settings, secrets: secrets, http: http)
        let db = try Database.inMemory()
        let logs = try await TempoReader(db: db, client: client).worklogs(
            accountId: "acc", fromDay: "2026-03-01", toDay: "2026-03-31")

        #expect(logs.map(\.id) == [1, 2])
        #expect(logs.map(\.issueKey) == ["TAO-1", "TAO-1"])
        #expect(
            http.requests[0].url!.absoluteString
                == "https://api.tempo.io/4/worklogs/user/acc?from=2026-03-01&to=2026-03-31&limit=1000")
        #expect(http.requests[0].value(forHTTPHeaderField: "Authorization") == "Bearer tempo-token")
        #expect(http.requests.filter { $0.url!.path.contains("/issue/") }.count == 1)  // one lookup, then cached
        #expect(db.cachedIssueKey(10001) == "TAO-1")
    }

    @Test func coverageNeedsAtLeastTheLocalTimePerTicketDay() {
        let local = [
            Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "TAO-1"),
            Fixture.worklog(day: "2026-03-03", "11:00", duration: 1800, issue: "INT-12"),
            Fixture.worklog(day: "2026-03-04", "09:00", duration: 3600, issue: nil),
        ]
        let tempo = [
            TempoWorklog(
                id: 1, issueId: 1, issueKey: "TAO-1", day: "2026-03-03", startTime: "09:00:00", seconds: 3540, description: ""),
            TempoWorklog(
                id: 2, issueId: 2, issueKey: "INT-12", day: "2026-03-03", startTime: "11:00:00", seconds: 900, description: ""),
        ]
        let covered = TempoWorklog.coverage(of: local, in: tempo)
        #expect(covered == ["TAO-1|2026-03-03"])  // INT-12 has only half its time in Tempo; no ticket never matches
        let groups = TicketPlan.groups(local, inTempo: covered)
        #expect(groups.first { $0.key == "TAO-1" }?.days.first?.entered == true)
        #expect(groups.first { $0.key == "INT-12" }?.days.first?.entered == false)
    }
}
