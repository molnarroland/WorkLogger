import SwiftUI

/// Project periods: a stretch of days booked to one issue (e.g. a migration that runs for three weeks).
struct PeriodsView: View {
    @Environment(AppState.self) private var state

    static let palette: [Color] = [.blue, .purple, .teal, .pink, .indigo, .mint, .orange]

    static func color(for period: ProjectPeriod, in all: [ProjectPeriod]) -> Color {
        palette[(all.firstIndex(of: period) ?? 0) % palette.count]
    }

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 0) {
            PaneHeader(title: "Periods", subtitle: "Book stretches of days to a larger project") {
                Button {
                    state.settings.periods.append(ProjectPeriod(title: "", issueKey: "", from: state.fromDay, to: state.toDay))
                } label: { Label("Add period", systemImage: "plus") }
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
                Form {
                    Section {
                        ForEach($state.settings.periods) { $period in
                            PeriodRow(period: $period, color: Self.color(for: period, in: state.settings.periods)) {
                                state.settings.periods.removeAll { $0.id == period.id }
                            }
                        }
                    } footer: {
                        Text("On every workday inside a period, commit time goes to the period's issue and the rest of the day is filled with it. Meetings keep their own issues; leave days are never filled. Later periods win where they overlap. Rebuild drafts after changing periods.")
                    }
                }
                .formStyle(.grouped)
            }
        }
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
    state.settings.periods = [ProjectPeriod(title: "Billing migration", issueKey: "TAO-200", from: state.fromDay, to: state.toDay)]
    return PeriodsView()
        .environment(state)
        .frame(width: 900, height: 500)
}
