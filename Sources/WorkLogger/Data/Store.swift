import Foundation

/// Typed persistence on top of `Database`.
extension Database {

    // MARK: Settings

    func loadSettings() -> AppSettings {
        guard let raw = (try? query("SELECT value FROM settings WHERE key = 'app'"))?.first?.string("value"),
              let data = raw.data(using: .utf8),
              let settings = AppSettings.decodeMerging(data)
        else { return AppSettings() }
        return settings
    }

    func saveSettings(_ s: AppSettings) throws {
        let json = String(decoding: try JSONEncoder().encode(s), as: UTF8.self)
        try run("INSERT INTO settings(key, value) VALUES('app', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
                [.text(json)])
    }

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

    // MARK: Commits

    func saveCommits(_ list: [StoredCommit]) throws {
        for c in list {
            try run("""
            INSERT INTO commits(repo_id, hash, ts, author, email, subject, body, ref, files, insertions, deletions)
            VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(repo_id, hash) DO UPDATE SET
                ref = excluded.ref, files = excluded.files,
                insertions = excluded.insertions, deletions = excluded.deletions
            """, [.int(c.repoId), .text(c.hash), .of(c.date), .text(c.author), .text(c.email),
                  .text(c.subject), .text(c.body), .text(c.ref),
                  .of(c.filesChanged), .of(c.insertions), .of(c.deletions)])
        }
    }

    func commits(day: String) throws -> [StoredCommit] {
        let (start, end) = DayKey.bounds(day)
        return try query("""
            SELECT c.* FROM commits c JOIN repos r ON r.id = c.repo_id
            WHERE r.enabled = 1 AND c.ts >= ? AND c.ts < ? ORDER BY c.ts
            """, [.of(start), .of(end)]).map {
            StoredCommit(repoId: $0.int("repo_id") ?? 0, hash: $0.string("hash") ?? "", date: $0.date("ts"),
                         author: $0.string("author") ?? "", email: $0.string("email") ?? "",
                         subject: $0.string("subject") ?? "", body: $0.string("body") ?? "",
                         ref: $0.string("ref") ?? "", filesChanged: $0.intValue("files"),
                         insertions: $0.intValue("insertions"), deletions: $0.intValue("deletions"))
        }
    }

    // MARK: Work sessions

    private func session(from r: Row) -> WorkSession {
        WorkSession(id: r.int("id"), day: r.string("day") ?? "", start: r.date("start_ts"), end: r.date("end_ts"),
                    kind: SessionKind(rawValue: r.string("kind") ?? "") ?? .work, label: r.string("label") ?? "")
    }

    func sessions(day: String) throws -> [WorkSession] {
        try query("SELECT * FROM sessions WHERE day = ? ORDER BY start_ts", [.text(day)]).map(session)
    }

    func sessions(fromDay: String, toDay: String) throws -> [WorkSession] {
        try query("SELECT * FROM sessions WHERE day >= ? AND day <= ? ORDER BY start_ts",
                  [.text(fromDay), .text(toDay)]).map(session)
    }

    /// Replaces the sessions and day records of every day in the import (days without sessions, e.g. leave, are cleared too).
    func importTimesheet(_ t: ParsedTimesheet) throws {
        try transaction {
            for day in t.dayKeys {
                try run("DELETE FROM sessions WHERE day = ?", [.text(day)])
            }
            for s in t.sessions {
                try run("INSERT INTO sessions(day, start_ts, end_ts, kind, label) VALUES(?, ?, ?, ?, ?)",
                        [.text(s.day), .of(s.start), .of(s.end), .text(s.kind.rawValue), .text(s.label)])
            }
            for d in t.days {
                try run("""
                INSERT INTO day_info(day, status, label, reported_worked, reported_breaks) VALUES(?, ?, ?, ?, ?)
                ON CONFLICT(day) DO UPDATE SET status = excluded.status, label = excluded.label,
                    reported_worked = excluded.reported_worked, reported_breaks = excluded.reported_breaks
                """, [.text(d.day), .text(d.status.rawValue), .text(d.label),
                      .of(d.reportedWorked.map { Int64($0) }), .of(d.reportedBreaks.map { Int64($0) })])
            }
        }
    }

    func deleteSessions(fromDay: String, toDay: String) throws {
        try transaction {
            try run("DELETE FROM sessions WHERE day >= ? AND day <= ?", [.text(fromDay), .text(toDay)])
            try run("DELETE FROM day_info WHERE day >= ? AND day <= ?", [.text(fromDay), .text(toDay)])
        }
    }

    // MARK: Day records (status and totals from the HR system)

    private func dayRecord(from r: Row) -> DayRecord {
        DayRecord(day: r.string("day") ?? "", status: DayStatus(rawValue: r.string("status") ?? "") ?? .other,
                  label: r.string("label") ?? "",
                  reportedWorked: r.int("reported_worked").map { Int($0) },
                  reportedBreaks: r.int("reported_breaks").map { Int($0) })
    }

    func dayRecord(day: String) throws -> DayRecord? {
        try query("SELECT * FROM day_info WHERE day = ?", [.text(day)]).first.map(dayRecord)
    }

    func dayRecords(fromDay: String, toDay: String) throws -> [DayRecord] {
        try query("SELECT * FROM day_info WHERE day >= ? AND day <= ? ORDER BY day",
                  [.text(fromDay), .text(toDay)]).map(dayRecord)
    }

    // MARK: Meetings

    private func meeting(from r: Row) -> Meeting {
        Meeting(id: r.string("event_id") ?? "", calendarId: r.string("calendar_id") ?? "",
                title: r.string("title") ?? "", details: r.string("details") ?? "",
                start: r.date("start_ts"), end: r.date("end_ts"), skipped: r.bool("skipped"))
    }

    func meetings(day: String) throws -> [Meeting] {
        let (start, end) = DayKey.bounds(day)
        return try query("SELECT * FROM meetings WHERE start_ts >= ? AND start_ts < ? ORDER BY start_ts",
                         [.of(start), .of(end)]).map(meeting)
    }

    func meetings(from: Date, to: Date) throws -> [Meeting] {
        try query("SELECT * FROM meetings WHERE start_ts >= ? AND start_ts < ? ORDER BY start_ts",
                  [.of(from), .of(to)]).map(meeting)
    }

    /// Stores the events of the given calendars for an interval. Events deleted in the calendar, and events of
    /// calendars that are no longer read, are removed. Keeps the "skipped" choice.
    func syncMeetings(from: Date, to: Date, calendarIds: Set<String>, activeCalendarIds: Set<String>, events: [Meeting]) throws {
        try transaction {
            let ids = Set(events.map(\.id))
            for existing in try meetings(from: from, to: to) {
                let stale = calendarIds.contains(existing.calendarId) && !ids.contains(existing.id)
                if stale || !activeCalendarIds.contains(existing.calendarId) {
                    try run("DELETE FROM meetings WHERE event_id = ?", [.text(existing.id)])
                }
            }
            for e in events {
                try run("""
                INSERT INTO meetings(event_id, calendar_id, title, details, start_ts, end_ts, skipped)
                VALUES(?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(event_id) DO UPDATE SET title = excluded.title, details = excluded.details,
                    start_ts = excluded.start_ts, end_ts = excluded.end_ts
                """, [.text(e.id), .text(e.calendarId), .text(e.title), .text(e.details),
                      .of(e.start), .of(e.end), .of(e.skipped)])
            }
        }
    }

    func setMeetingSkipped(_ id: String, _ skipped: Bool) throws {
        try run("UPDATE meetings SET skipped = ? WHERE event_id = ?", [.of(skipped), .text(id)])
    }

    // MARK: Worklogs

    private func worklog(from r: Row) -> Worklog {
        Worklog(id: r.int("id") ?? 0, day: r.string("day") ?? "", start: r.date("start_ts"),
                duration: r.intValue("duration"), issueKey: r.string("issue_key"),
                description: r.string("description") ?? "",
                source: WorklogSource(rawValue: r.string("source") ?? "") ?? .manual,
                sourceRef: r.string("source_ref") ?? "", repoId: r.int("repo_id"),
                estimated: r.intValue("estimated"),
                status: WorklogStatus(rawValue: r.string("status") ?? "") ?? .draft,
                locked: r.bool("locked"), tempoId: r.int("tempo_id"))
    }

    func worklogs(fromDay: String, toDay: String) throws -> [Worklog] {
        try query("SELECT * FROM worklogs WHERE day >= ? AND day <= ? ORDER BY day, start_ts",
                  [.text(fromDay), .text(toDay)]).map(worklog)
    }

    func worklogs(day: String) throws -> [Worklog] {
        try worklogs(fromDay: day, toDay: day)
    }

    @discardableResult
    func insertWorklog(_ w: Worklog) throws -> Int64 {
        try run("""
        INSERT INTO worklogs(day, start_ts, duration, issue_key, description, source, source_ref, repo_id, estimated, status, locked, tempo_id)
        VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, [.text(w.day), .of(w.start), .of(w.duration), .of(w.issueKey), .text(w.description),
              .text(w.source.rawValue), .text(w.sourceRef), .of(w.repoId), .of(w.estimated),
              .text(w.status.rawValue), .of(w.locked), .of(w.tempoId)])
    }

    func updateWorklog(_ w: Worklog) throws {
        try run("""
        UPDATE worklogs SET day = ?, start_ts = ?, duration = ?, issue_key = ?, description = ?,
            status = ?, locked = ?, tempo_id = ? WHERE id = ?
        """, [.text(w.day), .of(w.start), .of(w.duration), .of(w.issueKey), .text(w.description),
              .text(w.status.rawValue), .of(w.locked), .of(w.tempoId), .int(w.id)])
    }

    func deleteWorklog(_ id: Int64) throws {
        try run("DELETE FROM worklogs WHERE id = ?", [.int(id)])
    }

    /// Removes generated drafts so they can be rebuilt. Locked, reviewed, exported and manual entries stay.
    func deleteRegeneratable(fromDay: String, toDay: String) throws {
        try run("""
        DELETE FROM worklogs WHERE day >= ? AND day <= ? AND locked = 0 AND status = 'draft'
            AND source IN ('git', 'meeting', 'absence', 'period')
        """, [.text(fromDay), .text(toDay)])
    }

    // MARK: Jira issue id cache (Tempo API v4 needs numeric issue ids)

    func cachedIssueId(_ key: String) -> Int64? {
        (try? query("SELECT issue_id FROM issue_cache WHERE key = ?", [.text(key)]))?.first?.int("issue_id")
    }

    func cacheIssueId(_ key: String, _ id: Int64) throws {
        try run("INSERT OR REPLACE INTO issue_cache(key, issue_id) VALUES(?, ?)", [.text(key), .int(id)])
    }
}
