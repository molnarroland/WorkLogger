import Foundation

enum JiraKey {
    private static let regex = try! NSRegularExpression(
        pattern: #"(?<![A-Za-z0-9])([A-Z][A-Z0-9_]{1,9})-(\d+)(?![A-Za-z0-9])"#)

    /// Prefixes that look like issue keys but never are.
    private static let deny: Set<String> = ["UTF", "ISO", "SHA", "RFC", "HTTP", "TLS", "SSL", "PSR", "CVE",
                                            "ES", "RS", "HS", "GPT", "MP", "PHP", "IPV", "X11", "COVID"]

    /// Finds the first issue key. Keys whose project prefix is known win over unknown ones.
    static func find(in text: String, known: Set<String>) -> String? {
        let ns = text as NSString
        var fallback: String?
        for m in regex.matches(in: text, range: NSRange(location: 0, length: ns.length)) {
            let prefix = ns.substring(with: m.range(at: 1))
            let key = ns.substring(with: m.range)
            if known.contains(prefix) { return key }
            if fallback == nil && !deny.contains(prefix) { fallback = key }
        }
        return fallback
    }

    static func normalize(_ s: String?) -> String? {
        guard let t = s?.trimmingCharacters(in: .whitespacesAndNewlines).uppercased(), !t.isEmpty else { return nil }
        return t
    }

    static func prefix(of key: String) -> String? {
        key.split(separator: "-").first.map(String.init)
    }

    /// "ABC-12: Fix login" → "Fix login"
    static func stripLeadingKey(_ subject: String) -> String {
        subject.replacingOccurrences(of: #"^\s*\[?[A-Z][A-Z0-9_]{1,9}-\d+\]?\s*[:\-–]?\s*"#,
                                     with: "", options: .regularExpression)
    }
}
