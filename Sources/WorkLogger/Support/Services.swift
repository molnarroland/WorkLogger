import Foundation
import AppKit
import UniformTypeIdentifiers
import WorkLoggerCore
import WorkLoggerStore

// Everything AppState needs from the outside world, behind small protocols so tests can pass fakes.

/// Where API tokens are kept. The app uses the Keychain.
protocol SecretStore: Sendable {
    func get(_ account: String) -> String?
    func set(_ value: String?, for account: String)
}

struct KeychainStore: SecretStore {
    func get(_ account: String) -> String? { Keychain.get(account) }
    func set(_ value: String?, for account: String) { Keychain.set(value, for: account) }
}

/// Sends HTTP requests. The app uses `URLSession.shared`.
protocol HTTPClient: Sendable {
    func send(_ request: URLRequest) async throws -> (Data, URLResponse)
}

extension URLSession: HTTPClient {
    func send(_ request: URLRequest) async throws -> (Data, URLResponse) {
        try await data(for: request)
    }
}

/// Reads repositories and history. The app runs the `git` command line tool.
protocol GitService: Sendable {
    func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit]
    func globalUserEmail() -> String?
    func findRepos(root: URL, excluded: [String]) -> [URL]
}

struct SystemGit: GitService {
    func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit] {
        try GitLogReader.log(repoPath: repoPath, since: since, until: until, authors: authors)
    }
    func globalUserEmail() -> String? { GitScanner.globalUserEmail() }
    func findRepos(root: URL, excluded: [String]) -> [URL] { GitScanner.findRepos(root: root, excluded: excluded) }
}

/// Calendars the system knows about. The app reads them through EventKit (`MacCalendarStore`).
protocol CalendarSource: AnyObject {
    var access: MacCalendarStore.Access { get }
    func requestAccess() async throws -> Bool
    func calendars() -> [MacCalendarInfo]
    func meetings(calendarIds: Set<String>, from: Date, to: Date) -> [Meeting]
}

extension MacCalendarStore: CalendarSource {}

/// A Google account read through the Calendar API (`GoogleAuth`).
protocol GoogleCalendarAccount: AnyObject {
    var clientId: String { get }
    var clientSecret: String { get }
    func signIn() async throws
    func signOut()
    func meetings(calendarId: String, from: Date, to: Date) async throws -> [Meeting]
}

/// Jira and Tempo calls needed to push worklogs (`TempoClient`).
protocol TempoService {
    func accountId() async throws -> String
    func issueId(for key: String) async throws -> Int64
    func create(_ payload: TempoWorklogPayload) async throws -> Int64
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
        panel.prompt = "Use folder"
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
