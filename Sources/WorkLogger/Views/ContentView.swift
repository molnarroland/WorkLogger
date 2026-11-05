import SwiftUI

enum Pane: String, CaseIterable, Identifiable {
    case repositories = "Repositories"
    case calendar = "Meetings"
    case sessions = "Work sessions"
    case periods = "Periods"
    case chats = "Chats"
    case timesheet = "Timesheet"
    case review = "Review"
    case reports = "Totals"
    case export = "Export"

    var id: String { rawValue }
    var module: Module? {
        switch self {
        case .repositories: return .git
        case .calendar: return .calendar
        case .sessions: return .workTimes
        default: return nil
        }
    }
    var icon: String {
        switch self {
        case .repositories: return "externaldrive.connected.to.line.below"
        case .calendar: return "person.2"
        case .sessions: return "clock.badge.checkmark"
        case .periods: return "flag"
        case .chats: return "bubble.left.and.bubble.right"
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
    func panes(_ modules: Set<Module>) -> [Pane] { allPanes.filter { $0.module.map(modules.contains) ?? true } }

    private var allPanes: [Pane] {
        switch self {
        case .setup: return [.repositories, .calendar]
        case .gather: return [.sessions, .periods, .chats]
        case .fill: return [.timesheet, .review]
        case .finish: return [.reports, .export]
        }
    }
}

struct ContentView: View {
    @Environment(AppState.self) private var state
    @State private var pane: Pane? = .timesheet
    @State private var showExport = false
    @State private var showWelcome = false
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        NavigationSplitView {
            List(selection: $pane) {
                ForEach(Step.allCases) { step in
                    let panes = step.panes(state.settings.modules)
                    if !panes.isEmpty { Section(step.rawValue) {
                        ForEach(panes) { p in
                            Label(p.rawValue, systemImage: p.icon).tag(p)
                        }
                    } }
                }
            }
            // The sidebar gets no toolbar inset and opens scrolled under the traffic lights, so reserve the
            // space outside the scroll view and pin the list to its top.
            // ponytail: fixed 28pt, tune if the header still overlaps.
            .safeAreaInset(edge: .top, spacing: 0) { Color.clear.frame(height: 28) }
            .defaultScrollAnchor(.top)
            .navigationSplitViewColumnWidth(min: 190, ideal: 210)
        } detail: {
            Group {
                switch pane ?? .timesheet {
                case .repositories: RepositoriesView()
                case .calendar: CalendarView()
                case .sessions: SessionsView()
                case .periods: PeriodsView()
                case .chats: ChatsView()
                case .timesheet: TimesheetView()
                case .review: ReviewView()
                case .reports: ReportsView()
                case .export: ExportView().frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top).padding(.top, 24)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .safeAreaInset(edge: .bottom, spacing: 0) { StatusBar() }
        }
        .sheet(isPresented: $showWelcome) { WelcomeView() }
        // A background fetch found a source signed out: show its sign-in window and bounce the Dock icon.
        .onChange(of: state.signInNeeded) { _, source in
            guard let source else { return }
            openWindow(id: source.windowID)
            NSApp.requestUserAttention(.criticalRequest)
        }
        .onAppear {
            showWelcome = !state.settings.welcomed
            #if DEBUG
            if WebDebugProbe.isActive { showWelcome = false; openWindow(id: WebSession.Source.hrmaster.windowID) }
            if WebDebugProbe.fetch != nil { showWelcome = false; Task { await WebDebugProbe.runFetch(state) } }
            #endif
        }
        .onChange(of: state.settings.modules) {
            if let m = pane?.module, !state.settings.modules.contains(m) { pane = .timesheet }
        }
        .toolbar(removing: .title)   // the title is drawn centered as the principal item instead
        .toolbar {
            // One item laid out by hand: separate .navigation items ignore spacers and share a pill.
            ToolbarItem(placement: .navigation) {
                HStack(spacing: 10) {
                    RangeField(isTo: false).toolbarPill()
                    RangeField(isTo: true).toolbarPill()
                    RangePresets().toolbarPill()
                }
            }
            .sharedBackgroundVisibility(.hidden)
            ToolbarItem(placement: .principal) { Text("WorkLogger").font(.headline) }
                .sharedBackgroundVisibility(.hidden)
            ToolbarItemGroup(placement: .primaryAction) {
                Button { state.rebuild() } label: { Label("Rebuild drafts", systemImage: "arrow.triangle.2.circlepath") }
                    .help("Recalculate unlocked drafts from stored commits, meetings, sessions and periods")
                Button { Task { await state.collect() } } label: { Label("Collect", systemImage: "arrow.down.circle") }
                    .disabled(state.isBusy)
                    .help("Read git history and calendar meetings for the range, then build drafts")
            }
            ToolbarSpacer(.fixed, placement: .primaryAction)
            ToolbarItemGroup(placement: .primaryAction) {
                Button { showWelcome = true } label: { Label("Modules", systemImage: "square.grid.2x2") }
                    .help("Choose which data sources WorkLogger uses")
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

/// From or To date field with a calendar dropdown (no stepper arrows); each sits in its own toolbar pill.
struct RangeField: View {
    @Environment(AppState.self) private var state
    let isTo: Bool

    var body: some View {
        @Bindable var state = state
        Group {
            if isTo {
                DatePicker("To", selection: $state.to, in: state.from..., displayedComponents: .date)
            } else {
                DatePicker("From", selection: $state.from, displayedComponents: .date)
            }
        }
        .help(isTo ? "To" : "From")
        .datePickerStyle(.field)   // no stepper: it can't fit a toolbar pill
        .labelsHidden()
        .fixedSize()
        .onChange(of: isTo ? state.to : state.from) { state.reloadRange() }
    }
}

extension View {
    /// Standalone glass capsule for a toolbar item whose shared background is hidden.
    func toolbarPill() -> some View {
        padding(.horizontal, 12).frame(height: 32).glassEffect(.regular.interactive(), in: .capsule)
    }
}

/// Preset ranges (today, this week, last month…), kept apart from the date fields.
struct RangePresets: View {
    @Environment(AppState.self) private var state

    var body: some View {
        Menu {
            ForEach(QuickRange.allCases, id: \.self) { r in
                Button(r.rawValue) { state.setRange(r) }
            }
        } label: { Label("Quick ranges", systemImage: "calendar.badge.clock") }
        .menuIndicator(.hidden)
        .buttonStyle(.borderless)   // the pill supplies the glass
        .help("Quick ranges")
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
    @Environment(AppState.self) private var state
    let title: String
    let account: String
    @State private var value = ""

    var body: some View {
        SecureField(title, text: $value)
            .onAppear { value = state.services.secrets.get(account) ?? "" }
            .onChange(of: value) { state.services.secrets.set(value, for: account) }
    }
}

#Preview {
    ContentView()
        .environment(AppState.preview)
        .frame(width: 1080, height: 680)
}
