import Foundation
import WorkLoggerCore
import WorkLoggerStore

public enum TempoError: LocalizedError {
    case missing(String)
    case http(String, Int, String)
    public var errorDescription: String? {
        switch self {
        case .missing(let what): return "Missing \(what). Add it in Settings → Jira & Tempo."
        case .http(let what, let code, let body): return "\(what) failed (HTTP \(code)): \(body.prefix(300))"
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
        if jiraBaseURL.isEmpty { throw TempoError.missing("the Jira site URL") }
        if jiraEmail.isEmpty || jiraToken.isEmpty { throw TempoError.missing("the Jira email or API token") }
        if tempoToken.isEmpty { throw TempoError.missing("the Tempo API token") }
    }

    private func jiraRequest(_ path: String) throws -> URLRequest {
        guard let url = URL(string: jiraBaseURL + path) else { throw TempoError.missing("a valid Jira site URL") }
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
        let data = try await send(try jiraRequest("/rest/api/3/myself"), "Reading your Jira account")
        return try JSONDecoder().decode(Me.self, from: data).accountId
    }

    public func issueId(for key: String) async throws -> Int64 {
        struct Issue: Decodable { let id: String }
        let encoded = key.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? key
        let data = try await send(try jiraRequest("/rest/api/3/issue/\(encoded)?fields=summary"), "Looking up \(key)")
        guard let id = Int64(try JSONDecoder().decode(Issue.self, from: data).id) else {
            throw TempoError.http("Looking up \(key)", 200, "non-numeric id")
        }
        return id
    }

    /// Creates a worklog and returns its Tempo id.
    public func create(_ payload: TempoWorklogPayload) async throws -> Int64 {
        struct Body: Encodable {
            let issueId: Int64, authorAccountId: String, startDate: String, startTime: String
            let timeSpentSeconds: Int, description: String
        }
        struct Created: Decodable { let tempoWorklogId: Int64 }
        guard let issueId = payload.issueId, let author = payload.authorAccountId else {
            throw TempoError.missing("the issue id or account id")
        }
        guard let url = URL(string: tempoBaseURL + "/worklogs") else { throw TempoError.missing("a valid Tempo API URL") }
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("Bearer \(tempoToken)", forHTTPHeaderField: "Authorization")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONEncoder().encode(Body(issueId: issueId, authorAccountId: author,
                                                     startDate: payload.startDate, startTime: payload.startTime,
                                                     timeSpentSeconds: payload.timeSpentSeconds,
                                                     description: payload.description))
        let data = try await send(req, "Creating the \(payload.issueKey) worklog")
        return try JSONDecoder().decode(Created.self, from: data).tempoWorklogId
    }
}
