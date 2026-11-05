import Foundation
import WorkLoggerCore
import WorkLoggerTempo

// Fakes used by the module tests and the app tests (through a symlink).

/// Secrets kept in a dictionary; empty values remove the entry, like the Keychain wrapper.
public final class InMemorySecrets: SecretStore, @unchecked Sendable {
    private let lock = NSLock()
    private var values: [String: String]

    public init(_ values: [String: String] = [:]) { self.values = values }

    public func get(_ account: String) -> String? { lock.withLock { values[account] } }
    public func set(_ value: String?, for account: String) {
        lock.withLock { values[account] = (value?.isEmpty ?? true) ? nil : value }
    }
}

/// Answers requests with `handler` and records them.
public final class FakeHTTP: HTTPClient, @unchecked Sendable {
    private let lock = NSLock()
    private var recorded: [URLRequest] = []
    public var handler: @Sendable (URLRequest) -> (status: Int, body: String)

    public init(_ handler: @escaping @Sendable (URLRequest) -> (status: Int, body: String) = { _ in (200, "{}") }) {
        self.handler = handler
    }

    public var requests: [URLRequest] { lock.withLock { recorded } }

    public func send(_ request: URLRequest) async throws -> (Data, URLResponse) {
        lock.withLock { recorded.append(request) }
        let (status, body) = handler(request)
        let response = HTTPURLResponse(url: request.url!, statusCode: status, httpVersion: nil, headerFields: nil)!
        return (Data(body.utf8), response)
    }
}

public struct FakeError: LocalizedError {
    public var message = "boom"
    public var errorDescription: String? { message }

    public init(message: String = "boom") {
        self.message = message
    }
}

public final class FakeTempo: TempoService {
    public init() {}
    public var account = "acc-1"
    public var issueIds: [String: Int64] = [:]
    public var failingKeys: Set<String> = []
    public var nextWorklogId: Int64 = 100
    public private(set) var created: [TempoWorklogPayload] = []
    public private(set) var issueLookups: [String] = []

    public func accountId() async throws -> String { account }
    public func issueId(for key: String) async throws -> Int64 {
        issueLookups.append(key)
        guard let id = issueIds[key] else { throw FakeError(message: "\(key) not found") }
        return id
    }
    public func create(_ payload: TempoWorklogPayload) async throws -> Int64 {
        if failingKeys.contains(payload.issueKey) { throw FakeError(message: "rejected") }
        created.append(payload)
        defer { nextWorklogId += 1 }
        return nextWorklogId
    }
}
