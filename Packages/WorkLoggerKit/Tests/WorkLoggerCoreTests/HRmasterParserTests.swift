import XCTest
@testable import WorkLoggerCore
import WorkLoggerTestSupport

/// Rows are generated with the exact markup of HRmaster's Munkaidő table (Datacenter → Calendar).
enum HRmasterFixture {
    static func cell(_ repeater: String, _ from: String = "", _ to: String = "") -> String {
        """
        <td class="col-1 min-width-100 text-center" data-e2e-testing="Datacenter.Employee.Calendar.\(repeater)"><div><div class="d-flex align-items-center">\
        <div class="flex-1" data-e2e-testing="Evolution.TimepickerSwitchInputContainer.From"><div tabindex="-1" class="timepicker disable-me disabled " data-e2e-testing="Evolution.TimepickerSwitchInput">\(from)</div></div>\
        <div class="flex-1" data-e2e-testing="Evolution.TimepickerSwitchInputContainer.To"><div tabindex="-1" class="timepicker disable-me disabled " data-e2e-testing="Evolution.TimepickerSwitchInput">\(to)</div></div></div></div></td>
        """
    }

    static func row(_ dd: String, _ type: String, _ name: String, _ label: String, _ worked: String, _ unpaid: String,
                    work: [(String, String)] = [], breaks: [(String, String)] = []) -> String {
        let w = work + Array(repeating: ("", ""), count: max(0, 3 - work.count))
        let b = breaks + Array(repeating: ("", ""), count: max(0, 2 - breaks.count))
        let cells = w.map { cell("WorkScheduleRepeater", $0.0, $0.1) }.joined()
            + b.map { cell("RestBreaksRepeater", $0.0, $0.1) }.joined()
        return """
        <tr class="calendar-closed-day " data-e2e-testing="Datacenter.Employee.Calendar.RowRepeater"><td class="col-0-5 min-width-200 table-sticky-column" style="position: relative;">\
        <div data-e2e-testing="Datacenter.Calendar.DayCell" class="
                            d-flex
                            Type_CalendarDayStatus_\(type)
                            null
                            calendar-date__container
                        "><div class="calendar-date__day" data-e2e-testing="Datacenter.Calendar.Day">\(dd).</div><div class="calendar-date__content">\
        <div class="calendar-date__day-name" data-e2e-testing="Datacenter.Calendar.Day.Name">\(name)</div><div class="calendar-date__status" data-e2e-testing="Datacenter.Calendar.Day.Status">\
        <div data-e2e-testing="Datacenter.Calendar.Day.Status.\(type)" id=":r43:" tabindex="0" aria-describedby=":r42:"><div class="LinesEllipsis  ">\(label)<wbr></div></div></div></div></div></td>\
        <td class="col-1 text-center" data-e2e-testing="Datacenter.Employee.Calendar.FactWorkedHoursSum">\(worked)</td>\
        <td class="col-1 text-center" data-e2e-testing="Datacenter.Employee.Calendar.UnpaidRestBreaksSum">\(unpaid)</td>\(cells)\
        <td class="col-0-5 min-width-100 text-center" data-e2e-testing="Datacenter.Calendar.Actual.PaidRestBreak.Column"><div class="d-flex align-items-center"><div class="flex-1">\
        <input tabindex="-1" type="text" class="form-control disable-me text-center disable-me" maxlength="7" disabled="" data-e2e-testing="Datacenter.Employee.Calendar.PaidRestBreakInput" value="0"></div></div></td></tr>
        """
    }

    static let march2026 = """
    <div class="cs-date-inner-toggle position-relative width-180" data-e2e-testing="Datacenter.Calendar.PeriodSelect">2026. március</div>\
    <div class="sd-element justify-content-center" data-e2e-testing="PeriodSelect.LeftRepeater.2027"><span>2027</span></div><span>április</span>\
    <table><thead><tr><th>Dátum</th><th>Munkaidő 1</th><th data-e2e-testing="Datacenter.Calendar.Actual.PaidRestBreak.Column">Fizetett szünet</th></tr></thead><tbody>
    """ + [
        row("01", "RestDay", "vasárnap", "Pihenőnap", "00:00", "00:00"),
        row("02", "Workday", "hétfő", "Munkanap (Home Office)", "07:22", "00:56", work: [("08:11", "16:29")], breaks: [("11:19", "12:15")]),
        row("04", "LabourCode", "szerda", "Szabadság", "08:00", "00:00"),
        row("09", "Workday", "hétfő", "Munkanap (Home Office)", "09:33", "00:26",
            work: [("08:16", "15:17"), ("20:06", "23:04")], breaks: [("12:50", "13:16")]),
        row("12", "Workday", "csütörtök", "Munkanap", "10:08", "00:00",
            work: [("08:02", "11:27"), ("13:07", "15:45"), ("19:11", "23:16")]),
        row("15", "PublicHoliday", "vasárnap", "Ünnepnap", "00:00", "00:00"),
        row("26", "Workday", "csütörtök", "Munkanap", "08:54", "04:45",
            work: [("07:58", "21:37")], breaks: [("11:34", "13:03"), ("14:54", "18:10")]),
    ].joined() + """
    </tbody><tfoot><tr><td class="working-time-table-summary-label" colspan="1">Havi összesen:</td><td class="text-center" data-e2e-testing="Datacenter.Employee.AllPlanWorkingHour">172:37</td></tr></tfoot></table>
    """
}

final class HRmasterParserTests: XCTestCase {
    private func parse(_ raw: String = HRmasterFixture.march2026) -> ParsedTimesheet {
        HRmasterParser(fallbackYear: 2026, fallbackMonth: 11).parse(raw)
    }

    func testDetectsFormatAndMonth() {
        let t = parse()
        XCTAssertEqual(t.format, "HRmaster")
        XCTAssertEqual(t.days.map(\.day), ["2026-03-01", "2026-03-02", "2026-03-04", "2026-03-09",
                                           "2026-03-12", "2026-03-15", "2026-03-26"])
        XCTAssertTrue(t.warnings.isEmpty, "\(t.warnings)")
    }

    func testDayStatusesAndLabels() {
        let days = Dictionary(uniqueKeysWithValues: parse().days.map { ($0.day, $0) })
        XCTAssertEqual(days["2026-03-01"]?.status, .restDay)
        XCTAssertEqual(days["2026-03-02"]?.status, .workday)
        XCTAssertEqual(days["2026-03-02"]?.label, "Munkanap (Home Office)")
        XCTAssertEqual(days["2026-03-04"]?.status, .leave)
        XCTAssertEqual(days["2026-03-04"]?.reportedWorked, 8 * 3600)
        XCTAssertEqual(days["2026-03-15"]?.status, .publicHoliday)
    }

    func testSessionsMatchHRmasterTotals() {
        let t = parse()
        for record in t.days {
            let s = t.sessions.filter { $0.day == record.day }
            let work = s.filter { $0.kind == .work }.map { Interval(start: $0.start, end: $0.end) }
            guard !work.isEmpty else { continue }
            let pauses = s.filter { $0.kind == .pause }.map { Interval(start: $0.start, end: $0.end) }
            XCTAssertEqual(Int(Intervals.total(Intervals.subtract(work, pauses))), record.reportedWorked, record.day)
        }
        XCTAssertEqual(t.sessions.filter { $0.day == "2026-03-12" && $0.kind == .work }.count, 3)
        XCTAssertEqual(t.sessions.filter { $0.day == "2026-03-26" && $0.kind == .pause }.count, 2)
        XCTAssertEqual(t.sessions.first.map { DayKey.hm($0.start) }, "08:11")
    }

    func testMissingHeaderFallsBackToRangeMonth() {
        let rowsOnly = HRmasterFixture.row("05", "Workday", "csütörtök", "Munkanap", "08:00", "00:00", work: [("08:00", "16:00")])
        let t = parse("<table>" + rowsOnly + "</table>")
        XCTAssertEqual(t.days.first?.day, "2026-11-05")
        XCTAssertEqual(t.warnings.count, 1)
    }

    func testTargetsFollowDayStatus() {
        var settings = AppSettings()
        settings.roundingMode = .fixedHours
        settings.absenceIssueKey = "HR-1"
        let builder = WorklogBuilder(settings: settings, repos: [:], projects: [:])
        let t = parse()
        let days = Dictionary(uniqueKeysWithValues: t.days.map { ($0.day, $0) })

        func input(_ day: String) -> DayInput {
            DayInput(day: day, sessions: t.sessions.filter { $0.day == day }, commits: [], meetings: [], kept: [], info: days[day])
        }
        func target(_ day: String) -> Int? {
            let (w, fromSessions) = builder.workWindows(input(day))
            return builder.dailyTarget(day: day, info: days[day], windows: w, fromSessions: fromSessions)
        }
        XCTAssertEqual(target("2026-03-02"), 7 * 3600 + 1800)   // worked 7h 22m → 7h 30m
        XCTAssertEqual(target("2026-03-09"), 9 * 3600 + 2700)   // worked 9h 33m → 9h 45m
        XCTAssertEqual(target("2026-03-12"), 10 * 3600 + 900)   // worked 10h 08m → 10h 15m
        XCTAssertEqual(target("2026-03-03"), 8 * 3600)   // no sessions: hours per day
        XCTAssertNil(target("2026-03-01"))
        XCTAssertNil(target("2026-03-04"))
        XCTAssertNil(target("2026-03-15"))

        let leave = builder.build(input("2026-03-04"))
        XCTAssertEqual(leave.count, 1)
        XCTAssertEqual(leave.first?.issueKey, "HR-1")
        XCTAssertEqual(leave.first?.duration, 8 * 3600)
        XCTAssertEqual(leave.first?.source, .absence)
        XCTAssertTrue(builder.build(input("2026-03-15")).isEmpty)
    }

    func testLeaveDayCommitsMoveToClosestWorkday() {
        var settings = AppSettings()
        settings.roundingMode = .fixedHours
        settings.absenceIssueKey = "HR-1"
        let builder = WorklogBuilder(settings: settings, repos: [:], projects: [:])
        let days = Dictionary(uniqueKeysWithValues: parse().days.map { ($0.day, $0) })
        let info: (String) -> DayRecord? = { days[$0] }

        // 03-04 (Wed) is Szabadság; 03-03 (Tue) has no record and is a weekday, so it wins over 03-05.
        XCTAssertEqual(builder.commitDay(for: "2026-03-04", info: info), "2026-03-03")
        XCTAssertEqual(builder.commitDay(for: "2026-03-02", info: info), "2026-03-02")

        // Weekend without work sessions: Saturday → Friday, Sunday → Monday; a worked Saturday keeps its commits.
        XCTAssertEqual(builder.commitDay(for: "2026-03-07", info: info), "2026-03-06")
        XCTAssertEqual(builder.commitDay(for: "2026-03-08", info: info), "2026-03-09")
        XCTAssertEqual(builder.commitDay(for: "2026-03-07", info: info, worked: { $0 == "2026-03-07" }), "2026-03-07")

        let commit = StoredCommit(repoId: 1, hash: "abc", date: DayKey.at("2026-03-04", hm: "10:00")!, author: "", email: "",
                                  subject: "TAO-1 fix", body: "", ref: "", filesChanged: 1, insertions: 10, deletions: 0)
        let leave = builder.build(DayInput(day: "2026-03-04", sessions: [], commits: [commit], meetings: [], kept: [], info: days["2026-03-04"]))
        XCTAssertEqual(leave.map(\.source), [.absence])
        XCTAssertEqual(leave.first?.duration, 8 * 3600)

        let moved = builder.build(DayInput(day: "2026-03-03", sessions: [], commits: [commit], meetings: [], kept: [], info: nil))
        XCTAssertEqual(moved.map(\.source), [.git])
        XCTAssertEqual(moved.first?.duration, 8 * 3600)
        XCTAssertEqual(moved.first?.day, "2026-03-03")
    }

    func testPeriodFillsWorkdaysWithItsIssue() {
        var settings = AppSettings()
        settings.roundingMode = .fixedHours
        settings.periods = [ProjectPeriod(title: "Migration", issueKey: "TAO-200", from: "2026-03-02", to: "2026-03-06")]
        let builder = WorklogBuilder(settings: settings, repos: [:], projects: [:])

        // No commits: the workday is filled with the period issue.
        let empty = builder.build(DayInput(day: "2026-03-03", sessions: [], commits: [], meetings: [], kept: []))
        XCTAssertEqual(empty.map(\.issueKey), ["TAO-200"])
        XCTAssertEqual(empty.first?.duration, 8 * 3600)
        XCTAssertEqual(empty.first?.description, "Migration")
        XCTAssertEqual(empty.first?.source, .period)

        // Commits with their own keys are booked to the period issue too.
        let commit = StoredCommit(repoId: 1, hash: "abc", date: DayKey.at("2026-03-03", hm: "10:00")!, author: "", email: "",
                                  subject: "TAO-1 fix rounding", body: "", ref: "", filesChanged: 1, insertions: 10, deletions: 0)
        let withCommit = builder.build(DayInput(day: "2026-03-03", sessions: [], commits: [commit], meetings: [], kept: []))
        XCTAssertEqual(withCommit.map(\.issueKey), ["TAO-200"])
        XCTAssertEqual(withCommit.first?.description, "Migration: fix rounding")

        // A locked fill entry is not duplicated; weekends and days outside the period stay empty.
        XCTAssertTrue(builder.build(DayInput(day: "2026-03-03", sessions: [], commits: [], meetings: [], kept: empty)).isEmpty)
        XCTAssertTrue(builder.build(DayInput(day: "2026-03-07", sessions: [], commits: [], meetings: [], kept: [])).isEmpty)
        XCTAssertTrue(builder.build(DayInput(day: "2026-03-09", sessions: [], commits: [], meetings: [], kept: [])).isEmpty)
    }
}
