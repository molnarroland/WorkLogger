import Foundation
import WorkLoggerCore

/// Worklogs and the Jira issue id cache.
extension Database {

    // MARK: Worklogs

    private func worklog(from r: Row) -> Worklog {
        Worklog(
            id: r.int("id") ?? 0, day: r.string("day") ?? "", start: r.date("start_ts"),
            duration: r.intValue("duration"), issueKey: r.string("issue_key"),
            description: r.string("description") ?? "",
            source: WorklogSource(rawValue: r.string("source") ?? "") ?? .manual,
            sourceRef: r.string("source_ref") ?? "", repoId: r.int("repo_id"),
            estimated: r.intValue("estimated"),
            status: WorklogStatus(rawValue: r.string("status") ?? "") ?? .draft,
            locked: r.bool("locked"), tempoId: r.int("tempo_id"))
    }

    public func worklogs(fromDay: String, toDay: String) throws -> [Worklog] {
        try query(
            "SELECT * FROM worklogs WHERE day >= ? AND day <= ? ORDER BY day, start_ts",
            [.text(fromDay), .text(toDay)]
        ).map(worklog)
    }

    public func worklogs(day: String) throws -> [Worklog] {
        try worklogs(fromDay: day, toDay: day)
    }

    @discardableResult
    public func insertWorklog(_ w: Worklog) throws -> Int64 {
        try run(
            """
            INSERT INTO worklogs(day, start_ts, duration, issue_key, description, source, source_ref, repo_id, estimated, status, locked, tempo_id)
            VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            [
                .text(w.day), .of(w.start), .of(w.duration), .of(w.issueKey), .text(w.description),
                .text(w.source.rawValue), .text(w.sourceRef), .of(w.repoId), .of(w.estimated),
                .text(w.status.rawValue), .of(w.locked), .of(w.tempoId),
            ])
    }

    public func updateWorklog(_ w: Worklog) throws {
        try run(
            """
            UPDATE worklogs SET day = ?, start_ts = ?, duration = ?, issue_key = ?, description = ?,
                status = ?, locked = ?, tempo_id = ? WHERE id = ?
            """,
            [
                .text(w.day), .of(w.start), .of(w.duration), .of(w.issueKey), .text(w.description),
                .text(w.status.rawValue), .of(w.locked), .of(w.tempoId), .int(w.id),
            ])
    }

    public func deleteWorklog(_ id: Int64) throws {
        try run("DELETE FROM worklogs WHERE id = ?", [.int(id)])
    }

    /// Removes generated drafts so they can be rebuilt. Locked, reviewed, exported and manual entries stay.
    public func deleteRegeneratable(fromDay: String, toDay: String) throws {
        try run(
            """
            DELETE FROM worklogs WHERE day >= ? AND day <= ? AND locked = 0 AND status = 'draft'
                AND source IN ('git', 'meeting', 'absence', 'period')
            """, [.text(fromDay), .text(toDay)])
    }

    // MARK: Jira issue id cache (Tempo API v4 needs numeric issue ids), per Jira site

    public func cachedIssueId(_ key: String, site: String) -> Int64? {
        (try? query("SELECT issue_id FROM issue_cache WHERE site = ? AND key = ?", [.text(site), .text(key)]))?.first?
            .int("issue_id")
    }

    /// The issue key for a Jira issue id seen before (pushing or reading Tempo).
    public func cachedIssueKey(_ id: Int64, site: String) -> String? {
        (try? query("SELECT key FROM issue_cache WHERE site = ? AND issue_id = ?", [.text(site), .int(id)]))?.first?
            .string("key")
    }

    public func cacheIssueId(_ key: String, _ id: Int64, site: String) throws {
        try run(
            "INSERT OR REPLACE INTO issue_cache(site, key, issue_id) VALUES(?, ?, ?)", [.text(site), .text(key), .int(id)])
    }
}
