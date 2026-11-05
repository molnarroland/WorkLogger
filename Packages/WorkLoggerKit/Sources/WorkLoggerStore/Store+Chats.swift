import Foundation
import WorkLoggerCore

/// Imported chat histories.
public extension Database {

    /// Adds the export's conversations and messages; messages already imported are kept once (their text and files
    /// refreshed). Returns the number of new or changed messages.
    @discardableResult
    func importChats(_ export: ChatExport) throws -> Int {
        var added = 0
        try transaction {
            for c in export.conversations {
                try run("""
                INSERT INTO chat_conversations(id, source, type, name) VALUES(?, ?, ?, ?)
                ON CONFLICT(id) DO UPDATE SET type = excluded.type, name = excluded.name
                """, [.text(c.id), .text(export.source), .text(c.type), .text(c.displayName)])
                for m in c.messages {
                    try run("""
                    INSERT INTO chat_messages(conversation_id, ts_ms, sender, me, text, files) VALUES(?, ?, ?, ?, ?, ?)
                    ON CONFLICT(conversation_id, ts_ms, sender) DO UPDATE SET text = excluded.text, files = excluded.files
                    WHERE text != excluded.text OR files != excluded.files
                    """, [.text(c.id), .int(Int64((m.time.timeIntervalSince1970 * 1000).rounded())), .text(m.sender ?? ""),
                          .of(m.me), .text(m.body), .text(m.files.joined(separator: "\n"))])
                    added += try query("SELECT changes() AS n").first?.intValue("n") ?? 0
                }
            }
        }
        return added
    }

    /// Conversations with messages in [from, to), split per day.
    func chatDays(from: Date, to: Date, gap: TimeInterval = ChatDay.defaultGap) throws -> [ChatDay] {
        let rows = try query("""
        SELECT m.conversation_id, m.ts_ms, c.name, c.type FROM chat_messages m
        JOIN chat_conversations c ON c.id = m.conversation_id
        WHERE m.ts_ms >= ? AND m.ts_ms < ? ORDER BY m.ts_ms
        """, [.int(Int64(from.timeIntervalSince1970 * 1000)), .int(Int64(to.timeIntervalSince1970 * 1000))])
        var byKey: [String: ChatDay] = [:]
        for r in rows {
            let id = r.string("conversation_id") ?? ""
            let t = Date(timeIntervalSince1970: TimeInterval(r.int("ts_ms") ?? 0) / 1000)
            let day = DayKey.key(t)
            byKey[id + "|" + day, default: ChatDay(conversationId: id, name: r.string("name") ?? id,
                                                   type: r.string("type") ?? "", day: day, times: [], gap: gap)].times.append(t)
        }
        return byKey.values.sorted { ($0.day, $0.times[0]) < ($1.day, $1.times[0]) }
    }

    /// Messages of one conversation on one day, oldest first.
    func chatLines(conversationId: String, day: String) throws -> [ChatLine] {
        let (start, end) = DayKey.bounds(day)
        return try query("""
        SELECT ts_ms, sender, me, text, files FROM chat_messages
        WHERE conversation_id = ? AND ts_ms >= ? AND ts_ms < ? ORDER BY ts_ms
        """, [.text(conversationId), .int(Int64(start.timeIntervalSince1970 * 1000)), .int(Int64(end.timeIntervalSince1970 * 1000))])
        .map { ChatLine(time: Date(timeIntervalSince1970: TimeInterval($0.int("ts_ms") ?? 0) / 1000),
                        sender: $0.string("sender") ?? "", me: $0.bool("me"), text: $0.string("text") ?? "",
                        files: ($0.string("files") ?? "").split(separator: "\n").map(String.init)) }
    }

    func deleteChats(from: Date, to: Date) throws {
        try run("DELETE FROM chat_messages WHERE ts_ms >= ? AND ts_ms < ?",
                [.int(Int64(from.timeIntervalSince1970 * 1000)), .int(Int64(to.timeIntervalSince1970 * 1000))])
    }
}
