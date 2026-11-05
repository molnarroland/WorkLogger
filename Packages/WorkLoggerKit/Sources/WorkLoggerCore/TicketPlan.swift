import Foundation

/// The month as Tempo wants it typed by hand: per ticket, one row per day (Tempo takes one worklog per day),
/// with what is already entered. Entering by hand marks the worklogs `.exported` without a Tempo id.
public struct TicketDay: Identifiable, Hashable, Sendable {
    public var key: String?
    public var day: String
    public var seconds: Int
    public var description: String
    public var ids: [Int64]
    /// All worklogs of the day are in Tempo (sent by the API or ticked off by hand).
    public var entered: Bool

    public var id: String { (key ?? "") + "|" + day }

    public init(key: String?, day: String, seconds: Int, description: String, ids: [Int64], entered: Bool) {
        self.key = key
        self.day = day
        self.seconds = seconds
        self.description = description
        self.ids = ids
        self.entered = entered
    }
}

public struct TicketGroup: Identifiable, Hashable, Sendable {
    public var key: String?    // nil: entries without an issue
    public var days: [TicketDay]

    public var id: String { key ?? "" }
    public var seconds: Int { days.reduce(0) { $0 + $1.seconds } }
    public var enteredDays: Int { days.filter(\.entered).count }
    public var isDone: Bool { key != nil && enteredDays == days.count }

    public init(key: String?, days: [TicketDay]) {
        self.key = key
        self.days = days
    }
}

public enum TicketPlan: Sendable {
    /// "No issue" first, then tickets by time, largest first.
    public static func groups(_ logs: [Worklog]) -> [TicketGroup] {
        let timed = logs.filter { $0.duration > 0 }
        let byKey = Dictionary(grouping: timed, by: \.issueKey)
        return byKey.map { key, list in
            let days = Dictionary(grouping: list, by: \.day).map { day, items in
                let sorted = items.sorted { $0.start < $1.start }
                var seen: Set<String> = []
                let texts = sorted.map(\.description).filter { !$0.isEmpty && seen.insert($0).inserted }
                return TicketDay(key: key, day: day, seconds: items.totalDuration, description: texts.joined(separator: "; "),
                                 ids: sorted.map(\.id), entered: items.allSatisfy { $0.status == .exported })
            }
            return TicketGroup(key: key, days: days.sorted { $0.day < $1.day })
        }
        .sorted { a, b in
            if (a.key == nil) != (b.key == nil) { return a.key == nil }
            return a.seconds != b.seconds ? a.seconds > b.seconds : (a.key ?? "") < (b.key ?? "")
        }
    }

    /// Plain list for a note or for Claude: "2026-11-03  1h 15m  Daily standup".
    public static func text(_ g: TicketGroup) -> String {
        ([g.key ?? "No issue"] + g.days.map { d in
            let dur = DurationFormat.string(d.seconds)
            return "\(d.day)  \(dur.padding(toLength: max(7, dur.count), withPad: " ", startingAt: 0))  \(d.description)"
        }).joined(separator: "\n")
    }
}
