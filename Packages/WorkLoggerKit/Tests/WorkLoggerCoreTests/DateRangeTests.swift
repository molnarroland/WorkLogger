import Foundation
import Testing
@testable import WorkLoggerCore
import WorkLoggerTestSupport

@Suite struct DateRangeTests {
    init() { Fixture.pinTimeZone() }

    let wednesday = Fixture.at("2026-03-04", "15:00")

    func days(_ r: DateRange?) -> String? { r.map { "\($0.fromDay)…\($0.toDay)" } }

    @Test func orderInsensitive() {
        let r = DateRange(from: Fixture.at("2026-03-06", "10:00"), to: Fixture.at("2026-03-02", "08:00"))
        #expect(days(r) == "2026-03-02…2026-03-06")
        #expect(r.start == Fixture.at("2026-03-02", "00:00"))
        #expect(r.end == Fixture.at("2026-03-07", "00:00"))
        #expect(r.days.count == 5)
    }

    @Test func monthAcrossDaylightSaving() throws {
        let march = try #require(DateRange.month(containing: wednesday))
        #expect(days(march) == "2026-03-01…2026-03-31")
        #expect(march.days.count == 31)
        #expect(march.end == Fixture.at("2026-04-01", "00:00"))
    }

    @Test(arguments: [(QuickRange.today, "2026-03-04…2026-03-04"),
                      (.thisWeek, "2026-03-02…2026-03-08"),
                      (.lastWeek, "2026-02-23…2026-03-01"),
                      (.thisMonth, "2026-03-01…2026-03-31"),
                      (.lastMonth, "2026-02-01…2026-02-28")])
    func quickRanges(range: QuickRange, expected: String) {
        #expect(days(DateRange.quick(range, now: wednesday)) == expected)
    }

    @Test func isoWeekAcrossNewYear() {
        #expect(days(DateRange.quick(.thisWeek, now: Fixture.at("2026-01-01", "12:00"))) == "2025-12-29…2026-01-04")
    }

    @Test func shiftByMonths() {
        let march = DateRange.month(containing: wednesday)
        #expect(days(march?.shiftedByMonths(-1)) == "2026-02-01…2026-02-28")
        #expect(days(march?.shiftedByMonths(10)) == "2027-01-01…2027-01-31")
    }

    @Test func shiftingAWeekMovesToTheWholeNeighbouringMonth() {
        let week = DateRange.quick(.thisWeek, now: wednesday)
        #expect(days(week?.shiftedByMonths(1)) == "2026-04-01…2026-04-30")
    }
}
