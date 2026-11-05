import Foundation

/// A chat history export (e.g. Google Chat), pasted as base64 of zlib-compressed JSON (PHP `gzcompress`) or as plain JSON.
public struct ChatExport: Decodable, Sendable {
    public var source: String
    public var conversations: [Conversation]

    public struct Conversation: Decodable, Sendable {
        public var id: String
        public var type: String
        public var name: String?
        public var messages: [Message]

        /// DMs carry the other person's name; spaces have none, but each message's `extra` starts with the space name.
        public var displayName: String {
            if let name, !name.isEmpty { return name }
            if type == "space", let first = messages.first?.extra?.components(separatedBy: " | ").first,
               !first.isEmpty { return first }
            return id
        }

        public init(id: String, type: String, name: String?, messages: [Message]) {
            self.id = id
            self.type = type
            self.name = name
            self.messages = messages
        }
    }

    public struct Message: Decodable, Sendable {
        public var time: Date
        public var sender: String?   // null when the exporter couldn't tell who wrote it
        public var me: Bool
        public var text: String
        public var extra: String?

        /// Attached file names: in `extra` ("Space | Projektterület | , | a.png | b.png") everything after the "," part.
        public var files: [String] {
            let parts = (extra ?? "").components(separatedBy: " | ")
            guard let comma = parts.firstIndex(where: { $0.trimmingCharacters(in: .whitespaces) == "," }) else { return [] }
            return parts[(comma + 1)...].map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        }

        /// The text without the export's "Ön:" ("You:") prefix on own messages.
        public var body: String {
            guard me, let colon = text.firstIndex(of: ":"), text[..<colon] == "Ön" else { return text }
            return text[text.index(after: colon)...].trimmingCharacters(in: .whitespaces)
        }

        public init(time: Date, sender: String?, me: Bool, text: String, extra: String?) {
            self.time = time
            self.sender = sender
            self.me = me
            self.text = text
            self.extra = extra
        }
    }

    public enum ParseError: LocalizedError, Sendable {
        case notRecognized
        public var errorDescription: String? { String(localized: "Paste the base64 text of the chat export, or its JSON.", bundle: .module) }
    }

    public static func parse(_ pasted: String) throws -> ChatExport {
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

    public init(source: String, conversations: [Conversation]) {
        self.source = source
        self.conversations = conversations
    }
}

public struct ChatLine: Hashable, Sendable {
    public var time: Date
    public var sender: String
    public var me: Bool
    public var text: String
    public var files: [String]

    public init(time: Date, sender: String, me: Bool, text: String, files: [String]) {
        self.time = time
        self.sender = sender
        self.me = me
        self.text = text
        self.files = files
    }
}

/// One conversation's messages on one day, as listed on the Chats page.
public struct ChatDay: Identifiable, Hashable, Sendable {
    public var conversationId: String
    public var name: String
    public var type: String
    public var day: String
    public var times: [Date]   // sorted
    /// Messages further apart than this start a new stretch (Settings → Chats).
    public var gap: TimeInterval = ChatDay.defaultGap

    public var id: String { conversationId + "|" + day }
    public var count: Int { times.count }
    /// Worklog `sourceRef` of this conversation-day once it is logged to an issue.
    public var ref: String { "chat:" + id }
    /// Shorter chats are not worth a worklog.
    public static let minLoggable = 15

    /// Messages less than `gap` apart form one burst; a burst lasts from its first to its last message, at least a minute.
    public static let defaultGap: TimeInterval = 10 * 60
    public var activeSeconds: Int {
        guard var start = times.first, var last = times.first else { return 0 }
        var total: TimeInterval = 0
        for t in times.dropFirst() {
            if t.timeIntervalSince(last) > gap {
                total += max(last.timeIntervalSince(start), 60)
                start = t
            }
            last = t
        }
        total += max(last.timeIntervalSince(start), 60)
        return Int(total)
    }

    public init(conversationId: String, name: String, type: String, day: String, times: [Date],
                gap: TimeInterval = ChatDay.defaultGap) {
        self.gap = gap
        self.conversationId = conversationId
        self.name = name
        self.type = type
        self.day = day
        self.times = times
    }
}
