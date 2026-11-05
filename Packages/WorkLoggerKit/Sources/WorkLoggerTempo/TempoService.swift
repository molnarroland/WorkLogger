import Foundation
import WorkLoggerCore

/// Jira and Tempo calls needed to push worklogs (`TempoClient`).
public protocol TempoService: Sendable {
    func accountId() async throws -> String
    func issueId(for key: String) async throws -> Int64
    func create(_ payload: TempoWorklogPayload) async throws -> Int64
    /// Your worklogs in Tempo for the days (inclusive), all pages.
    func worklogs(accountId: String, fromDay: String, toDay: String) async throws -> [TempoWorklog]
    /// The Jira key of an issue id (e.g. 10123 → "TAO-1").
    func issueKey(for id: Int64) async throws -> String
}
