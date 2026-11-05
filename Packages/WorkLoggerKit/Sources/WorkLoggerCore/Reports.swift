import Foundation

/// Totals of worklogs per day, week or month, split by issue and by source (the Reports pane).
public enum Period: String, CaseIterable, Identifiable, Sendable {
    case day = "Daily"
    case week = "Weekly"
    case month = "Monthly"
    public var id: String { rawValue }
}

public struct ReportBucket: Identifiable, Sendable {
    public var issue: String
    public var seconds: Int
    public var id: String { issue }

    public init(issue: String, seconds: Int) {
        self.issue = issue
        self.seconds = seconds
    }
}

public struct SourceSlice: Identifiable, Sendable {
    public var source: String
    public var hours: Double
    public var id: String { source }

    public init(source: String, hours: Double) {
        self.source = source
        self.hours = hours
    }
}

public struct ReportGroup: Identifiable, Sendable {
    public var id: String
    public var label: String
    public var total: Int
    public var byIssue: [ReportBucket]
    public var bySource: [SourceSlice]

    public init(id: String, label: String, total: Int, byIssue: [ReportBucket], bySource: [SourceSlice]) {
        self.id = id
        self.label = label
        self.total = total
        self.byIssue = byIssue
        self.bySource = bySource
    }
}

public enum Report: Sendable {
    public static func group(_ logs: [Worklog], by period: Period) -> [ReportGroup] {
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
            return ReportGroup(
                id: key, label: label(key, period: period, days: items.map(\.day)),
                total: items.totalDuration, byIssue: issues, bySource: sources)
        }
    }

    private static func label(_ key: String, period: Period, days: [String]) -> String {
        switch period {
        case .day:
            return DayKey.date(key).formatted(.dateTime.weekday(.abbreviated).day().month(.abbreviated))
        case .week:
            guard let first = days.min(),
                let start = DayKey.calendar.dateInterval(of: .weekOfYear, for: DayKey.date(first))?.start
            else { return key }
            let date = start.formatted(.dateTime.day().month(.abbreviated))
            return String(localized: "Week of \(date)", bundle: .module)
        case .month:
            return DayKey.date(key + "-01").formatted(.dateTime.month(.wide).year())
        }
    }
}
