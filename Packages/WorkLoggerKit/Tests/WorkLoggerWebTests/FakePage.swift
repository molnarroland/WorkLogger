import Foundation
@testable import WorkLoggerWeb

/// A page that records what the collectors open and run, and answers scripts from `answer`.
@MainActor
final class FakePage: PageDriver {
    let siteName = "Fake site"
    var signedIn = true
    /// Where opening the start page lands (e.g. Google's /u/1/ account path).
    var redirect: [URL: URL] = [:]
    /// The script's result for the page that is open, and the progress JSON it reports while running.
    var answer: (URL?, [String: Any]) -> Any? = { _, _ in "[]" }
    var progressJSON: [String] = []

    private(set) var currentURL: URL?
    private(set) var opened: [URL] = []
    private(set) var runs: [(url: URL?, arguments: [String: Any])] = []

    func open(_ url: URL) {
        opened.append(url)
        currentURL = redirect[url] ?? url
    }
    func waitUntilLoaded() async throws {}
    func run(_ script: String, arguments: [String: Any]) async throws -> Any? {
        runs.append((currentURL, arguments))
        return answer(currentURL, arguments)
    }
    func run(_ script: String, arguments: [String: Any], progress: @escaping (String) -> Void) async throws -> Any? {
        progressJSON.forEach(progress)
        return try await run(script, arguments: arguments)
    }
    func isSignedIn() async -> Bool { signedIn }
}
