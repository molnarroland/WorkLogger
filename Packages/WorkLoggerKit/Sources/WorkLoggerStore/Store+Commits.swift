import Foundation
import WorkLoggerCore

/// Commits read from git, with ignored paths already removed from the stats.
extension Database {

    // MARK: Commits

    public func saveCommits(_ list: [StoredCommit]) throws {
        for c in list {
            try run(
                """
                INSERT INTO commits(repo_id, hash, ts, author, email, subject, body, ref, files, insertions, deletions)
                VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(repo_id, hash) DO UPDATE SET
                    ref = excluded.ref, files = excluded.files,
                    insertions = excluded.insertions, deletions = excluded.deletions
                """,
                [
                    .int(c.repoId), .text(c.hash), .of(c.date), .text(c.author), .text(c.email),
                    .text(c.subject), .text(c.body), .text(c.ref),
                    .of(c.filesChanged), .of(c.insertions), .of(c.deletions),
                ])
        }
    }

    /// Your stored commits per repository in [from, to).
    public func commitCounts(from: Date, to: Date) throws -> [Int64: Int] {
        Dictionary(
            uniqueKeysWithValues: try query(
                "SELECT repo_id, COUNT(*) AS n FROM commits WHERE ts >= ? AND ts < ? GROUP BY repo_id",
                [.of(from), .of(to)]
            ).map { ($0.int("repo_id") ?? 0, $0.intValue("n")) })
    }

    public func commits(day: String) throws -> [StoredCommit] {
        let (start, end) = DayKey.bounds(day)
        return try query(
            """
            SELECT c.* FROM commits c JOIN repos r ON r.id = c.repo_id
            WHERE r.enabled = 1 AND c.ts >= ? AND c.ts < ? ORDER BY c.ts
            """, [.of(start), .of(end)]
        ).map {
            StoredCommit(
                repoId: $0.int("repo_id") ?? 0, hash: $0.string("hash") ?? "", date: $0.date("ts"),
                author: $0.string("author") ?? "", email: $0.string("email") ?? "",
                subject: $0.string("subject") ?? "", body: $0.string("body") ?? "",
                ref: $0.string("ref") ?? "", filesChanged: $0.intValue("files"),
                insertions: $0.intValue("insertions"), deletions: $0.intValue("deletions"))
        }
    }
}
