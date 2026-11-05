import Foundation

// Services shared by several modules, behind protocols so tests can pass fakes.

/// Keychain account names of the API tokens.
public enum SecretKey {
    public static let jiraToken = "jira.apiToken"
    public static let tempoToken = "tempo.apiToken"
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
