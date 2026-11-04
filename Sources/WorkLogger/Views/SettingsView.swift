import SwiftUI
import AppKit

struct SettingsView: View {
    @Environment(AppState.self) private var state

    var body: some View {
        @Bindable var state = state
        Form {
            Section {
                Picker("Daily total", selection: $state.settings.roundingMode) {
                    ForEach(RoundingMode.allCases) { Text($0.label).tag($0) }
                }
                if state.settings.roundingMode != .estimate {
                    Stepper(value: $state.settings.fixedDailyHours, in: 1...12, step: 0.5) {
                        LabeledContent("Hours per day", value: String(format: "%.1f h", state.settings.fixedDailyHours))
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
                Text("Meetings keep their real length; commit entries are scaled so each day adds up. The workday is used when no work sessions were imported. With “imported work sessions”, days without sessions fall back to the hours per day.")
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

            Section {
                ListField(title: "Break keywords", values: $state.settings.breakKeywords)
                Toggle("Short dates like 11.03. are month.day", isOn: $state.settings.shortDateMonthFirst)
            } header: {
                Text("Work-time logger import")
            } footer: {
                Text("HRmaster pages are recognised automatically; these options apply to other reports.")
            }

            AbsenceSettings()

            Section {
                TextField("Jira site", text: $state.settings.jiraBaseURL, prompt: Text("https://yourcompany.atlassian.net"))
                TextField("Jira email", text: $state.settings.jiraEmail)
                SecretField(title: "Jira API token", account: SecretKey.jiraToken)
                SecretField(title: "Tempo API token", account: SecretKey.tempoToken)
                TextField("Tempo API base URL", text: $state.settings.tempoBaseURL)
                TextField("Jira account ID (filled in automatically)", text: $state.settings.jiraAccountId)
            } header: {
                Text("Jira & Tempo")
            } footer: {
                Text("Tokens are stored in your Keychain. Create the Jira token at id.atlassian.com and the Tempo token in Tempo → Settings → API Integration.")
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
