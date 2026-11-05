import Foundation
import Testing
import WorkLoggerCore
import WorkLoggerTestSupport

@testable import WorkLoggerTempo

@Suite struct ConnectionCheckTests {
    init() { Fixture.pinTimeZone() }

    let account = JiraAccount(baseURL: "https://acme.atlassian.net", email: "me@example.com")
    var secrets: InMemorySecrets { InMemorySecrets([account.jiraTokenKey: "jira", account.tempoTokenKey: "tempo"]) }

    func check(_ http: FakeHTTP, account: JiraAccount? = nil, secrets: InMemorySecrets? = nil) -> ConnectionCheck {
        ConnectionCheck(
            account: account ?? self.account, secrets: secrets ?? self.secrets, http: http,
            now: Fixture.at("2026-03-04", "12:00"))
    }

    @Test func everythingWorks() async {
        let http = FakeHTTP { r in
            let url = r.url!.absoluteString
            if url.hasSuffix("/myself") { return (200, #"{"accountId":"acc-1","displayName":"Roland"}"#) }
            if url.contains("project/search") { return (200, "{}") }
            if r.httpMethod == "POST" { return (400, #"{"errors":[]}"#) }  // empty worklog refused as invalid
            return (200, #"{"results":[]}"#)
        }
        let c = check(http)
        let jira = await c.checkJira()
        #expect(jira.accountId == "acc-1")
        #expect(jira.lines.map(\.ok) == [true, true])
        #expect(jira.lines[0].detail.contains("Roland"))
        let tempo = await c.checkTempo(accountId: jira.accountId)
        #expect(tempo.map(\.ok) == [true, true])
        let view = http.requests.first { $0.url!.path.contains("/worklogs/user/acc-1") }
        #expect(view?.url?.query()?.contains("from=2026-03-04&to=2026-03-04") == true)
        #expect(view?.value(forHTTPHeaderField: "Authorization") == "Bearer tempo")
    }

    @Test func rejectedJiraTokenStopsBeforeIssues() async {
        let c = check(FakeHTTP { _ in (401, "") })
        let jira = await c.checkJira()
        #expect(jira.accountId == nil)
        #expect(jira.lines.map(\.ok) == [false, nil])
        #expect(jira.lines[0].detail.contains("rejected"))
    }

    @Test func missingReadWorkScopeIsNamed() async {
        let c = check(FakeHTTP { r in r.url!.absoluteString.hasSuffix("/myself") ? (200, #"{"accountId":"a"}"#) : (403, "") })
        let jira = await c.checkJira()
        #expect(jira.lines.map(\.ok) == [true, false])
        #expect(jira.lines[1].detail.contains("read:jira-work"))
    }

    @Test func tempoAddressAndPermissions() async {
        let wrongAddress = await check(FakeHTTP { _ in (404, "") }).checkTempo(accountId: "a")
        #expect(wrongAddress.map(\.ok) == [false, nil])
        #expect(wrongAddress[0].detail.contains("api.tempo.io"))

        let viewOnly = await check(FakeHTTP { r in r.httpMethod == "POST" ? (403, "") : (200, "{}") }).checkTempo(accountId: "a")
        #expect(viewOnly.map(\.ok) == [true, false])
        #expect(viewOnly[1].detail.contains("Manage worklogs"))
    }

    @Test func missingFieldsAreNamedWithoutCalls() async {
        var empty = account
        empty.baseURL = ""
        let http = FakeHTTP()
        let jira = await check(http, account: empty, secrets: InMemorySecrets()).checkJira()
        #expect(jira.lines.map(\.ok) == [false, nil])
        let tempo = await check(http, secrets: InMemorySecrets()).checkTempo(accountId: "a")
        #expect(tempo[0].ok == false)
        #expect(http.requests.isEmpty)
    }
}
