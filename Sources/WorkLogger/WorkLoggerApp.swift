import AppKit
import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
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
    #if DEBUG
        @State private var state = ScreenshotTour.isActive ? AppState.demo : AppState()
    #else
        @State private var state = AppState()
    #endif

    // Each part is its own small Scene / Commands type: one big nested body made the Swift runtime overflow the
    // stack while resolving its type on launch.
    var body: some Scene {
        MainWindow(state: state)
        MenuBarItem(state: state)
        SignInWindows(state: state)
        // App menu → Settings… (⌘,), also opened from the gear in the toolbar.
        Settings {
            SettingsView()
                .environment(state)
                .frame(width: 640, height: 560)
        }
    }
}

/// One main window; the menu bar item brings it back.
struct MainWindow: Scene {
    let state: AppState

    var body: some Scene {
        Window("WorkLogger", id: "main") {
            ContentView()
                .environment(state)
                .frame(minWidth: 700, minHeight: 560)
        }
        // Always open at launch: as a single Window it otherwise follows window restoration, and after quitting with
        // the window closed (closing it quits the app) WorkLogger started with no window at all.
        .defaultLaunchBehavior(.presented)
        .commands {
            FileCommands(state: state)
            PaneCommands(modules: state.settings.modules)
            GoCommands(state: state)
        }
    }
}

/// Menu bar item: today's time and quick actions (Settings → General can hide it).
struct MenuBarItem: Scene {
    @Bindable var state: AppState

    var body: some Scene {
        MenuBarExtra(isInserted: $state.settings.showMenuBarItem) {
            MenuBarView().environment(state)
        } label: {
            MenuBarLabel().environment(state)
        }
        .menuBarExtraStyle(.window)
    }
}

/// In-app browser windows for sources you sign in to. One window per source: a WebPage can only be shown in one
/// WebView, and a WindowGroup could restore or open a second copy (that crashed on launch).
struct SignInWindows: Scene {
    let state: AppState

    var body: some Scene {
        Window("Google Chat", id: WebSession.Source.googleChat.windowID) {
            WebSessionWindow(source: .googleChat).environment(state)
        }
        .defaultLaunchBehavior(.suppressed)  // opened when needed (sign-in, Google Chat…), not restored at launch
        Window("HRmaster", id: WebSession.Source.hrmaster.windowID) {
            WebSessionWindow(source: .hrmaster).environment(state)
        }
        .defaultLaunchBehavior(.suppressed)
    }
}

/// File menu: save, collect, rebuild and the site fetches.
struct FileCommands: Commands {
    let state: AppState

    var body: some Commands {
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
    }
}

/// Go: move through months, like the Timesheet arrows.
struct GoCommands: Commands {
    let state: AppState

    var body: some Commands {
        CommandMenu("Go") {
            Button("Previous Month") { state.shiftMonth(-1) }.keyboardShortcut("[")
            Button("Next Month") { state.shiftMonth(1) }.keyboardShortcut("]")
            Button("This Month") { state.setRange(.thisMonth) }.keyboardShortcut("t")
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
                Button(String(localized: p.title)) { selectPane?(p) }
                    .keyboardShortcut(KeyEquivalent(Character("\(i + 1)")))
                    .disabled(selectPane == nil)
            }
            Divider()
        }
    }
}
