import SwiftUI
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// Conversations grouped by day; the selected one's messages (oldest first) load underneath, with an issue field
/// to log its time. Used on the Chats page and in the Timesheet / Review inspector.
struct ChatBrowser: View {
    @Environment(AppState.self) private var state
    let chats: [ChatDay]
    @State private var selected: ChatDay.ID?

    var body: some View {
        let byDay = Dictionary(grouping: chats, by: \.day)
        VSplitView {
            List(selection: $selected) {
                ForEach(byDay.keys.sorted(), id: \.self) { day in
                    let items = byDay[day] ?? []
                    Section {
                        ForEach(items) { ChatListRow(chat: $0).tag($0.id) }
                    } header: {
                        HStack {
                            Text(DayKey.title(day))
                            Spacer()
                            Text(DurationFormat.string(items.reduce(0) { $0 + $1.activeSeconds })).monospacedDigit()
                        }
                    }
                }
            }
            .listStyle(.inset)
            .frame(minHeight: 140)

            Group {
                if let chat = chats.first(where: { $0.id == selected }) {
                    ChatMessages(chat: chat).id(chat.id)
                } else {
                    ContentUnavailableView("Select a conversation", systemImage: "bubble.left.and.bubble.right",
                                           description: Text("Its messages show here, oldest first."))
                }
            }
            .frame(maxWidth: .infinity, minHeight: 160, maxHeight: .infinity)
        }
    }
}

private struct ChatListRow: View {
    @Environment(AppState.self) private var state
    let chat: ChatDay

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: chat.type == "dm" ? "person" : "person.3").foregroundStyle(.secondary).frame(width: 18)
            Text(chat.name).lineLimit(1)
            Spacer()
            if let key = state.chatWorklog(chat)?.issueKey { Text(key).fontWeight(.medium) }
            Text("\(chat.count) msg").foregroundStyle(.secondary).monospacedDigit()
            Text(DurationFormat.string(chat.activeSeconds)).monospacedDigit().frame(minWidth: 44, alignment: .trailing)
        }
        .help("\(DayKey.hm(chat.times.first!)) – \(DayKey.hm(chat.times.last!)) · messages less than \(Int(chat.gap / 60)) minutes apart count as one stretch")
    }
}

private struct ChatMessages: View {
    @Environment(AppState.self) private var state
    let chat: ChatDay
    @State private var issue = ""

    var body: some View {
        let logged = state.chatWorklog(chat)
        let rule = state.settings.chatRules.first { $0.conversationId == chat.conversationId }
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text(chat.name).font(.headline).lineLimit(1)
                Spacer()
                if chat.activeSeconds > ChatDay.minLoggable {
                    TextField("Issue, e.g. TAO-12", text: $issue)
                        .textFieldStyle(.roundedBorder).frame(width: 140)
                        .onSubmit(log)
                    Button(logged == nil ? "Log \(DurationFormat.string(chat.activeSeconds))" : "Change issue", action: log)
                        .disabled(JiraKey.normalize(issue) == nil || logged?.status == .exported)
                }
                // A rule: this conversation goes to the ticket every day, like a meeting rule.
                Toggle("Always", isOn: Binding(
                    get: { rule != nil },
                    set: { state.setChatRule(chat, issue: $0 ? issue : nil) }))
                    .toggleStyle(.checkbox)
                    .disabled(rule == nil && JiraKey.normalize(issue) == nil)
                    .help("Log this conversation to \(rule?.issueKey ?? String(localized: "this ticket")) on every day it has messages")
            }
            ScrollView {
                VStack(alignment: .leading, spacing: 6) {
                    ForEach(state.chatLines(chat), id: \.self) { m in
                        let who = m.me ? String(localized: "Me") : (m.sender.isEmpty ? "?" : m.sender.components(separatedBy: "@")[0])
                        HStack(alignment: .firstTextBaseline, spacing: 8) {
                            Text(DayKey.hm(m.time)).monospacedDigit().foregroundStyle(.secondary)
                            Text(who).fontWeight(.medium).foregroundStyle(m.me ? Color.accentColor : Color.primary)
                            VStack(alignment: .leading, spacing: 2) {
                                if !m.text.isEmpty {
                                    Text(m.text).textSelection(.enabled).fixedSize(horizontal: false, vertical: true)
                                }
                                ForEach(m.files, id: \.self) { f in
                                    Text("\(m.me ? "You" : who) sent a file: \(f)").italic().foregroundStyle(.teal)
                                }
                            }
                        }
                        .font(.callout)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .padding(12)
        .onAppear { issue = logged?.issueKey ?? "" }
    }

    private func log() { state.logChat(chat, issue: issue) }
}

/// One "Chats" line for a day on the Timesheet and Review panels; clicking it opens the chats in the inspector.
struct ChatDaySummary: View {
    @Environment(AppState.self) private var state
    let day: String
    let open: (String) -> Void

    var body: some View {
        let chats = state.chats.filter { $0.day == day }
        if !chats.isEmpty {
            Button { open(day) } label: {
                HStack(spacing: 8) {
                    Image(systemName: WorklogSource.chat.icon).foregroundStyle(.secondary).frame(width: 18)
                    Text("Chats · \(chats.count) conversations")
                    Spacer()
                    Text(DurationFormat.string(chats.reduce(0) { $0 + $1.activeSeconds })).monospacedDigit()
                    Image(systemName: "sidebar.right").foregroundStyle(.secondary)
                }
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .help("Show this day's chats on the right")
        }
    }
}

extension View {
    /// Right-hand inspector with one day's chats, open while `day` is set.
    func chatInspector(day: Binding<String?>) -> some View {
        inspector(isPresented: Binding(get: { day.wrappedValue != nil }, set: { if !$0 { day.wrappedValue = nil } })) {
            ChatInspectorContent(day: day.wrappedValue)
                .inspectorColumnWidth(min: 320, ideal: 420, max: 640)
        }
    }
}

private struct ChatInspectorContent: View {
    @Environment(AppState.self) private var state
    let day: String?

    var body: some View {
        if let day {
            ChatBrowser(chats: state.chats.filter { $0.day == day }).id(day)
        }
    }
}
