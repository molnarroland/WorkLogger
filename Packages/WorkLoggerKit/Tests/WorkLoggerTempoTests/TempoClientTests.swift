import Foundation
import Testing
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerTestSupport

@testable import WorkLoggerTempo

@Suite struct TempoClientTests {
    init() { Fixture.pinTimeZone() }

    var account: JiraAccount {
        var a = JiraAccount(baseURL: "https://acme.atlassian.net/ ", email: "me@example.com")
        a.id = UUID(uuidString: "00000000-0000-0000-0000-000000000001")!
        a.tempoBaseURL = "https://api.tempo.io/4/"
        return a
    }
    var secrets: InMemorySecrets {
        InMemorySecrets([account.jiraTokenKey: "jira-token", account.tempoTokenKey: "tempo-token"])
    }

    func client(_ http: FakeHTTP, provider: WorklogProvider = .tempo) throws -> TempoClient {
        var a = account
        a.provider = provider
        return try TempoClient(account: a, secrets: secrets, http: http)
    }

    func body(_ r: URLRequest) throws -> [String: Any] {
        try #require(try JSONSerialization.jsonObject(with: r.httpBody ?? Data()) as? [String: Any])
    }

    @Test func missingConfigurationIsReported() {
        var noSite = account
        noSite.baseURL = " / "
        #expect(throws: TempoError.self) { try TempoClient(account: noSite, secrets: secrets, http: FakeHTTP()) }
        #expect(throws: TempoError.self) {
            try TempoClient(account: account, secrets: InMemorySecrets([account.tempoTokenKey: "t"]), http: FakeHTTP())
        }
        #expect(throws: TempoError.self) {
            try TempoClient(account: account, secrets: InMemorySecrets([account.jiraTokenKey: "t"]), http: FakeHTTP())
        }
        var jira = account
        jira.provider = .jira  // Jira's own worklogs need no Tempo token
        #expect(throws: Never.self) {
            try TempoClient(account: jira, secrets: InMemorySecrets([account.jiraTokenKey: "t"]), http: FakeHTTP())
        }
    }

    @Test func projectKeysFollowThePages() async throws {
        let http = FakeHTTP { r in
            r.url!.query!.contains("startAt=0")
                ? (200, #"{"values":[{"key":"TAO"},{"key":"INT"}],"isLast":false}"#)
                : (200, #"{"values":[{"key":"HR"}],"isLast":true}"#)
        }
        #expect(try await client(http).projectKeys() == ["TAO", "INT", "HR"])
        #expect(http.requests.map { $0.url!.query! } == ["maxResults=100&startAt=0", "maxResults=100&startAt=2"])
    }

    @Test func jiraProviderCreatesTheWorklogOnTheIssue() async throws {
        let http = FakeHTTP { _ in (201, #"{"id": "555"}"#) }
        let payload = TempoWorklogPayload(
            issueKey: "TAO-1", issueId: 10001, authorAccountId: "acc-1",
            startDate: "2026-03-03", startTime: "09:00:00", timeSpentSeconds: 5400, description: "Fix")
        #expect(try await client(http, provider: .jira).create(payload) == 555)
        let request = try #require(http.requests.first)
        #expect(request.httpMethod == "POST")
        #expect(request.url?.absoluteString == "https://acme.atlassian.net/rest/api/3/issue/TAO-1/worklog")
        #expect(request.value(forHTTPHeaderField: "Authorization")?.hasPrefix("Basic ") == true)
        let json = try body(request)
        #expect(json["started"] as? String == "2026-03-03T09:00:00.000+0100")
        #expect(json["timeSpentSeconds"] as? Int == 5400)
        let doc = try #require(
            try? JSONDecoder().decode(ADF.self, from: JSONSerialization.data(withJSONObject: json["comment"]!)))
        #expect(doc.text == "Fix")
    }

    @Test func jiraProviderReadsYourWorklogsOfTheRange() async throws {
        let http = FakeHTTP { r in
            let url = r.url!.absoluteString
            if url.contains("/search/jql") { return (200, #"{"issues":[{"id":"10001","key":"TAO-1"}]}"#) }
            return (
                200,
                #"{"total":3,"worklogs":[{"id":"1","author":{"accountId":"acc-1"},"started":"2026-03-03T09:00:00.000+0100","timeSpentSeconds":3600,"comment":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Fix"}]}]}},{"id":"2","author":{"accountId":"someone"},"started":"2026-03-03T10:00:00.000+0100","timeSpentSeconds":60},{"id":"3","author":{"accountId":"acc-1"},"started":"2026-04-01T10:00:00.000+0100","timeSpentSeconds":60}]}"#
            )
        }
        let logs = try await client(http, provider: .jira).worklogs(
            accountId: "acc-1", fromDay: "2026-03-01", toDay: "2026-03-31")
        #expect(
            logs == [
                TempoWorklog(
                    id: 1, issueId: 10001, issueKey: "TAO-1", day: "2026-03-03", startTime: "09:00:00", seconds: 3600,
                    description: "Fix")
            ])
        let jql = try #require(
            URLComponents(url: http.requests[0].url!, resolvingAgainstBaseURL: false)?.queryItems?.first?.value)
        #expect(jql == #"worklogAuthor = currentUser() AND worklogDate >= "2026-03-01" AND worklogDate <= "2026-03-31""#)
        #expect(http.requests[1].url!.path == "/rest/api/3/issue/10001/worklog")
    }

    @Test func accountIdUsesJiraBasicAuth() async throws {
        let http = FakeHTTP { _ in (200, #"{"accountId": "acc-1", "displayName": "Me"}"#) }
        #expect(try await client(http).accountId() == "acc-1")
        let request = try #require(http.requests.first)
        #expect(request.url?.absoluteString == "https://acme.atlassian.net/rest/api/3/myself")
        let expected = "Basic " + Data("me@example.com:jira-token".utf8).base64EncodedString()
        #expect(request.value(forHTTPHeaderField: "Authorization") == expected)
    }

    @Test func issueIdIsNumeric() async throws {
        let http = FakeHTTP { _ in (200, #"{"id": "10001", "key": "TAO-1"}"#) }
        #expect(try await client(http).issueId(for: "TAO-1") == 10001)
        #expect(http.requests.first?.url?.absoluteString == "https://acme.atlassian.net/rest/api/3/issue/TAO-1?fields=summary")

        let odd = FakeHTTP { _ in (200, #"{"id": "abc"}"#) }
        await #expect(throws: TempoError.self) { try await client(odd).issueId(for: "TAO-1") }
    }

    @Test func createPostsTheWorklogWithTheTempoToken() async throws {
        let http = FakeHTTP { _ in (200, #"{"tempoWorklogId": 77}"#) }
        let payload = TempoWorklogPayload(
            issueKey: "TAO-1", issueId: 10001, authorAccountId: "acc-1",
            startDate: "2026-03-03", startTime: "09:00:00",
            timeSpentSeconds: 5400, description: "Fix")
        #expect(try await client(http).create(payload) == 77)

        let request = try #require(http.requests.first)
        #expect(request.httpMethod == "POST")
        #expect(request.url?.absoluteString == "https://api.tempo.io/4/worklogs")
        #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer tempo-token")
        let json = try body(request)
        #expect(json["issueId"] as? Int == 10001)
        #expect(json["authorAccountId"] as? String == "acc-1")
        #expect(json["startTime"] as? String == "09:00:00")
        #expect(json["timeSpentSeconds"] as? Int == 5400)
        #expect(json["issueKey"] == nil)  // Tempo v4 takes the numeric id only
    }

    @Test func createNeedsIssueAndAccountIds() async throws {
        let payload = TempoWorklogPayload(
            issueKey: "TAO-1", issueId: nil, authorAccountId: "acc-1", startDate: "",
            startTime: "", timeSpentSeconds: 60, description: "")
        let http = FakeHTTP()
        await #expect(throws: TempoError.self) { try await client(http).create(payload) }
        #expect(http.requests.isEmpty)
    }

    @Test func httpErrorsCarryStatusAndBody() async throws {
        let http = FakeHTTP { _ in (401, "Unauthorized") }
        do {
            _ = try await client(http).accountId()
            Issue.record("Expected an error")
        } catch {
            #expect(error.localizedDescription == "Reading your Jira account failed (HTTP 401): Unauthorized")
        }
    }
}

@MainActor
@Suite struct TempoExporterTests {
    let db: Database
    let tempo = FakeTempo()

    init() throws {
        Fixture.pinTimeZone()
        db = try Database.inMemory()
    }

    func insert(_ w: Worklog) throws -> Worklog {
        var w = w
        w.id = try db.insertWorklog(w)
        return w
    }

    @Test func usesCachedIssueIdsAndCachesNewOnes() async throws {
        try db.cacheIssueId("TAO-1", 1, site: tempo.site)
        tempo.issueIds = ["TAO-2": 2]
        let logs = [
            try insert(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "TAO-1")),
            try insert(Fixture.worklog(day: "2026-03-03", "10:00", duration: 3600, issue: "TAO-2")),
        ]

        let outcome = await TempoExporter(db: db, client: tempo).push(logs, accountId: "acc-1")

        #expect(outcome == TempoExporter.Outcome(sent: 2, failures: []))
        #expect(tempo.issueLookups == ["TAO-2"])
        #expect(tempo.created.map(\.issueId) == [1, 2])
        #expect(db.cachedIssueId("TAO-2", site: tempo.site) == 2)
    }

    @Test func successfulEntriesAreMarkedExportedAndFailuresAreReported() async throws {
        tempo.issueIds = ["TAO-1": 1, "TAO-2": 2]
        tempo.failingKeys = ["TAO-2"]
        let logs = [
            try insert(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "TAO-1")),
            try insert(Fixture.worklog(day: "2026-03-03", "10:00", duration: 3600, issue: "TAO-2")),
            try insert(Fixture.worklog(day: "2026-03-03", "11:00", duration: 3600, issue: nil)),
        ]
        var progress: [[Int]] = []

        let outcome = await TempoExporter(db: db, client: tempo).push(logs, accountId: "acc-1") { progress.append([$0, $1]) }

        #expect(outcome.sent == 1)
        #expect(outcome.failures == ["TAO-2 2026-03-03: rejected"])
        #expect(progress == [[1, 2], [2, 2]])
        let stored = try db.worklogs(day: "2026-03-03")
        #expect(stored.map(\.status) == [.exported, .draft, .draft])
        #expect(stored.map(\.tempoId) == [100, nil, nil])
        #expect(stored.map(\.locked) == [true, false, false])
    }

    @Test func unknownIssueIsAFailure() async throws {
        let logs = [try insert(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "NOPE-1"))]
        let outcome = await TempoExporter(db: db, client: tempo).push(logs, accountId: "acc-1")
        #expect(outcome.failures == ["NOPE-1 2026-03-03: NOPE-1 not found"])
        #expect(tempo.created.isEmpty)
    }
}
