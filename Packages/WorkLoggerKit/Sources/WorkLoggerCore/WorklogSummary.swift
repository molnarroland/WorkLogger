import Foundation

public extension Worklog {
    /// Has time but no issue: Tempo can't take it, so exports skip it.
    var needsIssue: Bool { issueKey == nil && duration > 0 }
}

public extension Sequence where Element == Worklog {
    /// Logged time in seconds.
    var totalDuration: Int { reduce(0) { $0 + $1.duration } }

    var missingIssue: [Worklog] { filter(\.needsIssue) }

    /// Logged time per source, largest first; sources without time are left out.
    var totalsBySource: [(source: WorklogSource, seconds: Int)] {
        Dictionary(grouping: self, by: \.source)
            .map { (source: $0.key, seconds: $0.value.totalDuration) }
            .filter { $0.seconds > 0 }
            .sorted { $0.seconds != $1.seconds ? $0.seconds > $1.seconds : $0.source.rawValue < $1.source.rawValue }
    }
}

public extension Sequence where Element == WorkSession {
    var workIntervals: [Interval] { filter { $0.kind == .work }.map { Interval(start: $0.start, end: $0.end) } }
    var pauseIntervals: [Interval] { filter { $0.kind == .pause }.map { Interval(start: $0.start, end: $0.end) } }
    /// Work sessions with the breaks taken out.
    var netWork: [Interval] { Intervals.subtract(workIntervals, pauseIntervals) }
}

/// How a timesheet day compares to what it should add up to; the day cell shows it as a colour.
public enum DayProgress: Equatable, Sendable {
    /// Leave or a public holiday.
    case absence
    /// Nothing expected and nothing logged (a free rest day).
    case free
    /// Nothing expected but work was logged.
    case extra
    case complete
    /// Short of the target, but the day hasn't come yet.
    case upcoming
    case empty
    case partial

    public init(logged: Int, expected: Int, absence: Bool, future: Bool) {
        if absence { self = .absence }
        else if expected == 0 { self = logged > 0 ? .extra : .free }
        else if logged >= expected { self = .complete }
        else if future { self = .upcoming }
        else { self = logged == 0 ? .empty : .partial }
    }
}

/// Dragged entries travel as "worklogs:1,2,3" so unrelated text drops are ignored. Exported entries can't move.
public enum WorklogDrag: Sendable {
    public static func payload(_ logs: [Worklog]) -> String {
        "worklogs:" + logs.filter { $0.status != .exported }.map { String($0.id) }.joined(separator: ",")
    }
    public static func ids(_ text: String) -> [Int64] {
        guard text.hasPrefix("worklogs:") else { return [] }
        return text.dropFirst("worklogs:".count).split(separator: ",").compactMap { Int64($0) }
    }
}
