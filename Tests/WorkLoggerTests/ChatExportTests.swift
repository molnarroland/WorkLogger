import Foundation
import Testing
@testable import WorkLogger

@Suite struct ChatExportTests {
    let json = """
    {"source":"google-chat","version":4,"conversations":[
      {"id":"space/A","type":"space","name":null,"oneOnOne":false,"messages":[
        {"time":"2026-03-03T08:00:00.000Z","sender":"me@x","me":true,"text":"Ön: hi","extra":"TAO | Project | , | a.png | b c.xlsx"},
        {"time":"2026-03-03T08:05:00.000Z","sender":null,"me":false,"text":"yo","extra":"TAO | Project | ,"},
        {"time":"2026-03-03T09:00:00.000Z","sender":"me@x","me":true,"text":"later"}]},
      {"id":"dm/B","type":"dm","name":"Bea","oneOnOne":true,"messages":[
        {"time":"2026-03-04T10:00:00Z","sender":"me@x","me":true,"text":"ping","extra":"Busy"}]}]}
    """

    /// What PHP's gzcompress + base64_encode produce: zlib header, raw DEFLATE, (adler32 trailer omitted).
    func packed() throws -> String {
        let deflated = try (Data(json.utf8) as NSData).compressed(using: .zlib) as Data
        return (Data([0x78, 0x9c]) + deflated).base64EncodedString()
    }

    @Test func parsesBase64ZlibAndPlainJSON() throws {
        let a = try ChatExport.parse(try packed())
        let b = try ChatExport.parse(json)
        #expect(a.conversations.map(\.displayName) == ["TAO", "Bea"])
        #expect(b.conversations.map { $0.messages.count } == [3, 1])
        let first = b.conversations[0].messages[0]
        #expect(first.body == "hi")
        #expect(first.files == ["a.png", "b c.xlsx"])
        #expect(b.conversations[0].messages[1].files.isEmpty)
        #expect(throws: (any Error).self) { try ChatExport.parse("not an export") }
    }

    @Test func activeTimeSplitsAtLongGaps() {
        let t0 = Date(timeIntervalSince1970: 0)
        let day = ChatDay(conversationId: "c", name: "c", type: "dm", day: "1970-01-01",
                          times: [t0, t0 + 300, t0 + 3600])   // 5 min burst, then a lone message 55 min later
        #expect(day.activeSeconds == 300 + 60)
    }

    @Test func importIsIdempotentAndSplitsPerDay() throws {
        let db = try Database.inMemory()
        let export = try ChatExport.parse(json)
        #expect(try db.importChats(export) == 4)
        #expect(try db.importChats(export) == 0)
        let from = Date(timeIntervalSince1970: 1_772_323_200)          // 2026-03-01
        let days = try db.chatDays(from: from, to: from + 31 * 86_400)
        #expect(days.map(\.name) == ["TAO", "Bea"])
        #expect(days.map(\.count) == [3, 1])
        let lines = try db.chatLines(conversationId: "space/A", day: days[0].day)
        #expect(lines.first?.files == ["a.png", "b c.xlsx"])
        #expect(lines.first?.text == "hi")
    }
}
