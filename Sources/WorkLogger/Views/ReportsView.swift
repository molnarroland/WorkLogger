import SwiftUI
import Charts
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

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
                ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                HStack(alignment: .firstTextBaseline) {
                    Text(DurationFormat.string(total)).font(.title2).monospacedDigit()
                    Text("in \(state.fromDay) – \(state.toDay)").foregroundStyle(.secondary)
                }
                TicketTotals(groups: TicketPlan.groups(state.worklogs))
                VStack(alignment: .leading, spacing: 8) {
                    Text("By source").font(.headline)
                    SourceStack(logs: state.worklogs, height: 18)
                    HStack(spacing: 14) {
                        ForEach(state.worklogs.filter { !$0.needsIssue }.totalsBySource, id: \.source) { s in
                            Label { Text("\(s.source.label) \(DurationFormat.string(s.seconds))") } icon: {
                                RoundedRectangle(cornerRadius: 2).fill(s.source.color).frame(width: 10, height: 10) }
                        }
                        let none = state.worklogs.missingIssue.totalDuration
                        if none > 0 {
                            Label { Text("No ticket \(DurationFormat.string(none))") } icon: {
                                RoundedRectangle(cornerRadius: 2).fill(.orange).frame(width: 10, height: 10) }
                        }
                    }
                    .font(.callout).foregroundStyle(.secondary)
                }
                .padding(14).glassEffect(.regular, in: .rect(cornerRadius: 18))

                Chart(groups) { g in
                    ForEach(g.bySource) { s in
                        BarMark(x: .value("Period", g.label), y: .value("Hours", s.hours))
                            .foregroundStyle(by: .value("Source", s.source))
                    }
                }
                .chartYAxisLabel("hours")
                .chartForegroundStyleScale(domain: WorklogSource.allCases.map(\.label), range: WorklogSource.allCases.map(\.color))
                .frame(height: 220)
                .padding(.vertical, 8)

                ForEach(groups) { g in
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
                }
                .padding(16)
                }
            }
        }
    }
}

/// Each ticket's time, split into what is already in Tempo (green) and what is still to enter.
struct TicketTotals: View {
    let groups: [TicketGroup]

    var body: some View {
        let top = max(groups.map(\.seconds).max() ?? 1, 1)
        VStack(alignment: .leading, spacing: 8) {
            Text("By ticket").font(.headline)
            ForEach(groups) { g in
                let entered = g.days.filter(\.entered).reduce(0) { $0 + $1.seconds }
                HStack(spacing: 10) {
                    Text(g.key ?? "No issue").font(.callout.monospaced().weight(.medium))
                        .foregroundStyle(g.key == nil ? Color.orange : Color.primary).frame(width: 100, alignment: .leading)
                    GeometryReader { geo in
                        HStack(spacing: 0) {
                            Color.green.frame(width: geo.size.width * CGFloat(entered) / CGFloat(top))
                            (g.key == nil ? Color.orange : Color.accentColor)
                                .frame(width: geo.size.width * CGFloat(g.seconds - entered) / CGFloat(top))
                            Spacer(minLength: 0)
                        }
                    }
                    .frame(height: 12).background(.quaternary).clipShape(.capsule)
                    Text(DurationFormat.string(g.seconds)).monospacedDigit().frame(width: 70, alignment: .trailing)
                }
            }
            HStack(spacing: 14) {
                Label { Text("In Tempo") } icon: { RoundedRectangle(cornerRadius: 2).fill(.green).frame(width: 10, height: 10) }
                Label { Text("Still to enter") } icon: { RoundedRectangle(cornerRadius: 2).fill(Color.accentColor).frame(width: 10, height: 10) }
                Label { Text("Needs a ticket") } icon: { RoundedRectangle(cornerRadius: 2).fill(.orange).frame(width: 10, height: 10) }
            }
            .font(.caption).foregroundStyle(.secondary)
        }
        .padding(14)
        .glassEffect(.regular, in: .rect(cornerRadius: 18))
    }
}

#Preview {
    ReportsView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
