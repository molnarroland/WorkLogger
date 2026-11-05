import Foundation
import Testing
import WorkLoggerTestSupport

@testable import WorkLoggerCore

/// Week used throughout: Mon 2026-03-02 … Fri 2026-03-06, Sat 03-07, Sun 03-08.
@Suite struct WorklogAllocationTests {
    let g = 900  // 15 minutes

    @Test func noWeightsGiveNoAllocations() {
        #expect(WorklogBuilder.allocate([], target: 28800, granularity: g).isEmpty)
    }

    @Test func withoutTargetEachEstimateIsRoundedToAtLeastOneUnit() {
        #expect(WorklogBuilder.allocate([100, 1000, 2000], target: nil, granularity: g) == [900, 900, 1800])
    }

    @Test func splitsProportionally() {
        #expect(WorklogBuilder.allocate([1, 1, 2], target: 28800, granularity: g) == [7200, 7200, 14400])
    }

    @Test(arguments: [[1, 1, 1], [5, 3, 2], [700, 1, 1, 1], [1234, 4321]])
    func alwaysSumsToTheTarget(weights: [Int]) {
        let result = WorklogBuilder.allocate(weights, target: 3600, granularity: g)
        #expect(result.reduce(0, +) == 3600)
        #expect(result.allSatisfy { $0 % g == 0 })
    }

    @Test func everyBlockGetsAUnitWhenThereIsRoom() {
        #expect(WorklogBuilder.allocate([1, 1000], target: 3600, granularity: g) == [900, 2700])
    }

    @Test(arguments: [0, 600])
    func targetBelowOneUnitGivesZeros(target: Int) {
        #expect(WorklogBuilder.allocate([10, 20], target: target, granularity: g) == [0, 0])
    }
}

@Suite struct WorklogPlacementTests {
    init() { Fixture.pinTimeZone() }
    let day = "2026-03-03"
    var free: [Interval] { [Fixture.interval(day, "09:00", "12:00"), Fixture.interval(day, "13:00", "17:00")] }

    func place(_ seconds: Int, from hm: String, in free: [Interval]? = nil) -> (String, String) {
        let (start, next) = WorklogBuilder.place(duration: seconds, from: Fixture.at(day, hm), in: free ?? self.free)
        return (DayKey.hm(start), DayKey.hm(next))
    }

    @Test func fitsInsideTheFirstSlot() {
        #expect(place(7200, from: "09:00") == ("09:00", "11:00"))
    }

    @Test func spillsOverTheLunchBreak() {
        #expect(place(7200, from: "11:00") == ("11:00", "14:00"))
    }

    @Test func startsAtTheNextFreeSlot() {
        #expect(place(3600, from: "12:30") == ("13:00", "14:00"))
    }

    @Test func overflowsPastTheLastWindow() {
        #expect(place(7200, from: "16:00") == ("16:00", "18:00"))
    }

    @Test func withoutFreeTimeStartsAtTheCursor() {
        #expect(place(3600, from: "09:00", in: []) == ("09:00", "10:00"))
    }
}

@Suite struct WorklogDayRulesTests {
    init() { Fixture.pinTimeZone() }

    func record(_ day: String, _ status: DayStatus, worked: Int? = nil) -> DayRecord {
        DayRecord(day: day, status: status, label: "", reportedWorked: worked, reportedBreaks: nil)
    }

    // MARK: isWorkday

    @Test func calendarDecidesWithoutHRData() {
        let b = Fixture.builder()
        #expect(b.isWorkday("2026-03-02", info: nil))
        #expect(!b.isWorkday("2026-03-07", info: nil))
    }

    @Test func hrStatusOverridesTheCalendar() {
        let b = Fixture.builder()
        #expect(b.isWorkday("2026-03-07", info: record("2026-03-07", .workday)))  // transferred Saturday
        #expect(!b.isWorkday("2026-03-02", info: record("2026-03-02", .publicHoliday)))
        #expect(b.isWorkday("2026-03-02", info: record("2026-03-02", .other)))  // unknown → calendar
    }

    // MARK: dailyTarget

    @Test func estimateModeHasNoTarget() {
        var s = AppSettings()
        s.roundingMode = .estimate
        #expect(Fixture.builder(s).dailyTarget(day: "2026-03-02", info: nil, windows: [], fromSessions: false) == nil)
    }

    @Test func sessionsAreRoundedUp() {
        let windows = [Fixture.interval("2026-03-02", "08:00", "15:22")]  // 7h 22m
        let target = Fixture.builder().dailyTarget(day: "2026-03-02", info: nil, windows: windows, fromSessions: true)
        #expect(target == 7 * 3600 + 30 * 60)
    }

    @Test func reportedHoursWinOverSessions() {
        let windows = [Fixture.interval("2026-03-02", "08:00", "16:00")]
        let info = record("2026-03-02", .workday, worked: 7 * 3600 + 5 * 60)
        let target = Fixture.builder().dailyTarget(day: "2026-03-02", info: info, windows: windows, fromSessions: true)
        #expect(target == 7 * 3600 + 15 * 60)
    }

    @Test func workdaysUseTheFixedHours() {
        #expect(Fixture.builder().dailyTarget(day: "2026-03-02", info: nil, windows: [], fromSessions: false) == 8 * 3600)
    }

    @Test func weekendsOnlyWhenIncluded() {
        var s = AppSettings()
        #expect(Fixture.builder(s).dailyTarget(day: "2026-03-07", info: nil, windows: [], fromSessions: false) == nil)
        s.includeWeekends = true
        #expect(Fixture.builder(s).dailyTarget(day: "2026-03-07", info: nil, windows: [], fromSessions: false) == 8 * 3600)
        let leave = record("2026-03-07", .leave)
        #expect(Fixture.builder(s).dailyTarget(day: "2026-03-07", info: leave, windows: [], fromSessions: false) == nil)
    }

    // MARK: commitDay

    @Test func workdayKeepsItsCommits() {
        #expect(Fixture.builder().commitDay(for: "2026-03-03", info: { _ in nil }) == "2026-03-03")
    }

    @Test func saturdayGoesToFridayAndSundayToMonday() {
        let b = Fixture.builder()
        #expect(b.commitDay(for: "2026-03-07", info: { _ in nil }) == "2026-03-06")
        #expect(b.commitDay(for: "2026-03-08", info: { _ in nil }) == "2026-03-09")
    }

    @Test func workedRestDayKeepsItsCommits() {
        #expect(
            Fixture.builder().commitDay(for: "2026-03-07", info: { _ in nil }, worked: { $0 == "2026-03-07" }) == "2026-03-07")
    }

    @Test func includedWeekendsKeepTheirCommits() {
        var s = AppSettings()
        s.includeWeekends = true
        #expect(Fixture.builder(s).commitDay(for: "2026-03-07", info: { _ in nil }) == "2026-03-07")
    }

    @Test func leaveGoesToTheClosestWorkdayEarlierFirst() {
        let records = ["2026-03-04": record("2026-03-04", .leave), "2026-03-02": record("2026-03-02", .leave)]
        let b = Fixture.builder()
        #expect(b.commitDay(for: "2026-03-04", info: { records[$0] }) == "2026-03-03")
        // Monday leave: Sunday before is not a workday, so Tuesday wins.
        #expect(b.commitDay(for: "2026-03-02", info: { records[$0] }) == "2026-03-03")
    }

    // MARK: work windows

    @Test func sessionsMinusBreaks() {
        let day = "2026-03-03"
        let input = DayInput(
            day: day,
            sessions: [
                Fixture.session(day: day, "08:00", "16:30"),
                Fixture.session(day: day, "12:00", "12:30", kind: .pause),
            ],
            commits: [], meetings: [], kept: [])
        let (windows, fromSessions) = Fixture.builder().workWindows(input)
        #expect(fromSessions)
        #expect(windows == [Fixture.interval(day, "08:00", "12:00"), Fixture.interval(day, "12:30", "16:30")])
    }

    @Test func defaultWorkdayWithoutSessions() {
        let day = "2026-03-03"
        let (windows, fromSessions) = Fixture.builder().workWindows(
            DayInput(day: day, sessions: [], commits: [], meetings: [], kept: []))
        #expect(!fromSessions)
        #expect(windows == [Fixture.interval(day, "09:00", "17:30")])
    }

    @Test func invalidDefaultWorkdayGivesNoWindows() {
        var s = AppSettings()
        s.defaultDayEnd = "08:00"
        let (windows, _) = Fixture.builder(s).workWindows(
            DayInput(day: "2026-03-03", sessions: [], commits: [], meetings: [], kept: []))
        #expect(windows.isEmpty)
    }
}

@Suite struct WorklogIssueResolutionTests {
    init() { Fixture.pinTimeZone() }

    @Test func commitMessageWins() {
        let b = Fixture.builder(repos: [Fixture.repo(issueKey: "TAO-1")])
        #expect(b.issue(for: Fixture.commit(day: "2026-03-03", "10:00", subject: "TAO-5 Fix totals")) == "TAO-5")
    }

    @Test func branchNameIsSecond() {
        let b = Fixture.builder(repos: [Fixture.repo(issueKey: "TAO-1")])
        let c = Fixture.commit(day: "2026-03-03", "10:00", subject: "Fix totals", ref: "refs/heads/feature/tao-7-login")
        #expect(b.issue(for: c) == "TAO-7")
    }

    @Test func repositoryIssueThenProjectDefault() {
        let c = Fixture.commit(day: "2026-03-03", "10:00", subject: "Fix totals")
        #expect(Fixture.builder(repos: [Fixture.repo(issueKey: "TAO-1")]).issue(for: c) == "TAO-1")
        let b = Fixture.builder(
            repos: [Fixture.repo(projectKey: "TAO")],
            projects: [ProjectMapping(key: "TAO", defaultIssue: "TAO-99")])
        #expect(b.issue(for: c) == "TAO-99")
        #expect(Fixture.builder().issue(for: c) == nil)
    }

    @Test func knownProjectPrefixBeatsEarlierUnknownKey() {
        let b = Fixture.builder(repos: [Fixture.repo(projectKey: "TAO")])
        let c = Fixture.commit(day: "2026-03-03", "10:00", subject: "FOO-1 and TAO-2")
        #expect(b.issue(for: c) == "TAO-2")
    }

    @Test func meetingIssues() {
        var s = AppSettings()
        s.meetingRules = [MeetingRule(contains: "standup", issueKey: "int-12")]
        s.defaultMeetingIssue = "int-1"
        let b = Fixture.builder(s)
        #expect(b.issue(for: Fixture.meeting(day: "2026-03-03", "09:00", "10:00", title: "TAO-3 planning")) == "TAO-3")
        #expect(b.issue(for: Fixture.meeting(day: "2026-03-03", "09:00", "10:00", title: "Daily Standup")) == "INT-12")
        #expect(b.issue(for: Fixture.meeting(day: "2026-03-03", "09:00", "10:00", title: "1:1")) == "INT-1")
    }

    @Test func skippedMeetings() {
        var s = AppSettings()
        s.skipMeetingKeywords = ["lunch", ""]
        let b = Fixture.builder(s)
        #expect(b.isSkipped(Fixture.meeting(day: "2026-03-03", "12:00", "13:00", title: "Team Lunch")))
        #expect(b.isSkipped(Fixture.meeting(day: "2026-03-03", "09:00", "10:00", skipped: true)))
        #expect(!b.isSkipped(Fixture.meeting(day: "2026-03-03", "09:00", "10:00", title: "Planning")))
    }

    @Test func absenceIssue() {
        var s = AppSettings()
        s.absenceRules = [MeetingRule(contains: "beteg", issueKey: "hr-3")]
        s.absenceIssueKey = "hr-1"
        let b = Fixture.builder(s)
        let sick = DayRecord(day: "2026-03-03", status: .leave, label: "Betegszabadság", reportedWorked: nil, reportedBreaks: nil)
        let leave = DayRecord(day: "2026-03-03", status: .leave, label: "Szabadság", reportedWorked: nil, reportedBreaks: nil)
        #expect(b.absenceIssue(for: sick) == "HR-3")
        #expect(b.absenceIssue(for: leave) == "HR-1")
    }

    @Test func latestPeriodWinsAndPeriodsWithoutIssueAreIgnored() {
        var s = AppSettings()
        s.periods = [
            ProjectPeriod(title: "Old", issueKey: "TAO-1", from: "2026-03-01", to: "2026-03-31"),
            ProjectPeriod(title: "New", issueKey: "TAO-2", from: "2026-03-02", to: "2026-03-06"),
            ProjectPeriod(title: "Empty", issueKey: " ", from: "2026-03-01", to: "2026-03-31"),
        ]
        let b = Fixture.builder(s)
        #expect(b.period(for: "2026-03-03")?.title == "New")
        #expect(b.period(for: "2026-03-10")?.title == "Old")
        #expect(b.period(for: "2026-04-01") == nil)
    }
}

@Suite struct WorklogBuildTests {
    init() { Fixture.pinTimeZone() }
    let day = "2026-03-03"  // Tuesday

    func input(
        sessions: [WorkSession] = [], commits: [StoredCommit] = [], meetings: [Meeting] = [],
        kept: [Worklog] = [], info: DayRecord? = nil, on day: String? = nil
    ) -> DayInput {
        DayInput(day: day ?? self.day, sessions: sessions, commits: commits, meetings: meetings, kept: kept, info: info)
    }

    @Test func emptyWorkdayHasNoEntries() {
        #expect(Fixture.builder().build(input()).isEmpty)
    }

    @Test func leaveDayIsOneAbsenceEntry() throws {
        var s = AppSettings()
        s.absenceIssueKey = "HR-1"
        let info = DayRecord(day: day, status: .leave, label: "Szabadság", reportedWorked: nil, reportedBreaks: nil)
        let logs = Fixture.builder(s).build(input(commits: [Fixture.commit(day: day, "10:00")], info: info))
        let entry = try #require(logs.only)
        #expect(entry.source == .absence)
        #expect(entry.issueKey == "HR-1")
        #expect(entry.duration == 8 * 3600)
        #expect(entry.description == "Szabadság")
        #expect(entry.sourceRef == "absence:\(day)")
    }

    @Test func leaveDayWithoutAbsenceIssueIsEmpty() {
        let info = DayRecord(day: day, status: .leave, label: "Szabadság", reportedWorked: nil, reportedBreaks: nil)
        #expect(Fixture.builder().build(input(info: info)).isEmpty)
    }

    @Test func meetingsKeepTheirSlotAndCommitsFillTheRestOfTheDay() throws {
        var s = AppSettings()
        s.meetingRules = [MeetingRule(contains: "standup", issueKey: "INT-12")]
        let logs = Fixture.builder(s).build(
            input(
                commits: [
                    Fixture.commit(day: day, "11:00", subject: "TAO-5 Invoices"),
                    Fixture.commit(day: day, "15:00", subject: "TAO-6: Reports"),
                ],
                meetings: [Fixture.meeting(day: day, "09:00", "09:15", title: "Daily standup")]))

        #expect(logs.totalDuration == 8 * 3600)
        let meeting = try #require(logs.first)
        #expect(meeting.source == .meeting)
        #expect(meeting.issueKey == "INT-12")
        #expect(DayKey.hm(meeting.start) == "09:00")
        #expect(meeting.duration == 900)

        let git = logs.filter { $0.source == .git }
        #expect(git.map(\.issueKey) == ["TAO-5", "TAO-6"])
        #expect(git.map(\.description) == ["Invoices", "Reports"])
        #expect(DayKey.hm(git[0].start) == "09:15")  // right after the meeting
        #expect(git[1].start == git[0].end)
        #expect(logs == logs.sorted { $0.start < $1.start })
    }

    @Test func commitsOfTheSameIssueAreGrouped() throws {
        let logs = Fixture.builder().build(
            input(commits: [
                Fixture.commit("a", day: day, "10:00", subject: "TAO-5 First"),
                Fixture.commit("b", day: day, "11:00", subject: "TAO-5 Second"),
                Fixture.commit("c", day: day, "12:00", subject: "TAO-5 First"),
            ]))
        let entry = try #require(logs.only)
        #expect(entry.refs == ["a", "b", "c"])
        #expect(entry.description == "First; Second")
        #expect(entry.duration == 8 * 3600)
    }

    @Test func keptEntriesOwnTheirCommitsAndReduceTheTarget() {
        let kept = [
            Fixture.worklog(day: day, "09:00", duration: 3600, source: .manual, locked: true),
            Fixture.worklog(day: day, "10:00", duration: 1800, ref: "claimed", locked: true),
        ]
        let logs = Fixture.builder().build(
            input(
                commits: [
                    Fixture.commit("claimed", day: day, "10:20", subject: "TAO-1 Old"),
                    Fixture.commit("new", day: day, "14:00", subject: "TAO-2 New"),
                ],
                kept: kept))
        #expect(logs.map(\.sourceRef) == ["new"])
        #expect(logs.totalDuration == 8 * 3600 - 3600 - 1800)
        #expect(!logs.contains { log in kept.contains { Interval(start: $0.start, end: $0.end).overlaps(log) } })
    }

    @Test func skippedMeetingsAreNotLogged() {
        let logs = Fixture.builder().build(input(meetings: [Fixture.meeting(day: day, "10:00", "11:00", skipped: true)]))
        #expect(!logs.contains { $0.source == .meeting })
    }

    @Test func commitsWithoutIssueAreGroupedPerRepository() throws {
        let b = Fixture.builder(repos: [Fixture.repo(name: "billing")])
        let entry = try #require(b.build(input(commits: [Fixture.commit(day: day, "10:00", subject: "Tidy up")])).only)
        #expect(entry.issueKey == nil)
        #expect(entry.description == "[billing] Tidy up")
    }

    @Test func longDescriptionsAreTruncated() throws {
        let subject = "TAO-1 " + String(repeating: "a", count: 300)
        let entry = try #require(
            Fixture.builder().build(input(commits: [Fixture.commit(day: day, "10:00", subject: subject)])).only)
        #expect(entry.description.count == 253)
        #expect(entry.description.hasSuffix("…"))
    }

    @Test func estimateModeKeepsRoundedEstimates() throws {
        var s = AppSettings()
        s.roundingMode = .estimate
        // First commit of the day: the gap from 09:00 is 60 minutes → 45 minute estimate.
        let entry = try #require(
            Fixture.builder(s).build(
                input(commits: [
                    Fixture.commit(day: day, "10:00", subject: "TAO-1 Add feature")
                ])
            ).only)
        #expect(entry.estimated == 45 * 60)
        #expect(entry.duration == 45 * 60)
    }

    @Test func sessionsDriveTheTarget() {
        let sessions = [Fixture.session(day: day, "08:00", "15:22")]
        let logs = Fixture.builder().build(
            input(sessions: sessions, commits: [Fixture.commit(day: day, "10:00", subject: "TAO-1 x")]))
        #expect(logs.totalDuration == 7 * 3600 + 30 * 60)
        #expect(logs.first.map { DayKey.hm($0.start) } == "08:00")
    }
}

extension Array {
    /// The single element, or nil when there are zero or several.
    fileprivate var only: Element? { count == 1 ? first : nil }
}

extension Interval {
    fileprivate func overlaps(_ w: Worklog) -> Bool { w.start < end && start < w.end }
}
