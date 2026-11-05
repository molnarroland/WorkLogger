import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

/// Chat histories of the range: per day, each conversation with its message count and active time.
struct ChatsView: View {
    @Environment(AppState.self) private var state
    @State private var showPaste = false
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 0) {
            HStack {
                Text("Chats").font(.title2.weight(.semibold))
                Text(
                    "\(state.chats.count) conversations · \(DurationFormat.string(state.chats.reduce(0) { $0 + $1.activeSeconds })) active"
                )
                .foregroundStyle(.secondary)
                Spacer()
                Button("Clear chats in range", role: .destructive) { state.clearChatsInRange() }
                    .buttonStyle(.glass)
                    .disabled(state.chats.isEmpty)
                TextField(
                    "Your Chat email", text: $state.settings.chatEmail,
                    prompt: Text(state.settings.authorEmails.first ?? "you@company.com")
                )
                .textFieldStyle(.roundedBorder).frame(width: 220)
                SignInChip(source: .googleChat)
                Button("Google Chat…") { openWindow(id: WebSession.Source.googleChat.windowID) }
                    .buttonStyle(.glass)
                    .help("Opens Google Chat inside WorkLogger; sign in there once")
                Button {
                    Task { await state.fetchGoogleChat() }
                } label: {
                    Label("Fetch range", systemImage: "arrow.down.circle")
                }
                .buttonStyle(.glass)
                .disabled(state.isBusy)
                .help("Searches your messages and mentions in Google Chat for the selected dates")
                Button {
                    showPaste = true
                } label: {
                    Label("Paste export…", systemImage: "doc.on.clipboard")
                }
                .buttonStyle(.glassProminent)
            }
            .padding(12)
            Divider()

            if state.chats.isEmpty {
                ContentUnavailableView(
                    "No chats in this range", systemImage: "bubble.left.and.bubble.right",
                    description: Text("Choose Paste export… and paste the exported chat history.")
                )
                .frame(maxHeight: .infinity)
            } else {
                ChatBrowser(chats: state.chats)
            }
        }
        .sheet(isPresented: $showPaste) { ChatPasteSheet() }
    }
}

private struct ChatPasteSheet: View {
    @Environment(AppState.self) private var state
    @Environment(\.dismiss) private var dismiss
    @State private var text = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Paste chat export").font(.title2.weight(.semibold))
            Text("Paste the base64 text of the export (zlib-compressed JSON) or the JSON itself.")
                .foregroundStyle(.secondary)
            TextEditor(text: $text)
                .font(.system(.body, design: .monospaced))
                .border(Color.secondary.opacity(0.3))
            HStack {
                Spacer()
                Button("Cancel") { dismiss() }.buttonStyle(.glass)
                    .keyboardShortcut(.cancelAction)
                Button("Import") { if state.importChats(text) { dismiss() } }
                    .buttonStyle(.glassProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(text.isEmpty)
            }
            if state.status.hasPrefix("Chat import failed") {
                Text(state.status).foregroundStyle(.red).font(.callout)
            }
        }
        .padding(20)
        .frame(width: 720, height: 520)
    }
}

#Preview("One conversation") {
    ChatsView()
        .environment(AppState.preview)
        .frame(width: 1000, height: 640)
}

#Preview("No chats") {
    ChatsView()
        .environment(AppState.empty)
        .frame(width: 1000, height: 640)
}
