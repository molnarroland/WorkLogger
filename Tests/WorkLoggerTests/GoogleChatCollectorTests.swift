import Foundation
import Testing
@testable import WorkLogger

@Suite struct GoogleChatCollectorTests {
    @Test func rangeIsSearchedDayByDay() {
        let days = GoogleChatCollector.days(from: Fixture.at("2026-03-01", "00:00"), to: Fixture.at("2026-03-03", "00:00"))
        #expect(days.map { DayKey.key($0.start) } == ["2026-03-01", "2026-03-02", "2026-03-03"])
        #expect(DayKey.key(days[2].end) == "2026-03-04")
    }

    @Test func searchURLMatchesTheExtension() {
        let day = (start: Fixture.at("2026-03-01", "00:00"), end: Fixture.at("2026-03-02", "00:00"))
        let url = GoogleChatCollector.searchURL(account: "1", params: [("cfrom", "me@x.hu")], day: day)
        #expect(url.absoluteString == "https://chat.google.com/u/1/app/search?q=&cfrom=me@x.hu&cdatestart=2026-3-1&cdateend=2026-3-1&cdateoption=custom&cmembership=1")
    }

    @Test func exportGroupsConversationsAndFillsMyEmail() {
        typealias M = GoogleChatCollector.RawMessage
        let msgs = [M(id: "2", gid: "space/A", conv: nil, ts: 2000, oneOnOne: false, sender: "Bea", me: false, text: "yo", extra: "TAO | x | ,"),
                    M(id: "1", gid: "space/A", conv: nil, ts: 1000, oneOnOne: false, sender: "Ön", me: true, text: "hi", extra: "TAO | x | ,"),
                    M(id: "3", gid: "dm/B", conv: "Bea", ts: 1500, oneOnOne: true, sender: nil, me: true, text: "ping", extra: nil)]
        let export = GoogleChatCollector.export(msgs, email: "me@x.hu")
        #expect(export.conversations.map(\.id) == ["space/A", "dm/B"])
        #expect(export.conversations[0].messages.map(\.text) == ["hi", "yo"])
        #expect(export.conversations[0].messages[0].sender == "me@x.hu")
        #expect(export.conversations.map(\.displayName) == ["TAO", "Bea"])
    }
}
