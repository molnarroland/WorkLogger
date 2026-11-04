import Foundation

/// Imported work sessions and the HR system's day records.
extension Database {

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
}
