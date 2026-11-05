import Foundation
import WorkLoggerCore

/// Repositories and their Jira project mappings.
public extension Database {

    // MARK: Repositories & projects

    func repos() throws -> [Repo] {
        try query("SELECT * FROM repos ORDER BY name COLLATE NOCASE").map {
            Repo(id: $0.int("id") ?? 0, path: $0.string("path") ?? "", name: $0.string("name") ?? "",
                 issueKey: $0.string("issue_key"), projectKey: $0.string("project_key"), enabled: $0.bool("enabled"))
        }
    }

    func addRepo(path: String, name: String) throws {
        try run("INSERT OR IGNORE INTO repos(path, name) VALUES(?, ?)", [.text(path), .text(name)])
    }

    func updateRepo(_ r: Repo) throws {
        try run("UPDATE repos SET name = ?, issue_key = ?, project_key = ?, enabled = ? WHERE id = ?",
                [.text(r.name), .of(r.issueKey), .of(r.projectKey), .of(r.enabled), .int(r.id)])
    }

    func deleteRepo(_ id: Int64) throws {
        try run("DELETE FROM repos WHERE id = ?", [.int(id)])
    }

    func projects() throws -> [ProjectMapping] {
        try query("SELECT * FROM projects ORDER BY key").map {
            ProjectMapping(key: $0.string("key") ?? "", defaultIssue: $0.string("default_issue"))
        }
    }

    func upsertProject(_ p: ProjectMapping) throws {
        try run("INSERT INTO projects(key, default_issue) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET default_issue = excluded.default_issue",
                [.text(p.key), .of(p.defaultIssue)])
    }

    func deleteProject(_ key: String) throws {
        try run("DELETE FROM projects WHERE key = ?", [.text(key)])
    }
}
