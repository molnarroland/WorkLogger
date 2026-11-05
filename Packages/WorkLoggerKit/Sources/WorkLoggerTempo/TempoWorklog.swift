import Foundation
import WorkLoggerCore
import WorkLoggerStore

/// A worklog that already exists in Tempo (yours, read back for the selected dates).
public struct TempoWorklog: Identifiable, Hashable, Sendable {
    public var id: Int64
    public var issueId: Int64
    /// Filled from the Jira issue id; nil when the issue can't be looked up.
    public var issueKey: String?
    public var day: String  // yyyy-MM-dd as Tempo stores it
    public var startTime: String  // HH:mm:ss
    public var seconds: Int
    public var description: String

    public init(
        id: Int64, issueId: Int64, issueKey: String? = nil, day: String, startTime: String, seconds: Int,
        description: String
    ) {
        self.id = id
        self.issueId = issueId
        self.issueKey = issueKey
        self.day = day
        self.startTime = startTime
        self.seconds = seconds
        self.description = description
    }
}

extension TempoWorklog {
    /// The "KEY|day" ticket-days whose time Tempo already has: Tempo's time for the ticket on that day is at least
    /// what WorkLogger has (a minute of rounding allowed). Entries without a ticket never match.
    public static func coverage(of logs: [Worklog], in tempo: [TempoWorklog]) -> Set<String> {
        var inTempo: [String: Int] = [:]
        for t in tempo { if let key = t.issueKey { inTempo["\(key)|\(t.day)", default: 0] += t.seconds } }
        var local: [String: Int] = [:]
        for w in logs where w.duration > 0 { if let key = w.issueKey { local["\(key)|\(w.day)", default: 0] += w.duration } }
        return Set(local.filter { ref, seconds in (inTempo[ref] ?? 0) >= seconds - 60 }.keys)
    }
}

/// Reads your worklogs (Tempo or Jira) for a range and names their issues. Tempo v4 only returns Jira issue ids, so keys are
/// looked up once and kept in the database's issue cache (the same one pushing uses).
@MainActor
public struct TempoReader {
    public let db: Database
    public let client: any TempoService

    public init(db: Database, client: any TempoService) {
        self.db = db
        self.client = client
    }

    public func worklogs(accountId: String, fromDay: String, toDay: String) async throws -> [TempoWorklog] {
        var logs = try await client.worklogs(accountId: accountId, fromDay: fromDay, toDay: toDay)
        var keys: [Int64: String] = [:]
        for i in logs.indices where logs[i].issueKey == nil {  // Jira worklogs come with their keys
            let id = logs[i].issueId
            if keys[id] == nil {
                if let cached = db.cachedIssueKey(id, site: client.site) {
                    keys[id] = cached
                } else if let key = try? await client.issueKey(for: id) {
                    try? db.cacheIssueId(key, id, site: client.site)
                    keys[id] = key
                }
            }
            logs[i].issueKey = keys[id]
        }
        return logs
    }
}
