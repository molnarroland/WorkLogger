import SwiftUI
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// "Make rule" banner for a meeting that keeps coming back without a ticket.
struct RuleSuggestion: View {
    @Environment(AppState.self) private var state
    let title: String
    let count: Int
    @State private var issue = ""

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: "lightbulb").foregroundStyle(.orange)
            Text("“\(title)” comes back \(count) times without a ticket.").lineLimit(2)
            Spacer()
            TextField("Ticket", text: $issue).textFieldStyle(.roundedBorder).frame(width: 110).onSubmit(make)
            Button("Make rule", action: make).disabled(JiraKey.normalize(issue) == nil)
        }
        .padding(10)
        .glassEffect(.regular.tint(.orange.opacity(0.12)), in: .rect(cornerRadius: 14))
    }

    private func make() {
        guard let key = JiraKey.normalize(issue) else { return }
        state.settings.meetingRules.append(MeetingRule(contains: title, issueKey: key))
        state.rebuild()
        issue = ""
    }
}

/// Orange ticket field for a meeting without one; submitting adds a rule for the meeting's title.
struct MeetingTicketField: View {
    @Environment(AppState.self) private var state
    let title: String
    @State private var issue = ""

    var body: some View {
        TextField("No issue", text: $issue)
            .textFieldStyle(.roundedBorder)
            .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(Color.orange.opacity(0.7)))
            .onSubmit {
                guard let key = JiraKey.normalize(issue) else { return }
                state.settings.meetingRules.append(MeetingRule(contains: title, issueKey: key))
                state.rebuild()
                issue = ""
            }
            .help("Type a ticket and press Return: a rule logs “\(title)” to it from now on")
    }
}

/// Accounts are added in Apple's Internet Accounts with the provider's own login page, so no OAuth client is needed.
