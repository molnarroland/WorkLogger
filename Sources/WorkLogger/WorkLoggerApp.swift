import SwiftUI
import AppKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        // Needed when launched with `swift run` (no app bundle): show a Dock icon and bring the window forward.
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
    }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
}

@main
struct WorkLoggerApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @State private var state = AppState()

    var body: some Scene {
        WindowGroup("WorkLogger") {
            ContentView()
                .environment(state)
                .frame(minWidth: 1080, minHeight: 680)
        }
        .commands {
            CommandGroup(after: .saveItem) {
                Button("Save Changes") { state.saveDirty() }
                    .keyboardShortcut("s")
                Button("Collect Range") { Task { await state.collect() } }
                    .keyboardShortcut("r")
                Button("Rebuild Drafts") { state.rebuild() }
                    .keyboardShortcut("r", modifiers: [.command, .shift])
                Divider()
                Button("Fetch Work Times from HRmaster") { Task { await state.fetchHRmaster() } }
                    .keyboardShortcut("h", modifiers: [.command, .option])
                    .disabled(!state.settings.modules.contains(.workTimes))
                Button("Fetch Google Chat") { Task { await state.fetchGoogleChat() } }
                    .keyboardShortcut("g", modifiers: [.command, .option])
                    .disabled(!state.settings.modules.contains(.chats))
            }
            PaneCommands(modules: state.settings.modules)
            // Go: move through months, like the Timesheet arrows.
            CommandMenu("Go") {
                Button("Previous Month") { state.shiftMonth(-1) }.keyboardShortcut("[")
                Button("Next Month") { state.shiftMonth(1) }.keyboardShortcut("]")
                Button("This Month") { state.setRange(.thisMonth) }.keyboardShortcut("t")
            }
        }

        // In-app browser windows for sources you sign in to. One window per source: a WebPage can only be shown
        // in one WebView, and a WindowGroup could restore or open a second copy (that crashed on launch).
        Window("Google Chat", id: WebSession.Source.googleChat.windowID) {
            WebSessionWindow(source: .googleChat).environment(state)
        }
        Window("HRmaster", id: WebSession.Source.hrmaster.windowID) {
            WebSessionWindow(source: .hrmaster).environment(state)
        }

        // App menu → Settings… (⌘,), also opened from the gear in the toolbar.
        Settings {
            SettingsView()
                .environment(state)
                .frame(width: 640, height: 720)
        }
    }
}

/// View menu: jump to a page (⌘1…⌘9 in sidebar order) and Log in Tempo (⌘L).
struct PaneCommands: Commands {
    let modules: Set<Module>
    @FocusedValue(\.selectPane) private var selectPane

    var body: some Commands {
        CommandGroup(before: .sidebar) {
            Button("Log in Tempo") { selectPane?(.byTicket) }
                .keyboardShortcut("l")
                .disabled(selectPane == nil)
            Divider()
            let panes = Step.allCases.flatMap { $0.panes(modules) }
            ForEach(Array(panes.prefix(9).enumerated()), id: \.element) { i, p in
                Button(p.rawValue) { selectPane?(p) }
                    .keyboardShortcut(KeyEquivalent(Character("\(i + 1)")))
                    .disabled(selectPane == nil)
            }
            Divider()
        }
    }
}
