#if DEBUG
import Foundation
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// Debug builds only: launch with `-WLProbeScript <file.js> -WLProbeOut <file>` to open the HRmaster window,
/// run the script (an async function body) on its page and write the result to the out file. Used to inspect
/// signed-in pages, whose cookies only this app can use.
@MainActor
enum WebDebugProbe {
    static var script: String? { UserDefaults.standard.string(forKey: "WLProbeScript") }
    static var out: String? { UserDefaults.standard.string(forKey: "WLProbeOut") }
    static var isActive: Bool { script != nil && out != nil }

    /// `-WLProbeFetch chat|hrmaster -WLProbeFrom 2026-10-01 -WLProbeTo 2026-10-02 -WLProbeOut <file>` (use with
    /// `-WLDatabase <copy>`): runs the background fetch and writes every status line it showed.
    static var fetch: String? { UserDefaults.standard.string(forKey: "WLProbeFetch") }

    static func runFetch(_ state: AppState) async {
        guard let kind = fetch, let outPath = out,
              let from = UserDefaults.standard.string(forKey: "WLProbeFrom").flatMap(DayKey.parse),
              let to = UserDefaults.standard.string(forKey: "WLProbeTo").flatMap(DayKey.parse) else { return }
        state.from = from
        state.to = to
        var lines: [String] = []
        let watch = Task { @MainActor in
            while !Task.isCancelled {
                if lines.last != state.status { lines.append(state.status) }
                try? await Task.sleep(for: .milliseconds(300))
            }
        }
        let needsSignIn = kind == "chat" ? await state.fetchGoogleChat() : await state.fetchHRmaster()
        watch.cancel()
        lines.append(state.status)
        lines.append("needsSignIn=\(needsSignIn) chats=\(state.chats.count) messages=\(state.chats.reduce(0) { $0 + $1.count }) sessions=\(state.sessions.count) days=\(state.dayRecords.count)")
        try? lines.joined(separator: "\n").write(toFile: outPath, atomically: true, encoding: .utf8)
    }

    static func run(_ session: WebSession) async {
        guard let scriptPath = script, let outPath = out else { return }
        var result = ""
        do {
            try await session.waitUntilLoaded()
            try await Task.sleep(for: .seconds(6))   // the Angular page renders after load
            let js = try String(contentsOfFile: scriptPath, encoding: .utf8)
            // A script returning "RETRY…" is run again (e.g. while you sign in in the window), for up to 5 minutes.
            let end = ContinuousClock.now + .seconds(300)
            repeat {
                let value = try? await session.run(js, arguments: [:])
                result = (value as? String) ?? String(describing: value)
                if !result.hasPrefix("RETRY") { break }
                try await Task.sleep(for: .seconds(5))
            } while ContinuousClock.now < end
        } catch {
            result = "ERROR: \(error)"
        }
        await session.keepSessionCookies(host: session.page.url?.host())
        let cookies = await session.store.httpCookieStore.allCookies().filter { !$0.domain.contains("google") }
            .map { "\($0.domain) \($0.name) session=\($0.isSessionOnly) expires=\($0.expiresDate.map { "\($0)" } ?? "-")" }
            .joined(separator: "\n")
        try? ("URL: \(session.page.url?.absoluteString ?? "-")\nCOOKIES:\n\(cookies)\n" + result).write(toFile: outPath, atomically: true, encoding: .utf8)
    }
}
#endif
