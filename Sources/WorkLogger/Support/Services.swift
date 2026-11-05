import AppKit
import Foundation
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

// Everything AppState needs from the outside world, behind small protocols so tests can pass fakes. The protocols
// for git, calendars and Tempo live in their WorkLoggerKit modules; secrets and HTTP in WorkLoggerCore.

struct KeychainStore: SecretStore {

    func get(_ account: String) -> String? { Keychain.get(account) }
    func set(_ value: String?, for account: String) { Keychain.set(value, for: account) }
}

/// Open and save panels and the pasteboard.
@MainActor
protocol FilePresenter {
    /// Asks for a folder; nil when cancelled.
    func chooseFolder() -> URL?
    /// Asks where to save a file; nil when cancelled.
    func saveLocation(suggestedName: String, type: UTType) -> URL?
    func copyToPasteboard(_ text: String)
}

struct AppKitFilePresenter: FilePresenter {
    func chooseFolder() -> URL? {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.prompt = String(localized: "Use folder")
        return panel.runModal() == .OK ? panel.url : nil
    }

    func saveLocation(suggestedName: String, type: UTType) -> URL? {
        let panel = NSSavePanel()
        panel.nameFieldStringValue = suggestedName
        panel.allowedContentTypes = [type]
        return panel.runModal() == .OK ? panel.url : nil
    }

    func copyToPasteboard(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }
}

/// The services an `AppState` works with.
struct AppDependencies {
    var secrets: any SecretStore
    var git: any GitService
    var calendars: any CalendarSource
    var files: any FilePresenter
    var makeGoogle: (_ clientId: String, _ clientSecret: String) -> any GoogleCalendarAccount
    var makeTempo: (AppSettings) throws -> any TempoService
    var now: () -> Date

    /// The real Keychain, git, EventKit, AppKit panels and network.
    @MainActor
    static var live: AppDependencies {
        let secrets = KeychainStore()
        let http: any HTTPClient = URLSession.shared
        return AppDependencies(
            secrets: secrets,
            git: SystemGit(),
            calendars: MacCalendarStore(),
            files: AppKitFilePresenter(),
            makeGoogle: { GoogleAuth(clientId: $0, clientSecret: $1, secrets: secrets, http: http) },
            makeTempo: { try TempoClient(settings: $0, secrets: secrets, http: http) },
            now: Date.init)
    }
}
