import Foundation
import WorkLoggerCore
import WorkLoggerStore

public enum TempoError: LocalizedError {
    case missing(String)
    case http(String, Int, String)
    public var errorDescription: String? {
        switch self {
        case .missing(let what):
            return String(localized: "Missing \(what). Add it in Settings → Jira.", bundle: .module)
        case .http(let what, let code, let body):
            return String(localized: "\(what) failed (HTTP \(code)): \(String(body.prefix(300)))", bundle: .module)
        }
    }
}

/// One Jira account: Jira Cloud (account, issue ids, project keys) plus its worklog provider, Tempo REST API v4 or
/// Jira's own worklogs.
public struct TempoClient: TempoService {
    public var jiraBaseURL: String
    public var jiraEmail: String
    public var jiraToken: String
    public var provider: WorklogProvider
    public var tempoBaseURL: String
    public var tempoToken: String
    public var http: any HTTPClient
    public var site: String { jiraBaseURL }

    public init(account: JiraAccount, secrets: any SecretStore, http: any HTTPClient) throws {
        self.http = http
        jiraBaseURL = account.site
        jiraEmail = account.email
        provider = account.provider
        tempoBaseURL = account.tempoBaseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ "))
        jiraToken = secrets.get(account.jiraTokenKey) ?? ""
        tempoToken = secrets.get(account.tempoTokenKey) ?? ""
        if jiraBaseURL.isEmpty { throw TempoError.missing(String(localized: "the Jira site URL", bundle: .module)) }
        if jiraEmail.isEmpty || jiraToken.isEmpty {
            throw TempoError.missing(String(localized: "the Jira email or API token", bundle: .module))
        }
        if provider == .tempo, tempoToken.isEmpty {
            throw TempoError.missing(String(localized: "the Tempo API token", bundle: .module))
        }
    }

    private func jiraRequest(_ path: String, method: String = "GET", body: Data? = nil) throws -> URLRequest {
        guard let url = URL(string: jiraBaseURL + path) else {
            throw TempoError.missing(String(localized: "a valid Jira site URL", bundle: .module))
        }
        var r = URLRequest(url: url)
        r.setValue("Basic " + Data("\(jiraEmail):\(jiraToken)".utf8).base64EncodedString(), forHTTPHeaderField: "Authorization")
        r.setValue("application/json", forHTTPHeaderField: "Accept")
        r.httpMethod = method
        if let body {
            r.setValue("application/json", forHTTPHeaderField: "Content-Type")
            r.httpBody = body
        }
        return r
    }

    private func send(_ req: URLRequest, _ what: String) async throws -> Data {
        let (data, resp) = try await http.send(req)
        let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
        guard (200..<300).contains(code) else { throw TempoError.http(what, code, String(decoding: data, as: UTF8.self)) }
        return data
    }

    public func accountId() async throws -> String {
        struct Me: Decodable { let accountId: String }
        let data = try await send(
            try jiraRequest("/rest/api/3/myself"), String(localized: "Reading your Jira account", bundle: .module))
        return try JSONDecoder().decode(Me.self, from: data).accountId
    }

    public func issueId(for key: String) async throws -> Int64 {
        struct Issue: Decodable { let id: String }
        let encoded = key.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? key
        let data = try await send(
            try jiraRequest("/rest/api/3/issue/\(encoded)?fields=summary"),
            String(localized: "Looking up \(key)", bundle: .module))
        guard let id = Int64(try JSONDecoder().decode(Issue.self, from: data).id) else {
            throw TempoError.http(String(localized: "Looking up \(key)", bundle: .module), 200, "non-numeric id")
        }
        return id
    }

    public func issueKey(for id: Int64) async throws -> String {
        struct Issue: Decodable { let key: String }
        let data = try await send(
            try jiraRequest("/rest/api/3/issue/\(id)?fields=summary"),
            String(localized: "Looking up \(String(id))", bundle: .module))
        return try JSONDecoder().decode(Issue.self, from: data).key
    }

    /// The site's project keys (GET /rest/api/3/project/search, all pages).
    public func projectKeys() async throws -> [String] {
        struct Page: Decodable {
            struct Project: Decodable { let key: String }
            let values: [Project]
            let isLast: Bool?
        }
        var keys: [String] = []
        while true {
            let data = try await send(
                try jiraRequest("/rest/api/3/project/search?maxResults=100&startAt=\(keys.count)"),
                String(localized: "Reading the Jira projects", bundle: .module))
            let page = try JSONDecoder().decode(Page.self, from: data)
            keys += page.values.map(\.key)
            if page.isLast ?? true || page.values.isEmpty { return keys }
        }
    }

    /// Your worklogs for the days (inclusive), from Tempo or from Jira.
    public func worklogs(accountId: String, fromDay: String, toDay: String) async throws -> [TempoWorklog] {
        provider == .jira
            ? try await jiraWorklogs(accountId: accountId, fromDay: fromDay, toDay: toDay)
            : try await tempoWorklogs(accountId: accountId, fromDay: fromDay, toDay: toDay)
    }

    /// GET /4/worklogs/user/{accountId}?from&to, following `metadata.next` through the pages.
    func tempoWorklogs(accountId: String, fromDay: String, toDay: String) async throws -> [TempoWorklog] {
        struct Page: Decodable {
            struct Metadata: Decodable { let next: String? }
            struct Result: Decodable {
                struct Issue: Decodable { let id: Int64 }
                let tempoWorklogId: Int64
                let issue: Issue
                let timeSpentSeconds: Int
                let startDate: String
                let startTime: String?
                let description: String?
            }
            let metadata: Metadata?
            let results: [Result]
        }
        var c = URLComponents(string: tempoBaseURL + "/worklogs/user/\(accountId)")
        c?.queryItems = [
            .init(name: "from", value: fromDay), .init(name: "to", value: toDay), .init(name: "limit", value: "1000"),
        ]
        var next = c?.url
        guard next != nil else { throw TempoError.missing(String(localized: "a valid Tempo API URL", bundle: .module)) }
        var out: [TempoWorklog] = []
        while let url = next {
            var req = URLRequest(url: url)
            req.setValue("Bearer \(tempoToken)", forHTTPHeaderField: "Authorization")
            req.setValue("application/json", forHTTPHeaderField: "Accept")
            let data = try await send(req, String(localized: "Reading your Tempo worklogs", bundle: .module))
            let page = try JSONDecoder().decode(Page.self, from: data)
            out += page.results.map {
                TempoWorklog(
                    id: $0.tempoWorklogId, issueId: $0.issue.id, day: $0.startDate, startTime: $0.startTime ?? "00:00:00",
                    seconds: $0.timeSpentSeconds, description: $0.description ?? "")
            }
            next = page.metadata?.next.flatMap(URL.init(string:))
        }
        return out
    }

    /// Jira's own worklogs: the issues you logged on in the range (JQL), then each issue's worklogs that are yours.
    func jiraWorklogs(accountId: String, fromDay: String, toDay: String) async throws -> [TempoWorklog] {
        struct Search: Decodable {
            struct Issue: Decodable { let id: String, key: String }
            let issues: [Issue]
            let nextPageToken: String?
        }
        struct Logs: Decodable {
            struct Entry: Decodable {
                struct Author: Decodable { let accountId: String }
                let id: String
                let author: Author?
                let started: String  // 2026-03-03T09:00:00.000+0100
                let timeSpentSeconds: Int
                let comment: ADF?
            }
            let worklogs: [Entry]
            let total: Int?
        }
        let jql = "worklogAuthor = currentUser() AND worklogDate >= \"\(fromDay)\" AND worklogDate <= \"\(toDay)\""
        let what = String(localized: "Reading your Jira worklogs", bundle: .module)
        var issues: [Search.Issue] = []
        var token: String?
        repeat {
            var c = URLComponents()
            c.queryItems = [
                .init(name: "jql", value: jql), .init(name: "fields", value: "key"), .init(name: "maxResults", value: "100"),
            ]
            if let token { c.queryItems?.append(.init(name: "nextPageToken", value: token)) }
            let page = try JSONDecoder().decode(
                Search.self,
                from: try await send(try jiraRequest("/rest/api/3/search/jql?" + (c.percentEncodedQuery ?? "")), what))
            issues += page.issues
            token = page.nextPageToken
        } while token != nil
        let after = Int64(DayKey.bounds(fromDay).0.timeIntervalSince1970 * 1000)
        let before = Int64(DayKey.bounds(toDay).1.timeIntervalSince1970 * 1000)
        var out: [TempoWorklog] = []
        for issue in issues {
            guard let issueId = Int64(issue.id) else { continue }
            var startAt = 0
            while true {
                let data = try await send(
                    try jiraRequest(
                        "/rest/api/3/issue/\(issue.id)/worklog?startedAfter=\(after)&startedBefore=\(before)&startAt=\(startAt)&maxResults=1000"
                    ), what)
                let page = try JSONDecoder().decode(Logs.self, from: data)
                for w in page.worklogs where w.author?.accountId == accountId {
                    let day = String(w.started.prefix(10))
                    guard day >= fromDay, day <= toDay, let id = Int64(w.id) else { continue }
                    out.append(
                        TempoWorklog(
                            id: id, issueId: issueId, issueKey: issue.key, day: day,
                            startTime: String(w.started.dropFirst(11).prefix(8)), seconds: w.timeSpentSeconds,
                            description: w.comment?.text ?? ""))
                }
                startAt += page.worklogs.count
                if page.worklogs.isEmpty || startAt >= page.total ?? 0 { break }
            }
        }
        return out
    }

    /// Creates a worklog (in Tempo, or on the Jira issue) and returns its id.
    public func create(_ payload: TempoWorklogPayload) async throws -> Int64 {
        provider == .jira ? try await createInJira(payload) : try await createInTempo(payload)
    }

    /// POST /rest/api/3/issue/{key}/worklog with the start in your time zone and the description as the comment.
    func createInJira(_ payload: TempoWorklogPayload) async throws -> Int64 {
        struct Body: Encodable { let started: String, timeSpentSeconds: Int, comment: ADF }
        struct Created: Decodable { let id: String }
        guard let start = DayKey.at(payload.startDate, hm: String(payload.startTime.prefix(5))) else {
            throw TempoError.missing(String(localized: "the start of the worklog", bundle: .module))
        }
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.timeZone = DayKey.timeZone
        f.dateFormat = "yyyy-MM-dd'T'HH:mm:ss.SSSZ"
        let encoded = payload.issueKey.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? payload.issueKey
        let body = try JSONEncoder().encode(
            Body(started: f.string(from: start), timeSpentSeconds: payload.timeSpentSeconds, comment: ADF(payload.description)))
        let data = try await send(
            try jiraRequest("/rest/api/3/issue/\(encoded)/worklog", method: "POST", body: body),
            String(localized: "Creating the \(payload.issueKey) worklog", bundle: .module))
        guard let id = Int64(try JSONDecoder().decode(Created.self, from: data).id) else {
            throw TempoError.http(
                String(localized: "Creating the \(payload.issueKey) worklog", bundle: .module), 200, "non-numeric id")
        }
        return id
    }

    /// Creates a worklog in Tempo and returns its Tempo id.
    func createInTempo(_ payload: TempoWorklogPayload) async throws -> Int64 {
        struct Body: Encodable {
            let issueId: Int64, authorAccountId: String, startDate: String, startTime: String
            let timeSpentSeconds: Int, description: String
        }
        struct Created: Decodable { let tempoWorklogId: Int64 }
        guard let issueId = payload.issueId, let author = payload.authorAccountId else {
            throw TempoError.missing(String(localized: "the issue id or account id", bundle: .module))
        }
        guard let url = URL(string: tempoBaseURL + "/worklogs") else {
            throw TempoError.missing(String(localized: "a valid Tempo API URL", bundle: .module))
        }
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("Bearer \(tempoToken)", forHTTPHeaderField: "Authorization")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONEncoder().encode(
            Body(
                issueId: issueId, authorAccountId: author,
                startDate: payload.startDate, startTime: payload.startTime,
                timeSpentSeconds: payload.timeSpentSeconds,
                description: payload.description))
        let data = try await send(req, String(localized: "Creating the \(payload.issueKey) worklog", bundle: .module))
        return try JSONDecoder().decode(Created.self, from: data).tempoWorklogId
    }
}

/// Jira's rich text (Atlassian Document Format), reduced to plain paragraphs of text.
struct ADF: Codable {
    struct Node: Codable {
        var type: String
        var text: String?
        var content: [Node]?
    }
    var type = "doc"
    var version = 1
    var content: [Node]

    init(_ text: String) {
        content = [Node(type: "paragraph", content: text.isEmpty ? [] : [Node(type: "text", text: text)])]
    }

    /// The text of all text nodes; paragraphs joined by spaces.
    var text: String {
        func flat(_ n: Node) -> String { n.text ?? (n.content ?? []).map(flat).joined() }
        return content.map(flat).filter { !$0.isEmpty }.joined(separator: " ")
    }
}
