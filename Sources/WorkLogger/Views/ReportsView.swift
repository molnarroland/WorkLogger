import SwiftUI
import Charts

struct ReportsView: View {
    @Environment(AppState.self) private var state
    @State private var period: Period = .week

    var body: some View {
        let groups = Report.group(state.worklogs, by: period)
        let total = state.worklogs.totalDuration
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
