import Foundation

/// Calendar events.
extension Database {

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
}
