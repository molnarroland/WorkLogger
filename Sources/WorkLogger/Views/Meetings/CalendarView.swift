import SwiftUI
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

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
                if let suggestion = repeatingWithoutTicket(builder) {
                    RuleSuggestion(title: suggestion.title, count: suggestion.count)
                        .padding(.horizontal, 12).padding(.top, 10)
                }
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
                            if let issue = builder.issue(for: m) {
                                Text(issue).foregroundStyle(.secondary).frame(width: 110, alignment: .trailing)
                            } else {
                                // Typing a ticket here makes a rule for this title, so next month gets it too.
                                MeetingTicketField(title: m.title).frame(width: 110)
                            }
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

    /// The meeting title that comes back most often without a ticket (at least twice), if any.
    private func repeatingWithoutTicket(_ builder: WorklogBuilder) -> (title: String, count: Int)? {
        let open = state.meetings.filter { !builder.isSkipped($0) && builder.issue(for: $0) == nil }
        return Dictionary(grouping: open, by: \.title).map { (title: $0.key, count: $0.value.count) }
            .filter { $0.count >= 2 }.max { $0.count < $1.count }
    }
}

#Preview {
    CalendarView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
