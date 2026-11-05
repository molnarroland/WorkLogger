import Foundation

/// A signed-in web page the collectors drive: open a URL, wait for it, run a script in it. The app's `WebSession`
/// (WebKit) is the real one; tests use a fake that returns canned script results.
@MainActor
public protocol PageDriver: AnyObject {
    /// The site's name, for messages ("Google Chat").
    var siteName: String { get }
    var currentURL: URL? { get }
    func open(_ url: URL)
    func waitUntilLoaded() async throws
    /// Runs a script (an async function body) and returns its result.
    func run(_ script: String, arguments: [String: Any]) async throws -> Any?
    /// Like `run`, reporting the script's `window.__wlProgress` (a JSON string) while it runs.
    func run(_ script: String, arguments: [String: Any], progress: @escaping (String) -> Void) async throws -> Any?
    func isSignedIn() async -> Bool
}

public enum WebSessionError: LocalizedError {
    case timeout(String), notSignedIn(String), script(String)
    public var errorDescription: String? {
        switch self {
        case .timeout(let s): return String(localized: "\(s) did not finish loading.", bundle: .module)
        case .notSignedIn(let s): return String(localized: "Sign in to \(s) in its window first.", bundle: .module)
        case .script(let m): return m
        }
    }
}

/// This module's resources (its String Catalog), for tests.
enum WebResources {
    static var bundle: Bundle { .module }
}
