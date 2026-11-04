import Foundation
import Testing
@testable import WorkLogger

@Suite struct WorklogSummaryTests {
    init() { Fixture.pinTimeZone() }
    let day = "2026-03-03"

    var logs: [Worklog] {
        [Fixture.worklog(day: day, "09:00", duration: 900, source: .meeting),
         Fixture.worklog(day: day, "09:15", duration: 7200, source: .git),
         Fixture.worklog(day: day, "11:15", duration: 1800, issue: nil, source: .git),
         Fixture.worklog(day: day, "11:45", duration: 0, issue: nil, source: .manual)]
    }

    @Test func totals() {
        #expect(logs.totalDuration == 9900)
        #expect([Worklog]().totalDuration == 0)
    }

    @Test func missingIssueIgnoresEmptyEntries() {
        #expect(logs.missingIssue.map(\.duration) == [1800])
    }

    @Test func totalsBySourceLargestFirstWithoutEmptySources() {
        #expect(logs.totalsBySource.map(\.source) == [.git, .meeting])
        #expect(logs.totalsBySource.map(\.seconds) == [9000, 900])
    }

    @Test func netWorkTakesBreaksOut() {
        let sessions = [Fixture.session(day: day, "08:00", "16:30"), Fixture.session(day: day, "12:00", "12:30", kind: .pause)]
        #expect(Intervals.total(sessions.netWork) == 8 * 3600)
        #expect(sessions.workIntervals.count == 1)
        #expect(sessions.pauseIntervals.count == 1)
    }
}

@Suite struct DayProgressTests {
    @Test(arguments: [
        (0, 0, true, false, DayProgress.absence),
        (0, 0, false, false, .free),
        (3600, 0, false, false, .extra),
        (28800, 28800, false, false, .complete),
        (30000, 28800, false, true, .complete),
        (0, 28800, false, true, .upcoming),
        (0, 28800, false, false, .empty),
        (3600, 28800, false, false, .partial),
    ])
    func progress(logged: Int, expected: Int, absence: Bool, future: Bool, result: DayProgress) {
        #expect(DayProgress(logged: logged, expected: expected, absence: absence, future: future) == result)
    }
}

@Suite struct WorklogDragTests {
    init() { Fixture.pinTimeZone() }

    @Test func payloadSkipsExportedEntriesAndRoundTrips() {
        var a = Fixture.worklog(day: "2026-03-03", "09:00", duration: 60)
        var b = Fixture.worklog(day: "2026-03-03", "10:00", duration: 60, status: .exported)
        var c = Fixture.worklog(day: "2026-03-03", "11:00", duration: 60)
        (a.id, b.id, c.id) = (1, 2, 3)
        let payload = WorklogDrag.payload([a, b, c])
        #expect(payload == "worklogs:1,3")
        #expect(WorklogDrag.ids(payload) == [1, 3])
    }

    @Test(arguments: ["1,2,3", "", "worklog:1", "worklogs:"])
    func unrelatedTextIsIgnored(text: String) {
        #expect(WorklogDrag.ids(text).isEmpty)
    }
}

@Suite struct ReportTests {
    init() { Fixture.pinTimeZone() }

    var logs: [Worklog] {
        [Fixture.worklog(day: "2026-03-02", "09:00", duration: 3600, issue: "TAO-1"),           // week 10
         Fixture.worklog(day: "2026-03-06", "09:00", duration: 7200, issue: "TAO-2", source: .meeting),
         Fixture.worklog(day: "2026-03-09", "09:00", duration: 1800, issue: nil),               // week 11
         Fixture.worklog(day: "2026-04-01", "09:00", duration: 900, issue: "TAO-1")]
    }

    @Test func groupsByIsoWeek() {
        let groups = Report.group(logs, by: .week)
        #expect(groups.map(\.id) == ["2026-W10", "2026-W11", "2026-W14"])
        #expect(groups.map(\.total) == [10800, 1800, 900])
        #expect(groups[0].byIssue.map(\.issue) == ["TAO-2", "TAO-1"])   // largest first
        #expect(groups[0].bySource.map(\.source) == ["Coding", "Meetings"])
        #expect(groups[1].byIssue.map(\.issue) == ["No issue"])
    }

    @Test func groupsByDayAndMonth() {
        #expect(Report.group(logs, by: .day).map(\.id) == ["2026-03-02", "2026-03-06", "2026-03-09", "2026-04-01"])
        let months = Report.group(logs, by: .month)
        #expect(months.map(\.id) == ["2026-03", "2026-04"])
        #expect(months.map(\.total) == [12600, 900])
    }

    @Test func noWorklogsNoGroups() {
        #expect(Report.group([], by: .week).isEmpty)
    }
}
