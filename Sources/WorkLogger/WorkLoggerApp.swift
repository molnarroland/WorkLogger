import SwiftUI
import AppKit

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
            }
        }

        // App menu → Settings… (⌘,), also opened from the gear in the toolbar.
        Settings {
            SettingsView()
                .environment(state)
                .frame(width: 640, height: 720)
        }
    }
}
