import Foundation
import Testing
@testable import WorkLoggerCore
import WorkLoggerTestSupport

@Suite struct TimeloggerParserTests {
    init() { Fixture.pinTimeZone() }

    let parser = TimeloggerParser(breakKeywords: ["lunch", "ebéd"], shortDateMonthFirst: true, defaultYear: 2026)

    func times(_ s: [WorkSession]) -> [String] {
        s.map { "\($0.day) \(DayKey.hm($0.start))-\(DayKey.hm($0.end)) \($0.kind.rawValue)" }
    }

    @Test func plainTextWithBreaks() {
        let raw = """
        2026-03-03 08:00 - 12:00
        Lunch 12:00-12:30
        12:30–16:30
        """
        #expect(times(parser.parse(raw)) == ["2026-03-03 08:00-12:00 work",
                                             "2026-03-03 12:00-12:30 pause",
                                             "2026-03-03 12:30-16:30 work"])
    }

    @Test func htmlTableWithSeparateTimeCells() {
        let raw = "<table><tr><td>2026.03.04.</td><td>08:15</td><td>16:45</td></tr>"
            + "<tr><td>Ebéd</td><td>12:00</td><td>12:20</td></tr></table>"
        #expect(times(parser.parse(raw)) == ["2026-03-04 08:15-16:45 work", "2026-03-04 12:00-12:20 pause"])
    }

    @Test func pastMidnightEndsTheNextDay() throws {
        let session = try #require(parser.parse("2026-03-03 22:00 - 01:00").first)
        #expect(session.day == "2026-03-03")
        #expect(session.duration == 3 * 3600)
    }

    @Test func rowsBeforeTheFirstDateAndDuplicatesAreIgnored() {
        let raw = """
        Header 07:00 - 08:00
        2026-03-03
        09:00 - 10:00
        09:00 - 10:00
        """
        #expect(times(parser.parse(raw)) == ["2026-03-03 09:00-10:00 work"])
    }

    @Test(arguments: [("2026-11-03", "2026-11-03"), ("2026. 11. 03.", "2026-11-03"), ("03.11.2026", "2026-11-03"),
                      ("11/03/2026", "2026-11-03"), ("03.11.", "2026-03-11")])
    func dateFormats(text: String, expected: String) {
        #expect(parser.findDate(in: text, contextYear: 2026)?.0 == expected)
    }

    @Test func shortDatesCanBeDayFirst() {
        let dayFirst = TimeloggerParser(breakKeywords: [], shortDateMonthFirst: false, defaultYear: 2026)
        #expect(dayFirst.findDate(in: "03.11.", contextYear: 2026)?.0 == "2026-11-03")
    }

    @Test func invalidDatesAreRejected() {
        #expect(parser.findDate(in: "2026-13-01", contextYear: 2026) == nil)
        #expect(parser.findDate(in: "no date", contextYear: 2026) == nil)
    }

    @Test func loneTimesAreNotRanges() {
        #expect(TimeloggerParser.findRanges(in: "Total 07:30").isEmpty)
        #expect(TimeloggerParser.findRanges(in: "08:00 12:00 13:00 17:00").count == 2)
    }

    @Test func decodesEntities() {
        #expect(TimeloggerParser.decodeEntities("a&nbsp;&amp;&#39;&#x41;&lt;") == "a &'A<")
    }

    @Test func genericFormatIsDetected() {
        let t = parser.parseTimesheet("2026-03-03 08:00 - 16:00", fallbackMonth: 3)
        #expect(t.format == "Generic")
        #expect(t.sessions.count == 1)
        #expect(t.days.isEmpty)
    }
}
