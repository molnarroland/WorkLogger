import Foundation
import SwiftUI
import Testing
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

@testable import WorkLogger

final class FakeGit: GitService, @unchecked Sendable {
    private let lock = NSLock()
    /// Commits returned per repository path.
    var commits: [String: [GitCommit]] = [:]
    var failingPaths: Set<String> = []
    var email: String?
    var foundRepos: [URL] = []
    private var calls: [(path: String, authors: [String])] = []

    var logCalls: [(path: String, authors: [String])] { lock.withLock { calls } }

    func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit] {
        lock.withLock { calls.append((repoPath, authors)) }
        if failingPaths.contains(repoPath) { throw FakeError(message: "not a git repository") }
        return commits[repoPath] ?? []
    }
    func globalUserEmail() -> String? { email }
    func findRepos(root: URL, excluded: [String]) -> [URL] { foundRepos }
}

final class FakeCalendar: CalendarSource {
    var access: MacCalendarStore.Access = .notDetermined
    var grantsAccess = true
    var list: [MacCalendarInfo] = []
    var events: [Meeting] = []

    func requestAccess() async throws -> Bool {
        if grantsAccess { access = .granted }
        return grantsAccess
    }
    func calendars() -> [MacCalendarInfo] { list }
    func meetings(calendarIds: Set<String>, from: Date, to: Date) -> [Meeting] {
        events.filter { calendarIds.contains($0.calendarId) && $0.start >= from && $0.start < to }
    }
}

@MainActor
final class FakeFiles: FilePresenter {
    var folder: URL?
    var saveURL: URL?
    private(set) var copied: [String] = []
    private(set) var suggestedNames: [String] = []

    func chooseFolder() -> URL? { folder }
    func saveLocation(suggestedName: String, type: UTType) -> URL? {
        suggestedNames.append(suggestedName)
        return saveURL
    }
    func copyToPasteboard(_ text: String) { copied.append(text) }
}

final class FakeGoogle: GoogleCalendarAccount {
    let clientId: String
    let clientSecret: String
    var signInError: Error?
    var events: [Meeting] = []
    private(set) var signedOut = false

    init(clientId: String, clientSecret: String) {
        self.clientId = clientId
        self.clientSecret = clientSecret
    }

    func signIn() async throws { if let signInError { throw signInError } }
    func signOut() { signedOut = true }
    func meetings(calendarId: String, from: Date, to: Date) async throws -> [Meeting] {
        events.filter { $0.start >= from && $0.start < to }
    }
}

/// An AppState on an in-memory database with fake services. Tweak the fakes before or after creating `state`.
/// Keep the harness alive while using `state`: the services read the fakes through it.
@MainActor
final class AppHarness {
    let db: Database
    let secrets = InMemorySecrets()
    let git = FakeGit()
    let calendars = FakeCalendar()
    let files = FakeFiles()
    let tempo = FakeTempo()
    var tempoError: Error?
    var unlocks = true
    var googleEvents: [Meeting] = []
    var googleSignInError: Error?
    private(set) var googleAccounts: [FakeGoogle] = []
    var now: Date

    init(now: Date = Fixture.at("2026-03-04", "12:00"), settings: AppSettings? = nil) throws {
        Fixture.pinTimeZone()
        db = try Database.inMemory()
        self.now = now
        if let settings { try db.saveSettings(settings) }
    }

    private(set) lazy var state = AppState(
        db: db,
        services: AppDependencies(
            secrets: secrets,
            git: git,
            calendars: calendars,
            files: files,
            makeGoogle: { [unowned self] id, secret in
                let account = FakeGoogle(clientId: id, clientSecret: secret)
                account.events = googleEvents
                account.signInError = googleSignInError
                googleAccounts.append(account)
                return account
            },
            makeTempo: { [unowned self] _ in
                if let tempoError { throw tempoError }
                return tempo
            },
            unlock: { [unowned self] in unlocks },
            now: { [unowned self] in now }))

    @discardableResult
    func addRepo(_ name: String = "billing") throws -> Repo {
        try db.addRepo(path: "/w/\(name)", name: name)
        state.reloadAll()
        return try #require(state.repos.first { $0.name == name })
    }

    @discardableResult
    func addWorklog(_ w: Worklog) throws -> Worklog {
        var w = w
        w.id = try db.insertWorklog(w)
        state.reloadRange()
        return w
    }
}
