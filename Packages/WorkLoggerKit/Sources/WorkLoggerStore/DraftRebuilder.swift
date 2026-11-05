import Foundation
import WorkLoggerCore

/// Replaces the unlocked drafts of a range with fresh ones built from stored commits, meetings and sessions.
public struct DraftRebuilder {
    public let db: Database
    public let builder: WorklogBuilder

    /// How far outside the range commits are looked for: leave and idle rest days hand their commits to the
    /// closest workday, which may lie just outside the range.
    public static let paddingDays = 31

    public func rebuild(_ range: DateRange) throws {
        let on = builder.settings.modules
        try db.transaction {
            try db.deleteRegeneratable(fromDay: range.fromDay, toDay: range.toDay)
            let cal = DayKey.calendar
            func shift(_ d: Date, _ n: Int) -> Date { cal.date(byAdding: .day, value: n, to: d)! }
            let pad = Self.paddingDays
            let padFrom = shift(range.start, -pad)
            let padTo = shift(range.end, pad)
            // Twice the padding, so the closest-workday search from a padded leave day sees real day types.
            let records =
                on.contains(.workTimes)
                ? Dictionary(
                    uniqueKeysWithValues: try db.dayRecords(
                        fromDay: DayKey.key(shift(range.start, -2 * pad)), toDay: DayKey.key(shift(range.end, 2 * pad))
                    ).map { ($0.day, $0) }) : [:]
            let workedDays =
                on.contains(.workTimes)
                ? Set(
                    try db.sessions(fromDay: DayKey.key(padFrom), toDay: DayKey.key(padTo))
                        .filter { $0.kind == .work }.map(\.day)) : []
            let rangeDays = range.days
            let inRange = Set(rangeDays)
            var commitsByDay: [String: [StoredCommit]] = [:]
            for day in on.contains(.git) ? DayKey.days(from: padFrom, to: padTo) : [] {
                let target = builder.commitDay(for: day, info: { records[$0] }, worked: { workedDays.contains($0) })
                guard inRange.contains(day) || target != day else { continue }
                commitsByDay[target, default: []] += try db.commits(day: day)
            }
            // Entries that survived the delete (locked, moved, reviewed) own their commits wherever they now sit.
            let claimed = Set(try db.worklogs(fromDay: DayKey.key(padFrom), toDay: DayKey.key(padTo)).flatMap(\.refs))
            for (day, list) in commitsByDay { commitsByDay[day] = list.filter { !claimed.contains($0.hash) } }
            for day in rangeDays {
                let input = DayInput(
                    day: day, sessions: on.contains(.workTimes) ? try db.sessions(day: day) : [],
                    commits: commitsByDay[day] ?? [],
                    meetings: on.contains(.calendar) ? try db.meetings(day: day).filter { !claimed.contains($0.id) } : [],
                    kept: try db.worklogs(day: day),
                    info: records[day])
                for w in builder.build(input) { try db.insertWorklog(w) }
            }
        }
    }

    public init(db: Database, builder: WorklogBuilder) {
        self.db = db
        self.builder = builder
    }
}
