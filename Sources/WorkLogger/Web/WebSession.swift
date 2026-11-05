import SwiftUI
import WebKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// A signed-in website the app reads from, with its own persistent cookies and storage (separate from Safari
/// and from the other sources). The user signs in once in its window; the app then drives the same page.
@MainActor @Observable
final class WebSession: PageDriver {
    enum Source: String, CaseIterable, Identifiable, Codable {
        case googleChat = "google-chat", hrmaster
        var id: String { rawValue }
        var title: String { self == .googleChat ? "Google Chat" : "HRmaster" }
        var windowID: String { "web-" + rawValue }
        /// Fixed per source so the login is found again after a restart. Google Chat keeps the spike's store.
        var storeID: UUID {
            switch self {
            case .googleChat: return UUID(uuidString: "6F1C2B0E-6B7A-4E59-9C61-3A1B9D0E7C21")!
            case .hrmaster: return UUID(uuidString: "0B8E4D52-1F3C-4A7B-9E2D-5C6A7B8C9D10")!
            }
        }
    }

    /// Google refuses sign-in in browsers it doesn't recognise; Safari's user agent is accepted.
    static let safariUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15"

    static let googleChat = WebSession(.googleChat)
    static let hrmaster = WebSession(.hrmaster)
    static func of(_ source: Source) -> WebSession { source == .googleChat ? googleChat : hrmaster }

    let source: Source
    let page: WebPage
    let store: WKWebsiteDataStore
    /// Result of the last sign-in check, nil before the first one.
    var signedIn: Bool?

    private init(_ source: Source) {
        self.source = source
        store = WKWebsiteDataStore(forIdentifier: source.storeID)
        var config = WebPage.Configuration()
        config.websiteDataStore = store
        page = WebPage(configuration: config)
        page.customUserAgent = Self.safariUserAgent
    }

    func open(_ url: URL) { page.load(URLRequest(url: url)) }

    // PageDriver
    var siteName: String { source.title }
    var currentURL: URL? { page.url }
    func isSignedIn() async -> Bool { await checkSignedIn() }
    func waitUntilLoaded() async throws { try await waitUntilLoaded(timeout: .seconds(45)) }

    /// Signed in when the site's session cookie is stored: Google's SID, HRmaster's ASP.NET sign-in cookie.
    @discardableResult
    func checkSignedIn(host: String? = nil) async -> Bool {
        let cookies = await store.httpCookieStore.allCookies()
        let ok: Bool
        switch source {
        case .googleChat:
            ok = cookies.contains { $0.domain.hasSuffix("google.com") && ($0.name == "SID" || $0.name == "__Secure-1PSID") }
        case .hrmaster:
            let host = host?.lowercased() ?? ""
            // HRmaster's sign-in cookie (ASP.NET); other cookies of the site exist while signed out too.
            ok = !host.isEmpty && cookies.contains { $0.name == ".AspNet.ApplicationCookie"
                && host.hasSuffix($0.domain.trimmingCharacters(in: CharacterSet(charactersIn: ".")).lowercased()) }
        }
        signedIn = ok
        return ok
    }

    /// HRmaster's login cookies are session-only, which WebKit drops when the app quits. Re-store the site's
    /// session cookies with an expiry so the login is kept; the server still ends the session when it wants.
    // ponytail: fixed 14 days; HRmaster asks for a new sign-in once its own session ends.
    func keepSessionCookies(host: String?) async {
        guard let host = host?.lowercased(), !host.isEmpty else { return }
        let cookies = store.httpCookieStore
        for c in await cookies.allCookies() where c.isSessionOnly
            && host.hasSuffix(c.domain.trimmingCharacters(in: CharacterSet(charactersIn: ".")).lowercased()) {
            var props = c.properties ?? [:]
            props[.discard] = nil
            props[.expires] = Date().addingTimeInterval(14 * 86_400)
            if let kept = HTTPCookie(properties: props) { await cookies.setCookie(kept) }
        }
    }

    func signOut() async {
        await store.removeData(ofTypes: WKWebsiteDataStore.allWebsiteDataTypes(), modifiedSince: .distantPast)
        signedIn = false
    }

    /// Runs a script (async function body); retried when a redirect replaces the page while it runs, which WebKit
    /// reports as "completion handler … no longer reachable".
    func run(_ script: String, arguments: [String: Any]) async throws -> Any? { try await run(script, arguments: arguments, attempts: 4) }

    func run(_ script: String, arguments: [String: Any], attempts: Int) async throws -> Any? {
        for attempt in 1... {
            do { return try await page.callJavaScript(script, arguments: arguments) } catch where attempt < attempts {
                try await Task.sleep(for: .seconds(3))
                try await waitUntilLoaded()
            }
        }
        return nil
    }

    /// Runs a long script that keeps its progress (a JSON string) in `window.__wlProgress`, reported every second.
    func run(_ script: String, arguments: [String: Any], progress: @escaping (String) -> Void) async throws -> Any? {
        let poll = Task { @MainActor [page] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(1))
                if let p = try? await page.callJavaScript("return window.__wlProgress ?? null") as? String { progress(p) }
            }
        }
        defer { poll.cancel() }
        return try await run(script, arguments: arguments)
    }

    /// Waits for the current navigation to finish (the page may redirect a few times).
    func waitUntilLoaded(timeout: Duration) async throws {
        let clock = ContinuousClock(), end = clock.now + timeout
        try await Task.sleep(for: .milliseconds(500))
        while page.isLoading {
            guard clock.now < end else { throw WebSessionError.timeout(source.title) }
            try await Task.sleep(for: .milliseconds(200))
        }
    }
}

/// The source's browser window: sign in here; fetching runs in the same page so you can watch it.
struct WebSessionWindow: View {
    @Environment(AppState.self) private var state
    let source: WebSession.Source

    var body: some View {
        let session = WebSession.of(source)
        VStack(spacing: 0) {
            HStack(spacing: 8) {
                Text(session.page.url?.absoluteString ?? "").lineLimit(1).truncationMode(.middle)
                    .foregroundStyle(.secondary).frame(maxWidth: .infinity, alignment: .leading)
                Button(source == .hrmaster ? "Work times page" : "Reload") { start() }
                if session.page.isLoading { ProgressView().controlSize(.small) }
                switch session.signedIn {
                case true?: Label("Signed in", systemImage: "checkmark.circle.fill").foregroundStyle(.green)
                case false?: Label("Not signed in", systemImage: "xmark.circle").foregroundStyle(.orange)
                case nil: EmptyView()
                }
                Button("Check sign-in") { Task { await session.checkSignedIn(host: hrmasterHost) } }
                Button("Sign out", role: .destructive) { Task { await session.signOut(); start() } }
            }
            .padding(10)
            Divider()
            WebView(session.page)
        }
        .frame(minWidth: 900, minHeight: 680)
        .navigationTitle("\(source.title) · WorkLogger")
        .onChange(of: session.page.isLoading) { _, loading in
            guard !loading else { return }
            Task {
                if source == .hrmaster { await session.keepSessionCookies(host: hrmasterHost) }
                if state.signInNeeded == source, await signedInNow() {
                    state.signInNeeded = nil
                    if source == .googleChat { await state.fetchGoogleChat() } else { await state.fetchHRmaster() }
                }
            }
        }
        .onAppear {
            if session.page.url == nil { start() }
            Task { await session.checkSignedIn(host: hrmasterHost) }
            #if DEBUG
            if source == .hrmaster && WebDebugProbe.isActive { Task { await WebDebugProbe.run(session) } }
            #endif
        }
    }

    /// After signing in: Google has its session cookie; HRmaster has left its login page for the Datacenter.
    private func signedInNow() async -> Bool {
        let session = WebSession.of(source)
        switch source {
        case .googleChat: return await session.checkSignedIn()
        case .hrmaster:
            guard let url = session.page.url, url.host() == hrmasterHost else { return false }
            return url.path().contains("/Datacenter") && !url.path().contains("/Account/Login")
        }
    }

    private var hrmasterHost: String? { HRmasterCollector.normalize(state.settings.hrmasterURL)?.host() }

    private func start() {
        let session = WebSession.of(source)
        switch source {
        case .googleChat: session.open(URL(string: "https://chat.google.com/")!)
        case .hrmaster: if let base = HRmasterCollector.normalize(state.settings.hrmasterURL) { session.open(HRmasterCollector.workingTimeURL(base)) }
        }
    }
}
