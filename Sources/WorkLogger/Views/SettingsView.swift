import SwiftUI
import AppKit

struct SettingsView: View {
    @Environment(AppState.self) private var state

    var body: some View {
        @Bindable var state = state
        Form {
            Section {
                Picker("Daily total", selection: $state.settings.roundingMode) {
                    ForEach(RoundingMode.choices) { Text($0.label).tag($0) }
                }
                if state.settings.roundingMode != .estimate {
                    Picker("Round worked time up to", selection: $state.settings.dayRoundUpMinutes) {
                        Text("15 minutes").tag(15)
                        Text("30 minutes").tag(30)
                        Text("Whole hours").tag(60)
                    }
                    Stepper(value: $state.settings.fixedDailyHours, in: 1...12, step: 0.5) {
                        LabeledContent("Hours per day without imported sessions",
                                       value: String(format: "%.1f h", state.settings.fixedDailyHours))
                    }
                }
                Picker("Round to", selection: $state.settings.granularityMinutes) {
                    ForEach([5, 10, 15, 30, 60], id: \.self) { Text("\($0) minutes").tag($0) }
                }
                Toggle("Fill rest days without imported sessions too", isOn: $state.settings.includeWeekends)
                HStack {
                    TextField("Workday start", text: $state.settings.defaultDayStart).frame(width: 160)
                    TextField("Workday end", text: $state.settings.defaultDayEnd).frame(width: 160)
                }
            } header: {
                Text("Daily totals")
            } footer: {
                Text("A day adds up to the time you worked according to the imported work sessions (HRmaster's worked hours), rounded up: with 15 minutes, 7h 22m becomes 7h 30m and 12h stays 12h. Days without imported sessions use the hours per day. Meetings keep their real length; commit entries fill the rest.")
            }

            Section {
                Stepper("Shortest commit: \(state.settings.minCommitMinutes) min", value: $state.settings.minCommitMinutes, in: 5...60, step: 5)
                Stepper("Longest commit: \(state.settings.maxCommitMinutes) min", value: $state.settings.maxCommitMinutes, in: 30...480, step: 15)
                Stepper("Ignore gaps longer than: \(state.settings.maxCommitGapMinutes) min", value: $state.settings.maxCommitGapMinutes, in: 30...480, step: 15)
                ListField(title: "Paths left out of the size estimate", values: $state.settings.ignoredPathPatterns)
            } header: {
                Text("Commit estimates")
            } footer: {
                Text("An estimate mixes the free working time since your previous commit with the size of the change (lines, files) and its kind (refactor, fix, typo…).")
            }

            AbsenceSettings()

            SignedInSites()

            Section {
                TextField("Jira site", text: $state.settings.jiraBaseURL, prompt: Text("https://yourcompany.atlassian.net"))
                TextField("Jira email", text: $state.settings.jiraEmail)
                SecretField(title: "Jira API token", account: SecretKey.jiraToken)
                SecretField(title: "Tempo API token", account: SecretKey.tempoToken)
                TextField("Tempo API base URL", text: $state.settings.tempoBaseURL)
                TextField("Jira account ID (filled in automatically)", text: $state.settings.jiraAccountId)
            } header: {
                Text("Tempo API (optional)")
            } footer: {
                Text("Only needed for Export → Send to Tempo. Log in Tempo by ticket works without any key. Tokens are stored in your Keychain; create the Jira token at id.atlassian.com and the Tempo token in Tempo → Settings → API Integration.")
            }

            Section("Storage") {
                LabeledContent("Database", value: state.databasePath)
                Button("Show in Finder") {
                    NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: state.databasePath)])
                }
            }
        }
        .formStyle(.grouped)
    }
}

/// Google Chat and HRmaster, signed in inside WorkLogger: state, open the window, sign out.
struct SignedInSites: View {
    @Environment(AppState.self) private var state
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Section {
            ForEach(WebSession.Source.allCases) { source in
                let session = WebSession.of(source)
                HStack {
                    Text(source.title)
                    if source == .hrmaster, let host = HRmasterCollector.normalize(state.settings.hrmasterURL)?.host() {
                        Text(host).foregroundStyle(.secondary).font(.callout)
                    }
                    Spacer()
                    switch session.signedIn {
                    case true?: Label("Signed in", systemImage: "checkmark.circle.fill").foregroundStyle(.green)
                    case false?: Label("Signed out", systemImage: "xmark.circle").foregroundStyle(.orange)
                    case nil: EmptyView()
                    }
                    Button(session.signedIn == true ? "Open…" : "Sign in…") { openWindow(id: source.windowID) }
                        .disabled(source == .hrmaster && HRmasterCollector.normalize(state.settings.hrmasterURL) == nil)
                    Button("Sign out") { Task { await session.signOut() } }
                        .disabled(session.signedIn != true)
                }
                .task { await session.checkSignedIn(host: HRmasterCollector.normalize(state.settings.hrmasterURL)?.host()) }
            }
        } header: {
            Text("Signed-in sites")
        } footer: {
            Text("Each site keeps its own cookies inside WorkLogger, separate from Safari; Sign out deletes them. HRmaster ends its session after about 25 idle minutes, so it often shows signed out; a fetch then asks you to sign in.")
        }
    }
}

/// Leave and public holidays imported from the HR system can be logged to Tempo absence issues.
struct AbsenceSettings: View {
    @Environment(AppState.self) private var state
    @State private var contains = ""
    @State private var issue = ""

    var body: some View {
        @Bindable var state = state
        Section {
            TextField("Absence issue, e.g. HR-1 (empty: don't log leave)", text: $state.settings.absenceIssueKey)
            ForEach(state.settings.absenceRules) { rule in
                HStack {
                    Text("“\(rule.contains)”")
                    Image(systemName: "arrow.right").foregroundStyle(.secondary)
                    Text(rule.issueKey).bold()
                    Spacer()
                    Button(role: .destructive) {
                        state.settings.absenceRules.removeAll { $0.id == rule.id }
                    } label: { Image(systemName: "trash") }
                    .buttonStyle(.borderless)
                }
            }
            HStack {
                TextField("Day label contains, e.g. Betegszabadság", text: $contains)
                TextField("Issue", text: $issue).frame(width: 110)
                Button("Add") {
                    state.settings.absenceRules.append(MeetingRule(contains: contains, issueKey: issue.uppercased()))
                    contains = ""
                    issue = ""
                }
                .disabled(contains.isEmpty || issue.isEmpty)
            }
        } header: {
            Text("Absences")
        } footer: {
            Text("Leave days (Szabadság and other leave codes) are logged with the hours HRmaster reports; public holidays only when it reports hours. Rest days, holidays and leave never get filled with commit time.")
        }
    }
}

#Preview {
    SettingsView()
        .environment(AppState.preview)
        .frame(width: 900, height: 700)
}
