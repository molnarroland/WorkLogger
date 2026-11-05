import AppKit
import Foundation
import LocalAuthentication
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

// Everything AppState needs from the outside world, behind small protocols so tests can pass fakes. The protocols
// for git, calendars and Tempo live in their WorkLoggerKit modules; secrets and HTTP in WorkLoggerCore.

/// The Keychain tokens, kept in memory after `unlock()`. Locked, it reads nothing and ignores writes, so the Keychain
/// is never touched before the user asks.
final class KeychainStore: SecretStore, @unchecked Sendable {
    private let lock = NSLock()
    private var values: [String: String]?  // nil: locked

    func get(_ account: String) -> String? { lock.withLock { values?[account] } }

    func set(_ value: String?, for account: String) {
        let all: [String: String]? = lock.withLock {
            guard values != nil else { return nil }
            values?[account] = (value?.isEmpty ?? true) ? nil : value
            return values
        }
        if let all { Keychain.saveAll(all) }
    }

    /// Touch ID or the login password, then one Keychain read. False when cancelled or refused.
    func unlock() async -> Bool {
        do {
            guard
                try await LAContext().evaluatePolicy(
                    .deviceOwnerAuthentication, localizedReason: String(localized: "unlock your API tokens"))
            else { return false }
        } catch {
            return false
        }
        guard let all = Keychain.loadAll() else { return false }
        lock.withLock { values = all }
        return true
    }
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
    var makeTempo: (JiraAccount) throws -> any TempoService
    /// Asks for Touch ID or the password and opens the secrets; false when refused.
    var unlock: @MainActor () async -> Bool
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
            makeTempo: { try TempoClient(account: $0, secrets: secrets, http: http) },
            unlock: { await secrets.unlock() },
            now: Date.init)
    }
}
