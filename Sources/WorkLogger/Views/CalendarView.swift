import SwiftUI

struct CalendarView: View {
    @Environment(AppState.self) private var state
    @State private var ruleText = ""
    @State private var ruleIssue = ""
    @State private var showAPI = false

    var body: some View {
        @Bindable var state = state
        let builder = state.makeBuilder()
        // Not HSplitView: its AppKit divider runs up under the toolbar and cuts through the centered title.
        HStack(spacing: 0) {
            Form {
                ConnectSection()
                if state.calendarAccess == .granted {
                    CalendarPicker()
                }

                Section("Logging meetings") {
                    TextField("Default issue for meetings, e.g. INT-12", text: $state.settings.defaultMeetingIssue)
                    ListField(title: "Never log meetings containing (comma separated)", values: $state.settings.skipMeetingKeywords)
                }
                Section {
                    ForEach(state.settings.meetingRules) { rule in
                        HStack {
                            Text("“\(rule.contains)”")
                            Image(systemName: "arrow.right").foregroundStyle(.secondary)
                            Text(rule.issueKey).bold()
                            Spacer()
                            Button(role: .destructive) {
                                state.settings.meetingRules.removeAll { $0.id == rule.id }
                            } label: { Image(systemName: "trash") }
                            .buttonStyle(.borderless)
                        }
                    }
                    HStack {
                        TextField("Title contains", text: $ruleText)
                        TextField("Issue", text: $ruleIssue).frame(width: 110)
                        Button("Add") {
                            state.settings.meetingRules.append(MeetingRule(contains: ruleText, issueKey: ruleIssue.uppercased()))
                            ruleText = ""
                            ruleIssue = ""
                        }
                        .disabled(ruleText.isEmpty || ruleIssue.isEmpty)
                    }
                } header: {
                    Text("Rules")
                } footer: {
                    Text("An issue key in the meeting title or description wins, then the first matching rule, then the default issue.")
                }

                Section {
                    DisclosureGroup("Google Calendar API with your own OAuth client", isExpanded: $showAPI) {
                        TextField("OAuth client ID", text: $state.settings.googleClientId)
                        SecretField(title: "OAuth client secret", account: SecretKey.googleSecret)
                        TextField("Calendar ID", text: $state.settings.googleCalendarId)
                        HStack {
                            if state.googleConnected {
                                Label("Connected", systemImage: "checkmark.circle.fill").foregroundStyle(.green)
                                Spacer()
                                Button("Disconnect") { state.disconnectGoogle() }
                            } else {
                                Spacer()
                                Button("Connect") { Task { await state.connectGoogle() } }
                                    .disabled(state.settings.googleClientId.isEmpty || state.isBusy)
                            }
                        }
                    }
                } footer: {
                    Text("Only needed when you can't add the Google account to macOS. Requires a “Desktop app” OAuth client in Google Cloud.")
                }
            }
            .formStyle(.grouped)
            .frame(width: 440)
            .frame(maxHeight: .infinity)

            Divider()

            VStack(alignment: .leading, spacing: 0) {
                HStack {
                    Text("Meetings in range").font(.title2.weight(.semibold))
                    Spacer()
                    Button("Read meetings") { Task { await state.refreshMeetings() } }
                        .buttonStyle(.glass)
                        .disabled(!state.canReadMeetings || state.isBusy)
                }
                .padding(12)
                Divider()
                if state.meetings.isEmpty {
                    ContentUnavailableView("No meetings in this range", systemImage: "calendar",
                                           description: Text(state.canReadMeetings ? "Choose Read meetings to load them." : "Add an online account and pick your calendars first."))
                } else {
                    List(state.meetings) { m in
                        HStack(spacing: 10) {
                            Toggle("", isOn: Binding(get: { !m.skipped }, set: { state.setSkipped(m, !$0) }))
                                .labelsHidden()
                                .help("Log this meeting")
                            VStack(alignment: .leading) {
                                Text(m.title).lineLimit(1)
                                Text("\(DayKey.title(DayKey.key(m.start))) · \(DayKey.hm(m.start))–\(DayKey.hm(m.end))")
                                    .font(.caption).foregroundStyle(.secondary)
                            }
                            Spacer()
                            Text(DurationFormat.string(m.duration)).monospacedDigit()
                            Text(builder.issue(for: m) ?? "No issue")
                                .foregroundStyle(builder.issue(for: m) == nil ? Color.orange : Color.secondary)
                                .frame(width: 90, alignment: .trailing)
                        }
                        .opacity(builder.isSkipped(m) ? 0.5 : 1)
                    }
                    .listStyle(.inset)
                }
            }
            .frame(minWidth: 420, maxHeight: .infinity)
        }
        .onAppear { state.refreshCalendars() }
    }
}

/// Accounts are added in Apple's Internet Accounts with the provider's own login page, so no OAuth client is needed.
private struct ConnectSection: View {
    @Environment(AppState.self) private var state

    var body: some View {
        let accounts = Set(state.macCalendars.map(\.account)).sorted()
        Section {
            HStack(alignment: .top, spacing: 12) {
                Image(systemName: "1.circle.fill").font(.title2).foregroundStyle(.secondary)
                VStack(alignment: .leading, spacing: 6) {
                    Text("Add your calendar account in macOS")
                    Text("Add Account…, sign in, keep Calendars switched on. Any account that syncs calendars to macOS works: Google, Microsoft Exchange / Outlook, iCloud, Yahoo or CalDAV.")
                        .font(.callout).foregroundStyle(.secondary)
                    Button {
                        state.openInternetAccounts()
                    } label: {
                        Label("Open Internet Accounts…", systemImage: "person.crop.circle.badge.plus")
                    }
                    .buttonStyle(.borderedProminent)
                }
            }
            HStack(alignment: .top, spacing: 12) {
                Image(systemName: "2.circle.fill").font(.title2).foregroundStyle(.secondary)
                VStack(alignment: .leading, spacing: 6) {
                    switch state.calendarAccess {
                    case .granted:
                        Label("WorkLogger can read your calendars", systemImage: "checkmark.circle.fill")
                            .foregroundStyle(.green)
                        if !accounts.isEmpty {
                            Text("Accounts: " + accounts.joined(separator: ", "))
                                .font(.callout).foregroundStyle(.secondary)
                        }
                    case .notDetermined:
                        Text("Let WorkLogger read your calendars")
                        Button("Allow calendar access") { Task { await state.requestCalendarAccess() } }
                    case .denied, .writeOnly:
                        Text("Calendar access is turned off for WorkLogger")
                        Text("Turn on full access under Privacy & Security → Calendars, then come back.")
                            .font(.callout).foregroundStyle(.secondary)
                        HStack {
                            Button("Open Privacy settings") { MacCalendarStore.openPrivacySettings() }
                            Button("Check again") { state.refreshCalendars() }
                        }
                    }
                }
            }
        } header: {
            Text("Online Accounts")
        } footer: {
            Text("Read-only. Your password stays with macOS; WorkLogger never sees it.")
        }
    }
}

private struct CalendarPicker: View {
    @Environment(AppState.self) private var state

    var body: some View {
        let byAccount = Dictionary(grouping: state.macCalendars, by: \.account)
        Section {
            if state.macCalendars.isEmpty {
                Text("No calendars yet. A newly added account can take a minute to sync.")
                    .foregroundStyle(.secondary)
            }
            ForEach(byAccount.keys.sorted(), id: \.self) { account in
                Text(account).font(.caption).foregroundStyle(.secondary)
                ForEach(byAccount[account] ?? []) { cal in
                    Toggle(isOn: Binding(get: { state.isCalendarSelected(cal.id) },
                                         set: { state.setCalendar(cal.id, selected: $0) })) {
                        HStack(spacing: 8) {
                            Circle().fill(cal.color).frame(width: 10, height: 10)
                            Text(cal.title)
                        }
                    }
                }
            }
        } header: {
            HStack {
                Text("Calendars to read")
                Spacer()
                Button("Refresh") { state.refreshCalendars() }.buttonStyle(.borderless)
            }
        }
    }
}

#Preview {
    CalendarView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
