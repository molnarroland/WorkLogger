import Foundation
import Testing
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerTestSupport

@testable import WorkLoggerTempo

@Suite struct TempoClientTests {
    var settings: AppSettings {
        var s = AppSettings()
        s.jiraBaseURL = "https://acme.atlassian.net/ "
        s.jiraEmail = "me@example.com"
        s.tempoBaseURL = "https://api.tempo.io/4/"
        return s
    }
    let secrets = InMemorySecrets([SecretKey.jiraToken: "jira-token", SecretKey.tempoToken: "tempo-token"])

    func client(_ http: FakeHTTP) throws -> TempoClient {
        try TempoClient(settings: settings, secrets: secrets, http: http)
    }

    func body(_ r: URLRequest) throws -> [String: Any] {
        try #require(try JSONSerialization.jsonObject(with: r.httpBody ?? Data()) as? [String: Any])
    }

    @Test func missingConfigurationIsReported() {
        var noSite = settings
        noSite.jiraBaseURL = " / "
        #expect(throws: TempoError.self) { try TempoClient(settings: noSite, secrets: secrets, http: FakeHTTP()) }
        #expect(throws: TempoError.self) {
            try TempoClient(settings: settings, secrets: InMemorySecrets([SecretKey.tempoToken: "t"]), http: FakeHTTP())
        }
        #expect(throws: TempoError.self) {
            try TempoClient(settings: settings, secrets: InMemorySecrets([SecretKey.jiraToken: "t"]), http: FakeHTTP())
        }
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
        try db.cacheIssueId("TAO-1", 1)
        tempo.issueIds = ["TAO-2": 2]
        let logs = [
            try insert(Fixture.worklog(day: "2026-03-03", "09:00", duration: 3600, issue: "TAO-1")),
            try insert(Fixture.worklog(day: "2026-03-03", "10:00", duration: 3600, issue: "TAO-2")),
        ]

        let outcome = await TempoExporter(db: db, client: tempo).push(logs, accountId: "acc-1")

        #expect(outcome == TempoExporter.Outcome(sent: 2, failures: []))
        #expect(tempo.issueLookups == ["TAO-2"])
        #expect(tempo.created.map(\.issueId) == [1, 2])
        #expect(db.cachedIssueId("TAO-2") == 2)
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
