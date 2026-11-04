import SwiftUI

enum Pane: String, CaseIterable, Identifiable {
    case repositories = "Repositories"
    case calendar = "Meetings"
    case sessions = "Work sessions"
    case periods = "Periods"
    case timesheet = "Timesheet"
    case review = "Review"
    case reports = "Totals"
    case export = "Export"

    var id: String { rawValue }
    var icon: String {
        switch self {
        case .repositories: return "externaldrive.connected.to.line.below"
        case .calendar: return "person.2"
        case .sessions: return "clock.badge.checkmark"
        case .periods: return "flag"
        case .timesheet: return "calendar"
        case .review: return "checklist"
        case .reports: return "chart.bar.xaxis"
        case .export: return "square.and.arrow.up"
        }
    }
}

/// Sidebar groups in the order a timesheet is filled: set up once, import the month, fill it, then export.
enum Step: String, CaseIterable, Identifiable {
    case setup = "1 · Set up"
    case gather = "2 · Bring in the month"
    case fill = "3 · Fill the timesheet"
    case finish = "4 · Check & export"

    var id: String { rawValue }
    var panes: [Pane] {
        switch self {
        case .setup: return [.repositories, .calendar]
        case .gather: return [.sessions, .periods]
        case .fill: return [.timesheet, .review]
        case .finish: return [.reports, .export]
        }
    }
}

struct ContentView: View {
    @Environment(AppState.self) private var state
    @State private var pane: Pane? = .timesheet
    @State private var showExport = false

    var body: some View {
        NavigationSplitView {
            List(selection: $pane) {
                ForEach(Step.allCases) { step in
                    Section(step.rawValue) {
                        ForEach(step.panes) { p in
                            Label(p.rawValue, systemImage: p.icon).tag(p)
                        }
                    }
                }
            }
            .navigationSplitViewColumnWidth(min: 190, ideal: 210)
        } detail: {
            Group {
                switch pane ?? .timesheet {
                case .repositories: RepositoriesView()
                case .calendar: CalendarView()
                case .sessions: SessionsView()
                case .periods: PeriodsView()
                case .timesheet: TimesheetView()
                case .review: ReviewView()
                case .reports: ReportsView()
                case .export: ExportView().frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top).padding(.top, 24)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .safeAreaInset(edge: .bottom, spacing: 0) { StatusBar() }
        }
        .toolbar {
            ToolbarItem(placement: .navigation) { RangeBar() }
            ToolbarItemGroup(placement: .primaryAction) {
                Button { state.rebuild() } label: { Label("Rebuild drafts", systemImage: "arrow.triangle.2.circlepath") }
                    .help("Recalculate unlocked drafts from stored commits, meetings, sessions and periods")
                Button { Task { await state.collect() } } label: { Label("Collect", systemImage: "arrow.down.circle") }
                    .disabled(state.isBusy)
                    .help("Read git history and calendar meetings for the range, then build drafts")
            }
            ToolbarSpacer(.fixed, placement: .primaryAction)
            ToolbarItemGroup(placement: .primaryAction) {
                Button { showExport.toggle() } label: { Label("Export", systemImage: "square.and.arrow.up") }
                    .help("Export to Tempo, Markdown for Claude, CSV or JSON")
                    .popover(isPresented: $showExport, arrowEdge: .bottom) { ExportView() }
                SettingsLink { Label("Settings", systemImage: "gearshape") }
                    .help("Settings (⌘,)")
            }
        }
    }
}

/// Window-wide status bar along the bottom edge.
struct StatusBar: View {
    @Environment(AppState.self) private var state
    var body: some View {
        VStack(spacing: 0) {
            Divider()
            HStack(spacing: 8) {
                if state.isBusy { ProgressView().controlSize(.small) }
                Text(state.status).lineLimit(1).truncationMode(.tail).textSelection(.enabled)
                    .help(state.status)
                Spacer()
                if !state.dirty.isEmpty {
                    Text("\(state.dirty.count) unsaved").foregroundStyle(.orange)
                    Button("Save") { state.saveDirty() }
                        .controlSize(.small)
                }
            }
            .font(.callout)
            .foregroundStyle(.secondary)
            .padding(.horizontal, 12)
            .frame(height: 28)
        }
        .background(.bar)
    }
}

/// Section title used above content areas.
struct PaneHeader<Trailing: View>: View {
    let title: String
    var subtitle: String = ""
    @ViewBuilder var trailing: Trailing

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 12) {
            Text(title).font(.title2.weight(.semibold))
            if !subtitle.isEmpty { Text(subtitle).foregroundStyle(.secondary) }
            Spacer()
            trailing
        }
        .padding(.horizontal, 20)
        .padding(.top, 16)
        .padding(.bottom, 8)
    }
}

/// Date range shared by every pane.
struct RangeBar: View {
    @Environment(AppState.self) private var state

    var body: some View {
        @Bindable var state = state
        HStack(spacing: 6) {
            DatePicker("From", selection: $state.from, displayedComponents: .date).labelsHidden()
            Image(systemName: "arrow.right").foregroundStyle(.secondary).imageScale(.small)
            DatePicker("To", selection: $state.to, displayedComponents: .date).labelsHidden()
            Menu {
                ForEach(QuickRange.allCases, id: \.self) { r in
                    Button(r.rawValue) { state.setRange(r) }
                }
            } label: { Image(systemName: "calendar.badge.clock") }
            .menuIndicator(.hidden)
            .help("Quick ranges")
        }
        .padding(.horizontal, 6)
        .fixedSize()
        .onChange(of: state.from) { state.reloadRange() }
        .onChange(of: state.to) { state.reloadRange() }
    }
}

/// Comma-separated list editor that commits on Return or when focus leaves the field.
struct ListField: View {
    let title: String
    @Binding var values: [String]
    @State private var text = ""
    @FocusState private var focused: Bool

    var body: some View {
        TextField(title, text: $text)
            .focused($focused)
            .onAppear { text = values.joined(separator: ", ") }
            .onSubmit(commit)
            .onChange(of: focused) { if !focused { commit() } }
    }

    private func commit() {
        values = text.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        text = values.joined(separator: ", ")
    }
}

/// Stores a secret in the Keychain instead of the database.
struct SecretField: View {
    let title: String
    let account: String
    @State private var value = ""

    var body: some View {
        SecureField(title, text: $value)
            .onAppear { value = Keychain.get(account) ?? "" }
            .onChange(of: value) { Keychain.set(value, for: account) }
    }
}

#Preview {
    ContentView()
        .environment(AppState.preview)
        .frame(width: 1080, height: 680)
}
