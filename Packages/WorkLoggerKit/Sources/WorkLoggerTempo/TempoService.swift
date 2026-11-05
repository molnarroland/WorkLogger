import Foundation
import WorkLoggerCore

/// Jira and Tempo calls needed to push worklogs (`TempoClient`).
public protocol TempoService: Sendable {
    func accountId() async throws -> String
    func issueId(for key: String) async throws -> Int64
    func create(_ payload: TempoWorklogPayload) async throws -> Int64
}
