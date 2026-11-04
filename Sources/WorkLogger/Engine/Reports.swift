import Foundation

/// Totals of worklogs per day, week or month, split by issue and by source (the Reports pane).
enum Period: String, CaseIterable, Identifiable {
    case day = "Daily", week = "Weekly", month = "Monthly"
    var id: String { rawValue }
}

struct ReportBucket: Identifiable {
    var issue: String
    var seconds: Int
    var id: String { issue }
}

struct SourceSlice: Identifiable {
    var source: String
    var hours: Double
    var id: String { source }
}

struct ReportGroup: Identifiable {
    var id: String
    var label: String
    var total: Int
    var byIssue: [ReportBucket]
    var bySource: [SourceSlice]
}

enum Report {
    static func group(_ logs: [Worklog], by period: Period) -> [ReportGroup] {
        let cal = DayKey.calendar
        let grouped = Dictionary(grouping: logs) { w -> String in
            switch period {
            case .day:
                return w.day
            case .week:
                let c = cal.dateComponents([.yearForWeekOfYear, .weekOfYear], from: DayKey.date(w.day))
                return String(format: "%04d-W%02d", c.yearForWeekOfYear ?? 0, c.weekOfYear ?? 0)
            case .month:
                return String(w.day.prefix(7))
            }
        }
        return grouped.keys.sorted().map { key in
            let items = grouped[key] ?? []
            let issues = Dictionary(grouping: items) { $0.issueKey ?? "No issue" }
                .map { ReportBucket(issue: $0.key, seconds: $0.value.totalDuration) }
                .sorted { $0.seconds > $1.seconds }
            let sources = WorklogSource.allCases.compactMap { src -> SourceSlice? in
                let s = items.filter { $0.source == src }.totalDuration
                return s > 0 ? SourceSlice(source: src.label, hours: Double(s) / 3600) : nil
            }
            return ReportGroup(id: key, label: label(key, period: period, days: items.map(\.day)),
                               total: items.totalDuration, byIssue: issues, bySource: sources)
        }
    }

    private static func label(_ key: String, period: Period, days: [String]) -> String {
        switch period {
        case .day:
            return DayKey.date(key).formatted(.dateTime.weekday(.abbreviated).day().month(.abbreviated))
        case .week:
            guard let first = days.min(), let start = DayKey.calendar.dateInterval(of: .weekOfYear, for: DayKey.date(first))?.start
            else { return key }
            return "Week of " + start.formatted(.dateTime.day().month(.abbreviated))
        case .month:
            return DayKey.date(key + "-01").formatted(.dateTime.month(.wide).year())
        }
    }
}
