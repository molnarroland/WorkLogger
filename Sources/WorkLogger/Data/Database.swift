import Foundation
import SQLite3

private let SQLITE_TRANSIENT = unsafeBitCast(-1, to: sqlite3_destructor_type.self)

enum DBError: LocalizedError {
    case open(String), prepare(String), step(String)
    var errorDescription: String? {
        switch self {
        case .open(let m), .prepare(let m), .step(let m): return m
        }
    }
}

enum SQLValue {
    case int(Int64), double(Double), text(String), null

    static func of(_ s: String?) -> SQLValue { s.map { .text($0) } ?? .null }
    static func of(_ i: Int64?) -> SQLValue { i.map { .int($0) } ?? .null }
    static func of(_ i: Int) -> SQLValue { .int(Int64(i)) }
    static func of(_ b: Bool) -> SQLValue { .int(b ? 1 : 0) }
    static func of(_ d: Date) -> SQLValue { .int(Int64(d.timeIntervalSince1970.rounded())) }
}

struct Row {
    let values: [String: SQLValue]

    func string(_ k: String) -> String? {
        switch values[k] {
        case .text(let s)?: return s
        case .int(let i)?: return String(i)
        case .double(let d)?: return String(d)
        default: return nil
        }
    }
    func int(_ k: String) -> Int64? {
        switch values[k] {
        case .int(let i)?: return i
        case .double(let d)?: return Int64(d)
        case .text(let s)?: return Int64(s)
        default: return nil
        }
    }
    func intValue(_ k: String) -> Int { Int(int(k) ?? 0) }
    func bool(_ k: String) -> Bool { (int(k) ?? 0) != 0 }
    func date(_ k: String) -> Date { Date(timeIntervalSince1970: TimeInterval(int(k) ?? 0)) }
}

/// Minimal, dependency-free SQLite wrapper (uses the system libsqlite3).
final class Database {
    private var handle: OpaquePointer?
    let url: URL

    init(url: URL) throws {
        self.url = url
        let flags = SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_FULLMUTEX
        guard sqlite3_open_v2(url.path, &handle, flags, nil) == SQLITE_OK else {
            throw DBError.open(String(cString: sqlite3_errmsg(handle)))
        }
        try execute("PRAGMA journal_mode=WAL;")
        try execute("PRAGMA foreign_keys=ON;")
        try migrate()
    }

    deinit { sqlite3_close(handle) }

    private var lastError: String { String(cString: sqlite3_errmsg(handle)) }

    func execute(_ sql: String) throws {
        var err: UnsafeMutablePointer<CChar>?
        if sqlite3_exec(handle, sql, nil, nil, &err) != SQLITE_OK {
            let msg = err.map { String(cString: $0) } ?? lastError
            sqlite3_free(err)
            throw DBError.step(msg)
        }
    }

    private func prepare(_ sql: String, _ params: [SQLValue]) throws -> OpaquePointer? {
        var stmt: OpaquePointer?
        guard sqlite3_prepare_v2(handle, sql, -1, &stmt, nil) == SQLITE_OK else {
            throw DBError.prepare("\(lastError) — in: \(sql)")
        }
        for (i, p) in params.enumerated() {
            let idx = Int32(i + 1)
            switch p {
            case .int(let v): sqlite3_bind_int64(stmt, idx, v)
            case .double(let v): sqlite3_bind_double(stmt, idx, v)
            case .text(let v): sqlite3_bind_text(stmt, idx, v, -1, SQLITE_TRANSIENT)
            case .null: sqlite3_bind_null(stmt, idx)
            }
        }
        return stmt
    }

    @discardableResult
    func run(_ sql: String, _ params: [SQLValue] = []) throws -> Int64 {
        let stmt = try prepare(sql, params)
        defer { sqlite3_finalize(stmt) }
        let rc = sqlite3_step(stmt)
        guard rc == SQLITE_DONE || rc == SQLITE_ROW else { throw DBError.step(lastError) }
        return sqlite3_last_insert_rowid(handle)
    }

    func query(_ sql: String, _ params: [SQLValue] = []) throws -> [Row] {
        let stmt = try prepare(sql, params)
        defer { sqlite3_finalize(stmt) }
        var rows: [Row] = []
        while true {
            let rc = sqlite3_step(stmt)
            if rc == SQLITE_DONE { break }
            guard rc == SQLITE_ROW else { throw DBError.step(lastError) }
            var dict: [String: SQLValue] = [:]
            for i in 0..<sqlite3_column_count(stmt) {
                let name = String(cString: sqlite3_column_name(stmt, i))
                switch sqlite3_column_type(stmt, i) {
                case SQLITE_INTEGER: dict[name] = .int(sqlite3_column_int64(stmt, i))
                case SQLITE_FLOAT: dict[name] = .double(sqlite3_column_double(stmt, i))
                case SQLITE_TEXT:
                    if let text = sqlite3_column_text(stmt, i) { dict[name] = .text(String(cString: text)) } else { dict[name] = .text("") }
                default: dict[name] = .null
                }
            }
            rows.append(Row(values: dict))
        }
        return rows
    }

    func transaction(_ body: () throws -> Void) throws {
        try execute("BEGIN IMMEDIATE")
        do {
            try body()
            try execute("COMMIT")
        } catch {
            try? execute("ROLLBACK")
            throw error
        }
    }

    // MARK: - Schema

    private func migrate() throws {
        let version = try query("PRAGMA user_version").first?.intValue("user_version") ?? 0
        if version < 1 {
            try execute("""
            CREATE TABLE IF NOT EXISTS repos(
                id INTEGER PRIMARY KEY,
                path TEXT UNIQUE NOT NULL,
                name TEXT NOT NULL,
                issue_key TEXT,
                project_key TEXT,
                enabled INTEGER NOT NULL DEFAULT 1
            );
            CREATE TABLE IF NOT EXISTS projects(
                key TEXT PRIMARY KEY,
                default_issue TEXT
            );
            CREATE TABLE IF NOT EXISTS commits(
                repo_id INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
                hash TEXT NOT NULL,
                ts INTEGER NOT NULL,
                author TEXT,
                email TEXT,
                subject TEXT NOT NULL DEFAULT '',
                body TEXT NOT NULL DEFAULT '',
                ref TEXT NOT NULL DEFAULT '',
                files INTEGER NOT NULL DEFAULT 0,
                insertions INTEGER NOT NULL DEFAULT 0,
                deletions INTEGER NOT NULL DEFAULT 0,
                PRIMARY KEY(repo_id, hash)
            );
            CREATE INDEX IF NOT EXISTS idx_commits_ts ON commits(ts);
            CREATE TABLE IF NOT EXISTS sessions(
                id INTEGER PRIMARY KEY,
                day TEXT NOT NULL,
                start_ts INTEGER NOT NULL,
                end_ts INTEGER NOT NULL,
                kind TEXT NOT NULL,
                label TEXT NOT NULL DEFAULT ''
            );
            CREATE INDEX IF NOT EXISTS idx_sessions_day ON sessions(day);
            CREATE TABLE IF NOT EXISTS meetings(
                event_id TEXT PRIMARY KEY,
                calendar_id TEXT NOT NULL,
                title TEXT NOT NULL DEFAULT '',
                details TEXT NOT NULL DEFAULT '',
                start_ts INTEGER NOT NULL,
                end_ts INTEGER NOT NULL,
                skipped INTEGER NOT NULL DEFAULT 0
            );
            CREATE INDEX IF NOT EXISTS idx_meetings_start ON meetings(start_ts);
            CREATE TABLE IF NOT EXISTS worklogs(
                id INTEGER PRIMARY KEY,
                day TEXT NOT NULL,
                start_ts INTEGER NOT NULL,
                duration INTEGER NOT NULL,
                issue_key TEXT,
                description TEXT NOT NULL DEFAULT '',
                source TEXT NOT NULL,
                source_ref TEXT NOT NULL DEFAULT '',
                repo_id INTEGER,
                estimated INTEGER NOT NULL DEFAULT 0,
                status TEXT NOT NULL DEFAULT 'draft',
                locked INTEGER NOT NULL DEFAULT 0,
                tempo_id INTEGER
            );
            CREATE INDEX IF NOT EXISTS idx_worklogs_day ON worklogs(day);
            CREATE TABLE IF NOT EXISTS issue_cache(
                key TEXT PRIMARY KEY,
                issue_id INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS settings(
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );
            PRAGMA user_version = 1;
            """)
        }
        if version < 2 {
            try execute("""
            CREATE TABLE IF NOT EXISTS day_info(
                day TEXT PRIMARY KEY,
                status TEXT NOT NULL,
                label TEXT NOT NULL DEFAULT '',
                reported_worked INTEGER,
                reported_breaks INTEGER
            );
            PRAGMA user_version = 2;
            """)
        }
    }
}
