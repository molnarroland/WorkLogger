import Foundation
import Testing
import WorkLoggerCore
import WorkLoggerTestSupport

@testable import WorkLoggerWeb

/// The collectors' loops against a fake page: what they open, what they keep, how they report and fail.
@MainActor
@Suite struct CollectorFlowTests {
    init() { Fixture.pinTimeZone() }

    func msg(_ id: String, _ gid: String?, _ day: String, _ hm: String, me: Bool = true) -> String {
        let ts = Fixture.at(day, hm).timeIntervalSince1970 * 1000
        let g = gid.map { "\"\($0)\"" } ?? "null"
        return
            #"{"id":"\#(id)","gid":\#(g),"conv":"TAO","ts":\#(ts),"oneOnOne":false,"sender":"Bea","me":\#(me),"text":"t\#(id)"}"#
    }

    @Test func chatSearchesEachDayTwiceAndKeepsOnlyThatDaysMessages() async throws {
        let page = FakePage()
        let home = URL(string: "https://chat.google.com/")!
        page.redirect[home] = URL(string: "https://chat.google.com/u/1/app/home")!
        page.answer = { url, _ in
            let q = url?.query() ?? ""
            if q.contains("cdatestart=2026-3-3") && q.contains("cfrom") {  // my messages on day 1, one from day 2
                return "[\(self.msg("1", "space/A", "2026-03-03", "09:00")),\(self.msg("x", "space/A", "2026-03-04", "09:00"))]"
            }
            if q.contains("cdatestart=2026-3-3") {  // mentions on day 1: a duplicate and one more
                return
                    "[\(self.msg("1", "space/A", "2026-03-03", "09:00")),\(self.msg("2", "dm/B", "2026-03-03", "10:00", me: false))]"
            }
            return "[\(self.msg("3", nil, "2026-03-04", "11:00"))]"  // no conversation id: dropped
        }
        let export = try await GoogleChatCollector(session: page, email: "me@x.hu")
            .collect(from: Fixture.at("2026-03-03", "00:00"), to: Fixture.at("2026-03-04", "00:00")) { _ in }

        #expect(page.opened.count == 1 + 2 * 2)
        #expect(page.opened.dropFirst().allSatisfy { $0.path() == "/u/1/app/search" })
        #expect(export.conversations.map(\.id) == ["space/A", "dm/B"])
        #expect(export.conversations.flatMap(\.messages).map(\.text) == ["t1", "t2"])
        #expect(export.conversations[0].messages[0].sender == "me@x.hu")
    }

    @Test func chatProgressNamesTheDayTimeAndCounts() async throws {
        let page = FakePage()
        let ts = Fixture.at("2026-03-03", "14:32").timeIntervalSince1970 * 1000
        page.progressJSON = [#"{"n":3,"ts":\#(ts)}"#]
        var lines: [String] = []
        _ = try await GoogleChatCollector(session: page, email: "me@x.hu")
            .collect(from: Fixture.at("2026-03-03", "00:00"), to: Fixture.at("2026-03-03", "00:00")) { lines.append($0) }
        let line = try #require(lines.first { $0.contains("at 14:32") })
        #expect(
            line.contains("(1 of 1)") && line.contains("my messages") && line.contains("3 today") && line.contains("3 in range"))
    }

    @Test func chatNeedsSignInAndReportsAChangedPage() async {
        let page = FakePage()
        page.signedIn = false
        let collector = GoogleChatCollector(session: page, email: "me@x.hu")
        let day = Fixture.at("2026-03-03", "00:00")
        await #expect(throws: WebSessionError.self) { try await collector.collect(from: day, to: day) { _ in } }
        #expect(page.opened.isEmpty)

        page.signedIn = true
        page.answer = { _, _ in "not json" }
        do {
            _ = try await collector.collect(from: day, to: day) { _ in }
            Issue.record("expected an error")
        } catch WebSessionError.script(let message) {
            #expect(message.contains("page changed"))
        } catch { Issue.record("unexpected \(error)") }
    }

    @Test func hrmasterReadsEachMonthWithItsArguments() async throws {
        let page = FakePage()
        page.answer = { _, args in #"{"html":"<p>\#(args["year"]!)-\#(args["month"]!)</p>"}"# }
        let months = [HRmasterCollector.Month(year: 2026, month: 10), .init(year: 2026, month: 11)]
        let pages = try await HRmasterCollector(session: page, base: URL(string: "https://acme.hrmaster.hu")!)
            .collect(months) { _ in }
        #expect(page.opened.map(\.absoluteString) == ["https://acme.hrmaster.hu/Datacenter/#/employeeWorkingTime/own"])
        #expect(pages.map(\.1) == ["<p>2026-10</p>", "<p>2026-11</p>"])
    }

    @Test func hrmasterMapsPageErrors() async {
        let page = FakePage()
        let collector = HRmasterCollector(session: page, base: URL(string: "https://acme.hrmaster.hu")!)
        let october = [HRmasterCollector.Month(year: 2026, month: 10)]

        page.answer = { _, _ in #"{"error":"no-panel"}"# }
        do {
            _ = try await collector.collect(october) { _ in }
            Issue.record("expected an error")
        } catch WebSessionError.notSignedIn(let site) { #expect(site == "HRmaster") } catch {
            Issue.record("unexpected \(error)")
        }

        page.answer = { _, _ in #"{"error":"unreachable"}"# }
        do {
            _ = try await collector.collect(october) { _ in }
            Issue.record("expected an error")
        } catch WebSessionError.script(let message) { #expect(message == "HRmaster: could not reach 2026-10.") } catch {
            Issue.record("unexpected \(error)")
        }
    }
}
