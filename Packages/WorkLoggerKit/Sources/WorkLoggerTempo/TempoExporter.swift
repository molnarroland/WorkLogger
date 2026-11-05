import Foundation
import WorkLoggerCore
import WorkLoggerStore

/// Creates worklogs in Tempo one request per entry and marks the successful ones as exported.
/// Jira issue ids (Tempo v4 needs them) are cached in the database.
@MainActor
public struct TempoExporter {
    public let db: Database
    public let client: any TempoService

    public struct Outcome: Equatable {
        public var sent = 0
        public var failures: [String] = []

        public init(sent: Int = 0, failures: [String] = []) {
            self.sent = sent
            self.failures = failures
        }
    }

    /// `progress` gets the 1-based index of the entry being sent and the number of entries.
    public func push(_ logs: [Worklog], accountId: String, progress: (Int, Int) -> Void = { _, _ in }) async -> Outcome {
        var outcome = Outcome()
        let payloads = TempoExport.payloads(logs, accountId: accountId)
        let pending = TempoExport.exportable(logs)
        for (i, (log, payload)) in zip(pending, payloads).enumerated() {
            progress(i + 1, pending.count)
            do {
                var p = payload
                p.issueId = try await issueId(for: p.issueKey)
                var w = log
                w.tempoId = try await client.create(p)
                w.status = .exported
                w.locked = true
                try db.updateWorklog(w)
                outcome.sent += 1
            } catch {
                outcome.failures.append("\(payload.issueKey) \(payload.startDate): \(error.localizedDescription)")
            }
        }
        return outcome
    }

    private func issueId(for key: String) async throws -> Int64 {
        if let cached = db.cachedIssueId(key) { return cached }
        let id = try await client.issueId(for: key)
        try db.cacheIssueId(key, id)
        return id
    }

    public init(db: Database, client: any TempoService) {
        self.db = db
        self.client = client
    }
}
