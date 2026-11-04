import Foundation

/// A chat history export (e.g. Google Chat), pasted as base64 of zlib-compressed JSON (PHP `gzcompress`) or as plain JSON.
struct ChatExport: Decodable {
    var source: String
    var conversations: [Conversation]

    struct Conversation: Decodable {
        var id: String
        var type: String
        var name: String?
        var messages: [Message]

        /// DMs carry the other person's name; spaces have none, but each message's `extra` starts with the space name.
        var displayName: String {
            if let name, !name.isEmpty { return name }
            if type == "space", let first = messages.first?.extra?.components(separatedBy: " | ").first,
               !first.isEmpty { return first }
            return id
        }
    }

    struct Message: Decodable {
        var time: Date
        var sender: String?   // null when the exporter couldn't tell who wrote it
        var me: Bool
        var text: String
        var extra: String?

        /// Attached file names: in `extra` ("Space | Projektterület | , | a.png | b.png") everything after the "," part.
        var files: [String] {
            let parts = (extra ?? "").components(separatedBy: " | ")
            guard let comma = parts.firstIndex(where: { $0.trimmingCharacters(in: .whitespaces) == "," }) else { return [] }
            return parts[(comma + 1)...].map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        }

        /// The text without the export's "Ön:" ("You:") prefix on own messages.
        var body: String {
            guard me, let colon = text.firstIndex(of: ":"), text[..<colon] == "Ön" else { return text }
            return text[text.index(after: colon)...].trimmingCharacters(in: .whitespaces)
        }
    }

    enum ParseError: LocalizedError {
        case notRecognized
        var errorDescription: String? { "Paste the base64 text of the chat export, or its JSON." }
    }

    static func parse(_ pasted: String) throws -> ChatExport {
        let text = pasted.trimmingCharacters(in: .whitespacesAndNewlines)
        let json: Data
        if text.hasPrefix("{") {
            json = Data(text.utf8)
        } else {
            guard let raw = Data(base64Encoded: text, options: .ignoreUnknownCharacters), raw.count > 2
            else { throw ParseError.notRecognized }
            // Foundation's .zlib is raw DEFLATE, so skip the 2-byte zlib header.
            json = try (raw.dropFirst(raw.first == 0x78 ? 2 : 0) as NSData).decompressed(using: .zlib) as Data
        }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { d in
            let s = try d.singleValueContainer().decode(String.self)
            guard let date = (try? Date(s, strategy: .iso8601.year().month().day().time(includingFractionalSeconds: true)))
                    ?? (try? Date(s, strategy: .iso8601))
            else { throw DecodingError.dataCorrupted(.init(codingPath: d.codingPath, debugDescription: "Bad date \(s)")) }
            return date
        }
        return try decoder.decode(ChatExport.self, from: json)
    }
}

struct ChatLine: Hashable {
    var time: Date
    var sender: String
    var me: Bool
    var text: String
    var files: [String]
}

/// One conversation's messages on one day, as listed on the Chats page.
struct ChatDay: Identifiable, Hashable {
    var conversationId: String
    var name: String
    var type: String
    var day: String
    var times: [Date]   // sorted

    var id: String { conversationId + "|" + day }
    var count: Int { times.count }
    /// Worklog `sourceRef` of this conversation-day once it is logged to an issue.
    var ref: String { "chat:" + id }
    /// Shorter chats are not worth a worklog.
    static let minLoggable = 15

    /// Messages less than `gap` apart form one burst; a burst lasts from its first to its last message, at least a minute.
    // ponytail: fixed 10-minute gap, make it a setting if it misjudges real chats.
    static let gap: TimeInterval = 10 * 60
    var activeSeconds: Int {
        guard var start = times.first, var last = times.first else { return 0 }
        var total: TimeInterval = 0
        for t in times.dropFirst() {
            if t.timeIntervalSince(last) > Self.gap {
                total += max(last.timeIntervalSince(start), 60)
                start = t
            }
            last = t
        }
        total += max(last.timeIntervalSince(start), 60)
        return Int(total)
    }
}
