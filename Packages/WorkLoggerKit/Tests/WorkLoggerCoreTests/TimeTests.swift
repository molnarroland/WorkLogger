import Foundation
import Testing
@testable import WorkLoggerCore
import WorkLoggerTestSupport

@Suite struct DayKeyTests {
    init() { Fixture.pinTimeZone() }

    @Test func keysUseThePinnedTimeZone() throws {
        // 23:30 UTC is already the next day in Budapest (UTC+1 in winter).
        let instant = try #require(ISO8601DateFormatter().date(from: "2026-03-02T23:30:00Z"))
        #expect(DayKey.key(instant) == "2026-03-03")
        #expect(DayKey.timeZone == Fixture.timeZone)
    }

    @Test func parseAndKeyRoundTrip() {
        #expect(DayKey.key(Fixture.at("2026-03-03", "23:59")) == "2026-03-03")
        #expect(DayKey.parse("2026-03-03").map(DayKey.key) == "2026-03-03")
        #expect(DayKey.parse("not a day") == nil)
    }

    @Test func daysAreInclusiveAndOrderInsensitive() {
        let days = DayKey.days(from: Fixture.at("2026-03-30", "10:00"), to: Fixture.at("2026-03-28", "08:00"))
        #expect(days == ["2026-03-28", "2026-03-29", "2026-03-30"])
    }

    @Test func boundsFollowDaylightSaving() {
        let (start, end) = DayKey.bounds("2026-03-29")   // clocks go forward: a 23-hour day
        #expect(end.timeIntervalSince(start) == 23 * 3600)
        let (s2, e2) = DayKey.bounds("2026-03-30")
        #expect(e2.timeIntervalSince(s2) == 24 * 3600)
    }

    @Test(arguments: [("9:05", "09:05"), ("17:30", "17:30"), (" 8 : 00 ", "08:00")])
    func atParsesHoursAndMinutes(input: String, expected: String) throws {
        let date = try #require(DayKey.at("2026-03-03", hm: input))
        #expect(DayKey.hm(date) == expected)
        #expect(DayKey.key(date) == "2026-03-03")
    }

    @Test(arguments: ["25:00", "10:60", "abc", "10", ""])
    func atRejectsInvalidTimes(input: String) {
        #expect(DayKey.at("2026-03-03", hm: input) == nil)
    }

    @Test func weekends() {
        #expect(DayKey.isWeekend("2026-03-07"))    // Saturday
        #expect(DayKey.isWeekend("2026-03-08"))    // Sunday
        #expect(!DayKey.isWeekend("2026-03-06"))   // Friday
    }

    @Test func weeksStartOnMonday() {
        #expect(DayKey.calendar.firstWeekday == 2)
    }
}

@Suite struct IntervalsTests {
    init() { Fixture.pinTimeZone() }
    let day = "2026-03-03"

    @Test func normalizeMergesOverlappingAndTouchingAndDropsEmpty() {
        let list = [Fixture.interval(day, "11:00", "12:00"), Fixture.interval(day, "09:00", "10:00"),
                    Fixture.interval(day, "09:30", "11:00"), Fixture.interval(day, "13:00", "13:00")]
        #expect(Intervals.normalize(list) == [Fixture.interval(day, "09:00", "12:00")])
    }

    @Test func subtractSplitsAroundCuts() {
        let result = Intervals.subtract([Fixture.interval(day, "09:00", "17:00")], [Fixture.interval(day, "12:00", "12:30")])
        #expect(result == [Fixture.interval(day, "09:00", "12:00"), Fixture.interval(day, "12:30", "17:00")])
    }

    @Test func subtractHandlesCoveringAndDisjointCuts() {
        let base = [Fixture.interval(day, "09:00", "10:00")]
        #expect(Intervals.subtract(base, [Fixture.interval(day, "08:00", "11:00")]).isEmpty)
        #expect(Intervals.subtract(base, [Fixture.interval(day, "10:00", "11:00")]) == base)
    }

    @Test func totalCountsOverlapOnce() {
        let list = [Fixture.interval(day, "09:00", "10:00"), Fixture.interval(day, "09:30", "10:30")]
        #expect(Intervals.total(list) == 5400)
    }

    @Test func overlapWithWindow() {
        let free = [Fixture.interval(day, "09:00", "12:00"), Fixture.interval(day, "13:00", "17:00")]
        #expect(Intervals.overlap(free, with: Fixture.interval(day, "11:00", "14:00")) == 7200)
    }

    @Test func negativeIntervalHasZeroDuration() {
        #expect(Fixture.interval(day, "10:00", "09:00").duration == 0)
    }
}

@Suite struct DurationFormatTests {
    @Test(arguments: [(0, "0m"), (89, "1m"), (1800, "30m"), (3600, "1h"), (5400, "1h 30m"), (8 * 3600, "8h")])
    func formats(seconds: Int, expected: String) {
        #expect(DurationFormat.string(seconds) == expected)
    }

    @Test func hours() {
        #expect(DurationFormat.hours(5400) == "1.50 h")
    }

    @Test(arguments: ["1h 30m", "1.5h", "1,5h", "90m", "1:30", "90", " 1H 30M "])
    func parsesNinetyMinutes(input: String) {
        #expect(DurationFormat.parse(input) == 5400)
    }

    @Test(arguments: ["", "   ", "abc", "1:xx", "1:2:3"])
    func rejectsGarbage(input: String) {
        #expect(DurationFormat.parse(input) == nil)
    }
}
