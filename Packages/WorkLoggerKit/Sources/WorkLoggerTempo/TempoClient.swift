import Foundation
import WorkLoggerCore
import WorkLoggerStore

public enum TempoError: LocalizedError {
    case missing(String)
    case http(String, Int, String)
    public var errorDescription: String? {
        switch self {
        case .missing(let what):
            return String(localized: "Missing \(what). Add it in Settings → Tempo API (optional).", bundle: .module)
        case .http(let what, let code, let body):
            return String(localized: "\(what) failed (HTTP \(code)): \(String(body.prefix(300)))", bundle: .module)
        }
    }
}

/// Jira Cloud (for account and issue ids) + Tempo REST API v4 (for creating worklogs).
public struct TempoClient: TempoService {
    public var jiraBaseURL: String
    public var jiraEmail: String
    public var jiraToken: String
    public var tempoBaseURL: String
    public var tempoToken: String
    public var http: any HTTPClient

    public init(settings: AppSettings, secrets: any SecretStore, http: any HTTPClient) throws {
        self.http = http
        jiraBaseURL = settings.jiraBaseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ "))
        jiraEmail = settings.jiraEmail
        tempoBaseURL = settings.tempoBaseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ "))
        jiraToken = secrets.get(SecretKey.jiraToken) ?? ""
        tempoToken = secrets.get(SecretKey.tempoToken) ?? ""
        if jiraBaseURL.isEmpty { throw TempoError.missing(String(localized: "the Jira site URL", bundle: .module)) }
        if jiraEmail.isEmpty || jiraToken.isEmpty {
            throw TempoError.missing(String(localized: "the Jira email or API token", bundle: .module))
        }
        if tempoToken.isEmpty { throw TempoError.missing(String(localized: "the Tempo API token", bundle: .module)) }
    }

    private func jiraRequest(_ path: String) throws -> URLRequest {
        guard let url = URL(string: jiraBaseURL + path) else {
            throw TempoError.missing(String(localized: "a valid Jira site URL", bundle: .module))
        }
        var r = URLRequest(url: url)
        r.setValue("Basic " + Data("\(jiraEmail):\(jiraToken)".utf8).base64EncodedString(), forHTTPHeaderField: "Authorization")
        r.setValue("application/json", forHTTPHeaderField: "Accept")
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

    /// GET /4/worklogs/user/{accountId}?from&to, following `metadata.next` through the pages.
    public func worklogs(accountId: String, fromDay: String, toDay: String) async throws -> [TempoWorklog] {
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

    /// Creates a worklog and returns its Tempo id.
    public func create(_ payload: TempoWorklogPayload) async throws -> Int64 {
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
