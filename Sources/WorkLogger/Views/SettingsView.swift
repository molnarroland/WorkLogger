import AppKit
import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

struct SettingsView: View {
    @Environment(AppState.self) private var state

    var body: some View {
        // Icon tabs along the top, one smaller page each (the standard macOS settings layout).
        TabView {
            Tab("General", systemImage: "gearshape") {
                page {
                    general
                    storage
                }
            }
            Tab("Timesheet", systemImage: "calendar") {
                page {
                    dailyTotals
                    AbsenceSettings()
                }
            }
            Tab("Coding", systemImage: "chevron.left.forwardslash.chevron.right") { page { commitEstimates } }
            Tab("Chats", systemImage: "bubble.left.and.bubble.right") { page { chats } }
            Tab("Sites", systemImage: "person.crop.circle.badge.checkmark") { page { SignedInSites() } }
            Tab("Jira", systemImage: "checklist.checked") { page { JiraAccountsSettings() } }
        }
    }

    private func page<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
        Form { content() }.formStyle(.grouped)
    }

    /// Menu bar item.
    @ViewBuilder private var general: some View {
        @Bindable var state = state
        Section("General") {
            Toggle("Show today's time in the menu bar", isOn: $state.settings.showMenuBarItem)
        }
    }

    /// Where the database lives.
    @ViewBuilder private var storage: some View {
        @Bindable var state = state
        Section("Storage") {
            LabeledContent("Database", value: state.databasePath)
            Button("Show in Finder") {
                NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: state.databasePath)])
            }
        }
    }

    /// How a day adds up.
    @ViewBuilder private var dailyTotals: some View {
        @Bindable var state = state
        Section {
            Picker("Daily total", selection: $state.settings.roundingMode) {
                ForEach(RoundingMode.choices) { Text($0.localizedLabel).tag($0) }
            }
            if state.settings.roundingMode != .estimate {
                Picker("Round worked time up to", selection: $state.settings.dayRoundUpMinutes) {
                    Text("15 minutes").tag(15)
                    Text("30 minutes").tag(30)
                    Text("Whole hours").tag(60)
                }
                Stepper(value: $state.settings.fixedDailyHours, in: 1...12, step: 0.5) {
                    LabeledContent(
                        "Hours per day without imported sessions",
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
            Text(
                "A day adds up to the time you worked according to the imported work sessions (HRmaster's worked hours), rounded up: with 15 minutes, 7h 22m becomes 7h 30m and 12h stays 12h. Days without imported sessions use the hours per day. Meetings keep their real length; commit entries fill the rest."
            )
        }
    }

    /// How long a commit counts.
    @ViewBuilder private var commitEstimates: some View {
        @Bindable var state = state
        Section {
            Stepper(
                "Shortest commit: \(state.settings.minCommitMinutes) min", value: $state.settings.minCommitMinutes,
                in: 5...60, step: 5)
            Stepper(
                "Longest commit: \(state.settings.maxCommitMinutes) min", value: $state.settings.maxCommitMinutes,
                in: 30...480, step: 15)
            Stepper(
                "Ignore gaps longer than: \(state.settings.maxCommitGapMinutes) min",
                value: $state.settings.maxCommitGapMinutes, in: 30...480, step: 15)
            ListField(title: "Paths left out of the size estimate", values: $state.settings.ignoredPathPatterns)
        } header: {
            Text("Commit estimates")
        } footer: {
            Text(
                "An estimate mixes the free working time since your previous commit with the size of the change (lines, files) and its kind (refactor, fix, typo…)."
            )
        }
    }

    /// Chat stretches and the conversations always logged.
    @ViewBuilder private var chats: some View {
        @Bindable var state = state
        Section {
            Stepper(value: $state.settings.chatGapMinutes, in: 1...60) {
                LabeledContent(
                    "New stretch after a pause of", value: String(localized: "\(state.settings.chatGapMinutes) min"))
            }
            ForEach(state.settings.chatRules) { rule in
                HStack {
                    Text(rule.name)
                    Image(systemName: "arrow.right").foregroundStyle(.secondary)
                    Text(rule.issueKey).bold()
                    Spacer()
                    Button(role: .destructive) {
                        state.settings.chatRules.removeAll { $0.id == rule.id }
                    } label: {
                        Image(systemName: "trash")
                    }
                    .buttonStyle(.borderless)
                }
            }
        } header: {
            Text("Chats")
        } footer: {
            Text(
                "Messages closer together than the pause count as one stretch of chatting; each stretch is at least a minute. Conversations listed here (ticked “Always” on the Chats page) are logged to their ticket on every day they have messages."
            )
        }
    }
}

/// Jira accounts, one per site: sign-in, where the worklogs live (Tempo, Jira or another tool), the projects that
/// decide which account an issue goes to, and the default for everything else.
struct JiraAccountsSettings: View {
    @Environment(AppState.self) private var state
    @State private var selected: JiraAccount.ID?

    var body: some View {
        @Bindable var state = state
        let accounts = state.settings.jiraAccounts
        let defaultId = state.settings.defaultJiraAccount?.id
        Section {
            ForEach(accounts) { account in
                HStack {
                    Image(systemName: account.id == current?.id ? "largecircle.fill.circle" : "circle")
                        .foregroundStyle(.tint)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(account.title.isEmpty ? String(localized: "New account") : account.title)
                        (Text(account.provider.title)
                            + Text(account.projectKeys.isEmpty ? "" : " · " + account.projectKeys.joined(separator: ", ")))
                            .font(.callout).foregroundStyle(.secondary).lineLimit(1)
                    }
                    Spacer()
                    if account.id == defaultId {
                        Text("Default").font(.callout).foregroundStyle(.secondary)
                    } else {
                        Button("Make default") { state.settings.defaultJiraAccountId = account.id }
                    }
                }
                .contentShape(Rectangle())
                .onTapGesture { selected = account.id }
            }
            HStack {
                Button("Add account") {
                    let account = JiraAccount()
                    state.settings.jiraAccounts.append(account)
                    if state.settings.defaultJiraAccountId == nil { state.settings.defaultJiraAccountId = account.id }
                    selected = account.id
                }
                Button("Remove", role: .destructive) { if let current { remove(current) } }
                    .disabled(current == nil)
            }
        } header: {
            Text("Jira accounts")
        } footer: {
            Text(
                "Each issue goes to the account whose projects include its key (TAO-12 → TAO); keys no account has go to the default account, which also gives the exports their Jira link."
            )
        }
        if let current, let i = accounts.firstIndex(of: current) {
            JiraAccountForm(account: $state.settings.jiraAccounts[i]).id(current.id)
        }
    }

    private var current: JiraAccount? {
        state.settings.jiraAccounts.first { $0.id == selected } ?? state.settings.defaultJiraAccount
    }

    private func remove(_ account: JiraAccount) {
        state.services.secrets.set(nil, for: account.jiraTokenKey)
        state.services.secrets.set(nil, for: account.tempoTokenKey)
        state.settings.jiraAccounts.removeAll { $0.id == account.id }
        if state.settings.defaultJiraAccountId == account.id {
            state.settings.defaultJiraAccountId = state.settings.jiraAccounts.first?.id
        }
        selected = nil
        Task { await state.refreshTempo() }
    }
}

/// One account's site, sign-in, provider and projects, with the checks.
struct JiraAccountForm: View {
    @Environment(AppState.self) private var state
    @Binding var account: JiraAccount
    @State private var loadingProjects = false

    var body: some View {
        Section {
            TextField("Name", text: $account.name, prompt: Text("e.g. Work, Client"))
            TextField("Jira site", text: $account.baseURL, prompt: Text("https://yourcompany.atlassian.net"))
            TextField("Jira email", text: $account.email)
            SecretField(title: "Jira API token", account: account.jiraTokenKey)
            Picker("Worklogs are kept in", selection: $account.provider) {
                ForEach(WorklogProvider.allCases) { Text($0.title).tag($0) }
            }
            if account.provider == .tempo {
                SecretField(title: "Tempo API token", account: account.tempoTokenKey)
                TextField("Tempo API base URL", text: $account.tempoBaseURL)
            }
            TextField("Jira account ID (filled in automatically)", text: $account.accountId)
            LabeledContent("Projects") {
                HStack {
                    Text(
                        account.projectKeys.isEmpty
                            ? String(localized: "None read yet") : account.projectKeys.joined(separator: ", ")
                    )
                    .foregroundStyle(.secondary).lineLimit(3).multilineTextAlignment(.trailing)
                    if loadingProjects { ProgressView().controlSize(.small) }
                    Button("Read from Jira") {
                        Task {
                            loadingProjects = true
                            await state.refreshProjects(account.id)
                            loadingProjects = false
                        }
                    }
                    .disabled(loadingProjects)
                }
            }
            ConnectionChecks(account: account)
        } header: {
            Text(account.title.isEmpty ? String(localized: "New account") : account.title)
        } footer: {
            Text(
                "Used to show what is already logged and for Export → Send. With Another tool, WorkLogger only exports files and checklists for this account's issues. Log in Tempo by ticket works without any key. Tokens are stored in your Keychain."
            )
        }
    }
}

/// Check Jira / Check Tempo: harmless calls that show, line by line, what works and which permission is missing,
/// plus what each token needs.
struct ConnectionChecks: View {
    @Environment(AppState.self) private var state
    let account: JiraAccount
    @State private var jira: [ConnectionCheck.Line] = []
    @State private var tempo: [ConnectionCheck.Line] = []
    @State private var checking = false
    @State private var showNeeds = false

    var body: some View {
        HStack {
            Button("Check Jira") { Task { await run(tempo: false) } }
            if account.provider == .tempo { Button("Check Tempo") { Task { await run(tempo: true) } } }
            if checking { ProgressView().controlSize(.small) }
            Spacer()
        }
        .disabled(checking)
        ForEach(jira + tempo) { line in
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Image(
                    systemName: line.ok == true
                        ? "checkmark.circle.fill" : line.ok == false ? "xmark.circle.fill" : "minus.circle"
                )
                .foregroundStyle(line.ok == true ? Color.green : line.ok == false ? Color.red : Color.secondary)
                VStack(alignment: .leading, spacing: 2) {
                    Text(line.title).fontWeight(.medium)
                    if !line.detail.isEmpty {
                        Text(line.detail).font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    }
                }
            }
        }
        DisclosureGroup("What the tokens need", isExpanded: $showNeeds) {
            VStack(alignment: .leading, spacing: 6) {
                Text(
                    "**Jira API token** (id.atlassian.com → Security → API tokens): a classic token works with your own permissions. A token with scopes needs **read:jira-user** (your account) and **read:jira-work** (issue keys and ids). Use the email of the same Atlassian account."
                )
                Text(
                    "**Tempo API token** (Tempo → Settings → API Integration → New token): **View worklogs** to show what is already in Tempo, and **Manage worklogs** for Send to Tempo. Give it enough days before it expires."
                )
                Text(
                    "**Tempo address:** https://api.tempo.io/4, or your region's (e.g. https://api.eu.tempo.io/4) if your Tempo data is hosted there."
                )
            }
            .font(.callout)
            .fixedSize(horizontal: false, vertical: true)
        }
    }

    private func run(tempo checkTempo: Bool) async {
        checking = true
        defer { checking = false }
        let check = ConnectionCheck(account: account, secrets: state.services.secrets, http: URLSession.shared)
        let result = await check.checkJira()
        jira = result.lines
        if let id = result.accountId { state.update(account.id) { $0.accountId = id } }
        tempo = checkTempo ? await check.checkTempo(accountId: result.accountId ?? account.accountId) : []
        if (jira + tempo).contains(where: { $0.ok == false }) { showNeeds = true }
        if result.accountId != nil { await state.refreshProjects(account.id) }
        await state.refreshTempo()
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
            Text(
                "Each site keeps its own cookies inside WorkLogger, separate from Safari; Sign out deletes them. HRmaster ends its session after about 25 idle minutes, so it often shows signed out; a fetch then asks you to sign in."
            )
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
                    } label: {
                        Image(systemName: "trash")
                    }
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
            Text(
                "Leave days (Szabadság and other leave codes) are logged with the hours HRmaster reports; public holidays only when it reports hours. Rest days, holidays and leave never get filled with commit time."
            )
        }
    }
}

#Preview {
    SettingsView()
        .environment(AppState.preview)
        .frame(width: 900, height: 700)
}
