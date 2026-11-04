import SwiftUI
import Charts

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
                .map { ReportBucket(issue: $0.key, seconds: $0.value.reduce(0) { $0 + $1.duration }) }
                .sorted { $0.seconds > $1.seconds }
            let sources = WorklogSource.allCases.compactMap { src -> SourceSlice? in
                let s = items.filter { $0.source == src }.reduce(0) { $0 + $1.duration }
                return s > 0 ? SourceSlice(source: src.label, hours: Double(s) / 3600) : nil
            }
            return ReportGroup(id: key, label: label(key, period: period, days: items.map(\.day)),
                               total: items.reduce(0) { $0 + $1.duration }, byIssue: issues, bySource: sources)
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

struct ReportsView: View {
    @Environment(AppState.self) private var state
    @State private var period: Period = .week

    var body: some View {
        let groups = Report.group(state.worklogs, by: period)
        let total = state.worklogs.reduce(0) { $0 + $1.duration }
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 12) {
                Text("Totals").font(.title2.weight(.semibold))
                Spacer()
                Picker("Period", selection: $period) {
                    ForEach(Period.allCases) { Text($0.rawValue).tag($0) }
                }
                .pickerStyle(.segmented)
                .labelsHidden()
                .frame(width: 260)
            }
            .padding(12)
            Divider()

            if groups.isEmpty {
                ContentUnavailableView("Nothing logged in this range", systemImage: "chart.bar",
                                       description: Text("Collect worklogs on the Review page first."))
            } else {
                HStack(alignment: .firstTextBaseline) {
                    Text(DurationFormat.string(total)).font(.title2).monospacedDigit()
                    Text("in \(state.fromDay) – \(state.toDay)").foregroundStyle(.secondary)
                }
                .padding([.horizontal, .top], 16)

                Chart(groups) { g in
                    ForEach(g.bySource) { s in
                        BarMark(x: .value("Period", g.label), y: .value("Hours", s.hours))
                            .foregroundStyle(by: .value("Source", s.source))
                    }
                }
                .chartYAxisLabel("hours")
                .frame(height: 220)
                .padding(16)

                List(groups) { g in
                    DisclosureGroup {
                        ForEach(g.byIssue) { b in
                            HStack {
                                Text(b.issue).foregroundStyle(b.issue == "No issue" ? Color.orange : Color.primary)
                                Spacer()
                                Text(DurationFormat.string(b.seconds)).monospacedDigit()
                                Text(DurationFormat.hours(b.seconds)).monospacedDigit()
                                    .foregroundStyle(.secondary).frame(width: 70, alignment: .trailing)
                            }
                        }
                    } label: {
                        HStack {
                            Text(g.label).font(.headline)
                            Spacer()
                            Text(DurationFormat.string(g.total)).monospacedDigit()
                            Text(DurationFormat.hours(g.total)).monospacedDigit()
                                .foregroundStyle(.secondary).frame(width: 70, alignment: .trailing)
                        }
                    }
                }
                .listStyle(.inset)
            }
        }
    }
}

#Preview {
    ReportsView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
