import Foundation

/// The month as Tempo wants it typed by hand: per ticket, one row per day (Tempo takes one worklog per day),
/// with what is already entered. Entering by hand marks the worklogs `.exported` without a Tempo id.
struct TicketDay: Identifiable, Hashable {
    var key: String?
    var day: String
    var seconds: Int
    var description: String
    var ids: [Int64]
    /// All worklogs of the day are in Tempo (sent by the API or ticked off by hand).
    var entered: Bool

    var id: String { (key ?? "") + "|" + day }
}

struct TicketGroup: Identifiable, Hashable {
    var key: String?    // nil: entries without an issue
    var days: [TicketDay]

    var id: String { key ?? "" }
    var seconds: Int { days.reduce(0) { $0 + $1.seconds } }
    var enteredDays: Int { days.filter(\.entered).count }
    var isDone: Bool { key != nil && enteredDays == days.count }
}

enum TicketPlan {
    /// "No issue" first, then tickets by time, largest first.
    static func groups(_ logs: [Worklog]) -> [TicketGroup] {
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
    static func text(_ g: TicketGroup) -> String {
        ([g.key ?? "No issue"] + g.days.map { d in
            let dur = DurationFormat.string(d.seconds)
            return "\(d.day)  \(dur.padding(toLength: max(7, dur.count), withPad: " ", startingAt: 0))  \(d.description)"
        }).joined(separator: "\n")
    }
}
