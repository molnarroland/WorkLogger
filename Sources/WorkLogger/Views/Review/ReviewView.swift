import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

struct ReviewView: View {
    @Environment(AppState.self) private var state
    /// Only the selected entry gets the editable row; the rest are plain text, which keeps scrolling smooth.
    @State private var selected: Int64?
    @State private var chatDay: String?
    @State private var filter: ReviewFilter = .all
    @State private var checked: Set<Int64> = []
    @State private var bulkIssue = ""

    enum ReviewFilter: CaseIterable {
        case noTicket, drafts, locked, all
        func title(missing: Int) -> LocalizedStringKey {
            switch self {
            case .noTicket: "No ticket · \(missing)"
            case .drafts: "Drafts"
            case .locked: "Locked"
            case .all: "All"
            }
        }
    }

    private func shown(_ w: Worklog) -> Bool {
        switch filter {
        case .all: return true
        case .noTicket: return w.needsIssue
        case .drafts: return w.status == .draft
        case .locked: return w.locked
        }
    }

    var body: some View {
        // Group once per render: each section then only builds its own rows (was days × all entries).
        let missingDays = state.onlyMissingDays ? state.missingDays : nil
        let byDay = Dictionary(
            grouping: state.worklogs.indices.filter { i in
                shown(state.worklogs[i]) && (missingDays?.contains(state.worklogs[i].day) ?? true)
            }, by: { state.worklogs[$0].day })
        let builder = state.makeBuilder()
        let missing = state.worklogs.missingIssue.count
        VStack(spacing: 0) {
            PaneHeader(title: "Review", subtitle: "\(state.worklogs.count) entries") {
                // Start with what blocks Tempo: entries without a ticket.
                HStack(spacing: 6) {
                    ForEach(ReviewFilter.allCases, id: \.self) { f in
                        Button(f.title(missing: missing)) { filter = f }
                            .buttonStyle(.glass)
                            .tint(filter == f ? .accentColor : nil)
                            .foregroundStyle(f == .noTicket && missing > 0 ? Color.orange : Color.primary)
                            .overlay { if filter == f { Capsule().strokeBorder(Color.accentColor, lineWidth: 1.5) } }
                    }
                }
                Button("Save changes") { state.saveDirty() }
                    .buttonStyle(.glassProminent)
                    .disabled(state.dirty.isEmpty)
            }
            if !checked.isEmpty {
                HStack(spacing: 10) {
                    Text("\(checked.count) selected").font(.headline)
                    TextField("Ticket, e.g. TAO-12", text: $bulkIssue).textFieldStyle(.roundedBorder).frame(width: 150)
                        .onSubmit(applyIssue)
                    Button("Set ticket", action: applyIssue).disabled(JiraKey.normalize(bulkIssue) == nil)
                    Button("Mark reviewed") {
                        state.setStatus(.reviewed, for: checked)
                        checked = []
                    }
                    Spacer()
                    Button("Clear selection") { checked = [] }
                }
                .padding(.horizontal, 12).padding(.vertical, 8)
                .glassEffect(.regular.tint(.orange.opacity(0.12)), in: .rect(cornerRadius: 14))
                .padding(.horizontal, 16).padding(.bottom, 8)
            }

            if state.worklogs.isEmpty {
                ContentUnavailableView {
                    Label("No worklogs in this range", systemImage: "clock")
                } description: {
                    Text("Pick a range and choose Collect to read your commits and meetings.")
                }
            } else {
                List(selection: $selected) {
                    ForEach(byDay.keys.sorted(), id: \.self) { day in
                        let rows = byDay[day]!.map { (index: $0, log: state.worklogs[$0]) }
                        Section {
                            ChatDaySummary(day: day) { chatDay = $0 }
                            ForEach(rows, id: \.log.id) { row in
                                HStack(spacing: 8) {
                                    Toggle(
                                        "Select",
                                        isOn: Binding(
                                            get: { checked.contains(row.log.id) },
                                            set: { if $0 { checked.insert(row.log.id) } else { checked.remove(row.log.id) } })
                                    )
                                    .labelsHidden().toggleStyle(.checkbox)
                                    if selected == row.log.id {
                                        WorklogRow(log: binding(row.index, row.log))
                                    } else {
                                        WorklogLine(log: row.log)
                                    }
                                }
                                .tag(row.log.id)
                                .contextMenu {
                                    Button(row.log.locked ? "Unlock" : "Lock") {
                                        binding(row.index, row.log).wrappedValue.locked.toggle()
                                        state.markDirty(row.log.id)
                                    }
                                    Button("Delete", role: .destructive) { state.delete(row.log) }
                                }
                            }
                        } header: {
                            DayHeader(day: day, logs: rows.map(\.log), target: state.target(day: day, builder: builder))
                        }
                    }
                }
                .listStyle(.inset)
            }
        }
        .chatInspector(day: $chatDay)
        .onAppear { if missing0 > 0 { filter = .noTicket } }
    }

    private var missing0: Int { state.worklogs.missingIssue.count }

    private func applyIssue() {
        state.setIssue(bulkIssue, for: checked)
        bulkIssue = ""
        checked = []
    }

    /// Binding to one entry by index, guarded by id so a row that is being removed never reads a shifted index.
    private func binding(_ i: Int, _ snapshot: Worklog) -> Binding<Worklog> {
        func valid() -> Bool { state.worklogs.indices.contains(i) && state.worklogs[i].id == snapshot.id }
        return Binding(
            get: { valid() ? state.worklogs[i] : snapshot },
            set: { if valid() { state.worklogs[i] = $0 } })
    }
}

#Preview {
    ReviewView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}

#Preview("Empty") {
    ReviewView()
        .environment(AppState.empty)
        .frame(width: 900, height: 600)
}
