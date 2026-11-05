import Foundation

// Services shared by several modules, behind protocols so tests can pass fakes.

/// Keychain account names of the API tokens (Jira and Tempo tokens are per account: `JiraAccount.jiraTokenKey`).
public enum SecretKey: Sendable {
    /// The single Jira and Tempo token from before accounts; moved to the first account on launch.
    public static let legacyJiraToken = "jira.apiToken"
    public static let legacyTempoToken = "tempo.apiToken"
    public static let googleSecret = "google.clientSecret"
    public static let googleRefresh = "google.refreshToken"
}

/// Where API tokens are kept. The app uses the Keychain.
public protocol SecretStore: Sendable {
    func get(_ account: String) -> String?
    func set(_ value: String?, for account: String)
}

/// Sends HTTP requests. The app uses `URLSession.shared`.
public protocol HTTPClient: Sendable {
    func send(_ request: URLRequest) async throws -> (Data, URLResponse)
}

extension URLSession: HTTPClient {
    public func send(_ request: URLRequest) async throws -> (Data, URLResponse) {
        try await data(for: request)
    }
}
