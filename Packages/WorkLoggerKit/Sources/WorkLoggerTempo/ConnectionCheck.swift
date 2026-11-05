import Foundation
import WorkLoggerCore

/// Settings → Tempo API: checks the Jira site, email and token, and the Tempo token, with harmless calls, and infers from
/// the answers which permissions (scopes) the tokens have. Tokens don't say their scopes, so 401 is read as "token
/// rejected", 403 as "missing permission" and 404 as "wrong address".
public struct ConnectionCheck: Sendable {
    public struct Line: Identifiable, Hashable, Sendable {
        public var title: String
        /// nil: not checked (something before it failed).
        public var ok: Bool?
        public var detail: String
        public var id: String { title }
    }

    let settings: AppSettings
    let jiraToken: String
    let tempoToken: String
    let http: any HTTPClient
    let today: String

    public init(settings: AppSettings, secrets: any SecretStore, http: any HTTPClient, now: Date = Date()) {
        self.settings = settings
        self.jiraToken = secrets.get(SecretKey.jiraToken) ?? ""
        self.tempoToken = secrets.get(SecretKey.tempoToken) ?? ""
        self.http = http
        self.today = DayKey.key(now)
    }

    var jiraSite: String { settings.jiraBaseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ ")) }
    var tempoBase: String { settings.tempoBaseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ ")) }

    /// Jira: the site answers, the email + token sign in (your account), and issues can be read (`read:jira-work`).
    /// Returns the lines and, when signed in, your Jira account id.
    public func checkJira() async -> (lines: [Line], accountId: String?) {
        let signIn = String(localized: "Site, email and API token", bundle: .module)
        let work = String(localized: "Read issues (read:jira-work)", bundle: .module)
        guard URL(string: jiraSite)?.host() != nil else {
            return (
                [
                    Line(
                        title: signIn, ok: false,
                        detail: String(
                            localized: "Enter your Jira site, e.g. https://yourcompany.atlassian.net.", bundle: .module)),
                    Line(title: work, ok: nil, detail: ""),
                ], nil
            )
        }
        guard !settings.jiraEmail.isEmpty, !jiraToken.isEmpty else {
            return (
                [
                    Line(
                        title: signIn, ok: false,
                        detail: String(localized: "Enter your Jira email and API token.", bundle: .module)),
                    Line(title: work, ok: nil, detail: ""),
                ], nil
            )
        }
        struct Me: Decodable {
            let accountId: String
            let displayName: String?
        }
        let me = await get(jiraSite + "/rest/api/3/myself", auth: jiraAuth)
        var lines: [Line] = []
        var accountId: String?
        switch me {
        case .ok(let data):
            let user = try? JSONDecoder().decode(Me.self, from: data)
            accountId = user?.accountId
            lines.append(
                Line(
                    title: signIn, ok: true,
                    detail: String(
                        localized: "Signed in as \(user?.displayName ?? user?.accountId ?? "?") (read:jira-user).",
                        bundle: .module)))
        case .status(401):
            lines.append(
                Line(
                    title: signIn, ok: false,
                    detail: String(
                        localized: "Jira rejected the email or API token. Check both, or create a new token.", bundle: .module)))
        case .status(403):
            lines.append(
                Line(
                    title: signIn, ok: false,
                    detail: String(localized: "The token can't read your account: it needs read:jira-user.", bundle: .module)))
        case .status(404):
            lines.append(
                Line(
                    title: signIn, ok: false,
                    detail: String(
                        localized: "No Jira at this address. Use your site, e.g. https://yourcompany.atlassian.net.",
                        bundle: .module)))
        case .status(let code):
            lines.append(
                Line(title: signIn, ok: false, detail: String(localized: "Jira answered with HTTP \(code).", bundle: .module)))
        case .unreachable(let message):
            lines.append(
                Line(
                    title: signIn, ok: false, detail: String(localized: "Can't reach the Jira site: \(message)", bundle: .module))
            )
        }
        guard accountId != nil else { return (lines + [Line(title: work, ok: nil, detail: "")], nil) }
        switch await get(jiraSite + "/rest/api/3/project/search?maxResults=1", auth: jiraAuth) {
        case .ok:
            lines.append(Line(title: work, ok: true, detail: String(localized: "Issue keys can be looked up.", bundle: .module)))
        case .status(let code) where code == 401 || code == 403:
            lines.append(
                Line(
                    title: work, ok: false,
                    detail: String(
                        localized:
                            "The token can't read issues: it needs read:jira-work (needed to send to Tempo and to name Tempo's issues).",
                        bundle: .module)))
        case .status(let code):
            lines.append(
                Line(title: work, ok: false, detail: String(localized: "Jira answered with HTTP \(code).", bundle: .module)))
        case .unreachable(let message):
            lines.append(Line(title: work, ok: false, detail: message))
        }
        return (lines, accountId)
    }

    /// Tempo: the token is accepted and can view your worklogs; creating worklogs is probed with an empty worklog,
    /// which Tempo rejects as invalid (400) when the token may create, and refuses (403) when it may not.
    public func checkTempo(accountId: String?) async -> [Line] {
        let view = String(localized: "View worklogs", bundle: .module)
        let manage = String(localized: "Create worklogs (for Send to Tempo)", bundle: .module)
        guard !tempoToken.isEmpty else {
            return [
                Line(title: view, ok: false, detail: String(localized: "Enter your Tempo API token.", bundle: .module)),
                Line(title: manage, ok: nil, detail: ""),
            ]
        }
        guard let accountId, !accountId.isEmpty else {
            return [
                Line(
                    title: view, ok: nil,
                    detail: String(localized: "Check Jira first: Tempo needs your Jira account id.", bundle: .module)),
                Line(title: manage, ok: nil, detail: ""),
            ]
        }
        var lines: [Line] = []
        switch await get(
            tempoBase + "/worklogs/user/\(accountId)?from=\(today)&to=\(today)&limit=1", auth: "Bearer \(tempoToken)")
        {
        case .ok:
            lines.append(Line(title: view, ok: true, detail: String(localized: "Your worklogs can be read.", bundle: .module)))
        case .status(401):
            return [
                Line(
                    title: view, ok: false,
                    detail: String(localized: "Tempo rejected the token: it is wrong, expired or revoked.", bundle: .module)),
                Line(title: manage, ok: nil, detail: ""),
            ]
        case .status(403):
            lines.append(
                Line(
                    title: view, ok: false,
                    detail: String(
                        localized: "The token can't view worklogs: give it the “View worklogs” permission.", bundle: .module)))
        case .status(404):
            return [
                Line(
                    title: view, ok: false,
                    detail: String(
                        localized:
                            "No Tempo API at this address. Use https://api.tempo.io/4 (or your region's, e.g. https://api.eu.tempo.io/4).",
                        bundle: .module)),
                Line(title: manage, ok: nil, detail: ""),
            ]
        case .status(let code):
            lines.append(
                Line(title: view, ok: false, detail: String(localized: "Tempo answered with HTTP \(code).", bundle: .module)))
        case .unreachable(let message):
            return [
                Line(title: view, ok: false, detail: String(localized: "Can't reach Tempo: \(message)", bundle: .module)),
                Line(title: manage, ok: nil, detail: ""),
            ]
        }
        switch await post(tempoBase + "/worklogs", body: "{}", auth: "Bearer \(tempoToken)") {
        case .status(400):
            lines.append(
                Line(
                    title: manage, ok: true,
                    detail: String(
                        localized: "Probably allowed (an empty test worklog was refused as invalid, nothing was saved).",
                        bundle: .module)))
        case .status(401), .status(403):
            lines.append(
                Line(
                    title: manage, ok: false,
                    detail: String(
                        localized:
                            "The token can't create worklogs: give it the “Manage worklogs” permission (only needed for Send to Tempo).",
                        bundle: .module)))
        case .ok, .status:
            lines.append(
                Line(title: manage, ok: nil, detail: String(localized: "Couldn't tell from Tempo's answer.", bundle: .module)))
        case .unreachable(let message):
            lines.append(Line(title: manage, ok: nil, detail: message))
        }
        return lines
    }

    // MARK: HTTP

    enum Answer {
        case ok(Data)
        case status(Int)
        case unreachable(String)
    }

    var jiraAuth: String { "Basic " + Data("\(settings.jiraEmail):\(jiraToken)".utf8).base64EncodedString() }

    func get(_ url: String, auth: String) async -> Answer { await send(url, method: "GET", body: nil, auth: auth) }
    func post(_ url: String, body: String, auth: String) async -> Answer {
        await send(url, method: "POST", body: body, auth: auth)
    }

    func send(_ url: String, method: String, body: String?, auth: String) async -> Answer {
        guard let u = URL(string: url) else { return .unreachable(url) }
        var r = URLRequest(url: u)
        r.httpMethod = method
        r.setValue(auth, forHTTPHeaderField: "Authorization")
        r.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            r.setValue("application/json", forHTTPHeaderField: "Content-Type")
            r.httpBody = Data(body.utf8)
        }
        do {
            let (data, resp) = try await http.send(r)
            let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
            return (200..<300).contains(code) ? .ok(data) : .status(code)
        } catch {
            return .unreachable(error.localizedDescription)
        }
    }
}
