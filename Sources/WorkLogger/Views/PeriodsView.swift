import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

/// Project periods: a stretch of days booked to one issue (e.g. a migration that runs for three weeks).
struct PeriodsView: View {
    @Environment(AppState.self) private var state

    static let palette: [Color] = [.blue, .purple, .teal, .pink, .indigo, .mint, .orange]

    static func color(for period: ProjectPeriod, in all: [ProjectPeriod]) -> Color {
        palette[(all.firstIndex(of: period) ?? 0) % palette.count]
    }

    @State private var wide = true

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 0) {
            PaneHeader(title: "Periods", subtitle: "Book stretches of days to a larger project") {
                Button {
                    state.settings.periods.append(ProjectPeriod(title: "", issueKey: "", from: state.fromDay, to: state.toDay))
                } label: {
                    Label("Add period", systemImage: "plus")
                }
                .buttonStyle(.glass)
                Button("Rebuild drafts") { state.rebuild() }
                    .buttonStyle(.glassProminent)
            }

            if state.settings.periods.isEmpty {
                ContentUnavailableView {
                    Label("No periods yet", systemImage: "flag")
                } description: {
                    Text("Add one here, or shift-click two days on the Timesheet to select a stretch and mark it.")
                }
            } else {
                // Side by side when there's room; on a half-screen window the month goes under the list.
                let layout = wide ? AnyLayout(HStackLayout(alignment: .top, spacing: 0)) : AnyLayout(VStackLayout(spacing: 0))
                layout {
                    Form {
                        Section {
                            ForEach($state.settings.periods) { $period in
                                PeriodRow(period: $period, color: Self.color(for: period, in: state.settings.periods)) {
                                    state.settings.periods.removeAll { $0.id == period.id }
                                }
                                .listRowBackground(Self.color(for: period, in: state.settings.periods).opacity(0.10))
                            }
                        } footer: {
                            Text(
                                "On every workday inside a period, commit time goes to the period's issue and the rest of the day is filled with it. Meetings keep their own issues; leave days are never filled. Later periods win where they overlap. Rebuild drafts after changing periods."
                            )
                        }
                    }
                    .formStyle(.grouped)
                    PeriodMonth(periods: state.settings.periods)
                        .frame(width: wide ? 300 : nil)
                        .padding(16)
                }
            }
        }
        .onGeometryChange(for: Bool.self) {
            $0.size.width >= 760
        } action: {
            wide = $0
        }
    }
}

/// The selected month with each period's days in its color, the same colors as on the Timesheet cards.
private struct PeriodMonth: View {
    @Environment(AppState.self) private var state
    let periods: [ProjectPeriod]

    var body: some View {
        let cal = DayKey.calendar
        let first = DayKey.startOfDay(state.rangeStart)
        let lead = (cal.component(.weekday, from: first) - cal.firstWeekday + 7) % 7
        let days = DayKey.days(from: first, to: cal.date(byAdding: DateComponents(month: 1, day: -1), to: first)!)
        VStack(alignment: .leading, spacing: 8) {
            Text(first.formatted(.dateTime.month(.wide).year())).font(.headline)
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 4), count: 7), spacing: 4) {
                ForEach(0..<lead, id: \.self) { _ in Color.clear.frame(height: 30) }
                ForEach(days, id: \.self) { day in
                    let p = periods.last { $0.contains(day) }
                    Text(DayKey.date(day).formatted(.dateTime.day()))
                        .font(.callout).monospacedDigit()
                        .frame(maxWidth: .infinity, minHeight: 30)
                        .background(
                            (p.map { PeriodsView.color(for: $0, in: periods) } ?? .clear).opacity(0.3),
                            in: .rect(cornerRadius: 7)
                        )
                        .overlay(RoundedRectangle(cornerRadius: 7).strokeBorder(.quaternary))
                        .help(p.map { "\($0.title.isEmpty ? $0.issueKey : $0.title) · \($0.issueKey)" } ?? "")
                }
            }
        }
        .padding(14)
        .glassEffect(.regular, in: .rect(cornerRadius: 18))
    }
}

private struct PeriodRow: View {
    @Binding var period: ProjectPeriod
    let color: Color
    let remove: () -> Void

    private func day(_ key: WritableKeyPath<ProjectPeriod, String>) -> Binding<Date> {
        Binding(get: { DayKey.date(period[keyPath: key]) }, set: { period[keyPath: key] = DayKey.key($0) })
    }

    var body: some View {
        HStack(spacing: 10) {
            Circle().fill(color).frame(width: 10, height: 10)
            TextField("Project name", text: $period.title)
            TextField("Issue", text: Binding(get: { period.issueKey }, set: { period.issueKey = $0.uppercased() }))
                .frame(width: 110)
                .foregroundStyle(JiraKey.normalize(period.issueKey) == nil ? Color.orange : Color.primary)
            DatePicker("From", selection: day(\.from), displayedComponents: .date).labelsHidden()
            Image(systemName: "arrow.right").foregroundStyle(.secondary).imageScale(.small)
            DatePicker("To", selection: day(\.to), in: DayKey.date(period.from)..., displayedComponents: .date).labelsHidden()
            Button(role: .destructive, action: remove) { Image(systemName: "trash") }
                .buttonStyle(.borderless)
        }
        .textFieldStyle(.roundedBorder)
    }
}

#Preview {
    let state = AppState.preview
    state.settings.periods = [
        ProjectPeriod(title: "Billing migration", issueKey: "TAO-200", from: state.fromDay, to: state.toDay)
    ]
    return PeriodsView()
        .environment(state)
        .frame(width: 900, height: 500)
}
