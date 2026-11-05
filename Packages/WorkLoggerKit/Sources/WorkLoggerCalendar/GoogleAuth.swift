import Foundation
import Network
import CryptoKit
import AppKit
import WorkLoggerCore

public enum GoogleAuthError: LocalizedError {
    case notConfigured, notSignedIn, denied(String), badResponse(String), timedOut
    public var errorDescription: String? {
        switch self {
        case .notConfigured: return "Add the Google OAuth client ID and secret first."
        case .notSignedIn: return "Google Calendar is not connected."
        case .denied(let e): return "Google sign-in was cancelled (\(e))."
        case .badResponse(let e): return "Google returned an unexpected response: \(e)"
        case .timedOut: return "Google sign-in timed out. Try connecting again."
        }
    }
}

/// One-shot HTTP listener on 127.0.0.1 that receives the OAuth redirect (Google's recommended desktop flow).
public final class LoopbackReceiver: @unchecked Sendable {
    private let listener: NWListener
    private let queue = DispatchQueue(label: "worklogger.oauth.loopback")
    private var startContinuation: CheckedContinuation<UInt16, Error>?
    private var callback: CheckedContinuation<URLComponents, Error>?
    private var pending: Result<URLComponents, Error>?

    public init() throws {
        let params = NWParameters.tcp
        params.requiredLocalEndpoint = .hostPort(host: .ipv4(.loopback), port: .any)
        listener = try NWListener(using: params)
    }

    public func start() async throws -> UInt16 {
        try await withCheckedThrowingContinuation { cont in
            queue.async {
                self.startContinuation = cont
                self.listener.stateUpdateHandler = { [weak self] state in
                    guard let self else { return }
                    switch state {
                    case .ready:
                        self.startContinuation?.resume(returning: self.listener.port?.rawValue ?? 0)
                        self.startContinuation = nil
                    case .failed(let error):
                        self.startContinuation?.resume(throwing: error)
                        self.startContinuation = nil
                    default:
                        break
                    }
                }
                self.listener.newConnectionHandler = { [weak self] conn in self?.handle(conn) }
                self.listener.start(queue: self.queue)
            }
        }
    }

    public func waitForRedirect(timeout: TimeInterval = 300) async throws -> URLComponents {
        try await withCheckedThrowingContinuation { cont in
            queue.async {
                if let p = self.pending {
                    self.pending = nil
                    cont.resume(with: p)
                } else {
                    self.callback = cont
                    self.queue.asyncAfter(deadline: .now() + timeout) {
                        self.deliver(.failure(GoogleAuthError.timedOut))
                    }
                }
            }
        }
    }

    public func stop() { listener.cancel() }

    private func deliver(_ result: Result<URLComponents, Error>) {
        if let cb = callback {
            callback = nil
            cb.resume(with: result)
        } else if pending == nil {
            pending = result
        }
    }

    private func handle(_ conn: NWConnection) {
        conn.start(queue: queue)
        conn.receive(minimumIncompleteLength: 1, maximumLength: 16_384) { [weak self] data, _, _, _ in
            guard let self, let data, let request = String(data: data, encoding: .utf8),
                  let firstLine = request.components(separatedBy: "\r\n").first else { conn.cancel(); return }
            let parts = firstLine.split(separator: " ")
            guard parts.count >= 2,
                  let comps = URLComponents(string: "http://127.0.0.1" + String(parts[1])),
                  comps.queryItems?.contains(where: { $0.name == "code" || $0.name == "error" }) == true
            else {
                self.respond(conn, status: "404 Not Found", body: "")
                return
            }
            self.respond(conn, status: "200 OK", body: """
                <html><body style="font-family:-apple-system,sans-serif;padding:48px">
                <h2>WorkLogger received the Google response.</h2><p>You can close this tab and return to the app.</p>
                </body></html>
                """)
            self.deliver(.success(comps))
        }
    }

    private func respond(_ conn: NWConnection, status: String, body: String) {
        let response = "HTTP/1.1 \(status)\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: \(body.utf8.count)\r\nConnection: close\r\n\r\n\(body)"
        conn.send(content: Data(response.utf8), completion: .contentProcessed { _ in conn.cancel() })
    }
}

/// OAuth 2.0 for installed apps: PKCE + loopback redirect. The refresh token is stored in the Keychain.
public final class GoogleAuth {
    public static let scope = "https://www.googleapis.com/auth/calendar.events.readonly"

    public let clientId: String
    public let clientSecret: String
    public let secrets: any SecretStore
    public let http: any HTTPClient
    private var accessToken: String?
    private var expiry = Date.distantPast

    public init(clientId: String, clientSecret: String, secrets: any SecretStore, http: any HTTPClient) {
        self.clientId = clientId
        self.clientSecret = clientSecret
        self.secrets = secrets
        self.http = http
    }

    public func signIn() async throws {
        guard !clientId.isEmpty, !clientSecret.isEmpty else { throw GoogleAuthError.notConfigured }
        let receiver = try LoopbackReceiver()
        defer { receiver.stop() }
        let port = try await receiver.start()
        let redirect = "http://127.0.0.1:\(port)"
        let verifier = Self.randomURLSafe(64)
        let challenge = Self.base64url(Data(SHA256.hash(data: Data(verifier.utf8))))
        let state = Self.randomURLSafe(24)

        var c = URLComponents(string: "https://accounts.google.com/o/oauth2/v2/auth")!
        c.queryItems = [
            URLQueryItem(name: "client_id", value: clientId),
            URLQueryItem(name: "redirect_uri", value: redirect),
            URLQueryItem(name: "response_type", value: "code"),
            URLQueryItem(name: "scope", value: Self.scope),
            URLQueryItem(name: "code_challenge", value: challenge),
            URLQueryItem(name: "code_challenge_method", value: "S256"),
            URLQueryItem(name: "state", value: state),
            URLQueryItem(name: "access_type", value: "offline"),
            URLQueryItem(name: "prompt", value: "consent"),
        ]
        guard let authURL = c.url else { throw GoogleAuthError.notConfigured }
        await MainActor.run { _ = NSWorkspace.shared.open(authURL) }

        let result = try await receiver.waitForRedirect()
        let items = Dictionary((result.queryItems ?? []).map { ($0.name, $0.value ?? "") }, uniquingKeysWith: { a, _ in a })
        if let error = items["error"] { throw GoogleAuthError.denied(error) }
        guard items["state"] == state, let code = items["code"] else {
            throw GoogleAuthError.badResponse("state mismatch")
        }

        let token = try await tokenRequest([
            "code": code, "client_id": clientId, "client_secret": clientSecret,
            "redirect_uri": redirect, "grant_type": "authorization_code", "code_verifier": verifier,
        ])
        guard let refresh = token.refresh_token else { throw GoogleAuthError.badResponse("no refresh token") }
        secrets.set(refresh, for: SecretKey.googleRefresh)
        accessToken = token.access_token
        expiry = Date().addingTimeInterval(TimeInterval(token.expires_in - 60))
    }

    public func signOut() {
        secrets.set(nil, for: SecretKey.googleRefresh)
        accessToken = nil
    }

    public func validAccessToken() async throws -> String {
        if let t = accessToken, expiry > Date() { return t }
        guard let refresh = secrets.get(SecretKey.googleRefresh) else { throw GoogleAuthError.notSignedIn }
        let token = try await tokenRequest([
            "refresh_token": refresh, "client_id": clientId, "client_secret": clientSecret,
            "grant_type": "refresh_token",
        ])
        accessToken = token.access_token
        expiry = Date().addingTimeInterval(TimeInterval(token.expires_in - 60))
        return token.access_token
    }

    private struct TokenResponse: Decodable {
        let access_token: String
        let expires_in: Int
        let refresh_token: String?
    }

    private func tokenRequest(_ form: [String: String]) async throws -> TokenResponse {
        var req = URLRequest(url: URL(string: "https://oauth2.googleapis.com/token")!)
        req.httpMethod = "POST"
        req.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        req.httpBody = form.map { "\($0.key)=\(Self.formEncode($0.value))" }
            .joined(separator: "&").data(using: .utf8)
        let (data, resp) = try await http.send(req)
        guard (resp as? HTTPURLResponse)?.statusCode == 200 else {
            throw GoogleAuthError.badResponse(String(decoding: data, as: UTF8.self))
        }
        return try JSONDecoder().decode(TokenResponse.self, from: data)
    }

    private static func formEncode(_ s: String) -> String {
        var allowed = CharacterSet(charactersIn: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
        allowed.insert(charactersIn: "-._~")
        return s.addingPercentEncoding(withAllowedCharacters: allowed) ?? s
    }

    private static func randomURLSafe(_ n: Int) -> String {
        let chars = Array("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
        return String((0..<n).map { _ in chars.randomElement()! })
    }

    private static func base64url(_ data: Data) -> String {
        data.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

extension GoogleAuth: GoogleCalendarAccount {
    public func meetings(calendarId: String, from: Date, to: Date) async throws -> [Meeting] {
        try await GoogleCalendarClient(auth: self, http: http).meetings(calendarId: calendarId, from: from, to: to)
    }
}
