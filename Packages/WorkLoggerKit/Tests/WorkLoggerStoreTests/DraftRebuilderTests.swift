import Foundation
import Testing
@testable import WorkLoggerStore
import WorkLoggerCore
import WorkLoggerTestSupport

/// Rebuilds against an in-memory database. The range is Mon 2026-03-02 … Fri 2026-03-06.
@Suite struct DraftRebuilderTests {
    let db: Database
    let repo: Repo
    let week: DateRange

    init() throws {
        Fixture.pinTimeZone()
        db = try Database.inMemory()
        try db.addRepo(path: "/tmp/billing", name: "billing")
        repo = try #require(try db.repos().first)
        week = DateRange(from: Fixture.at("2026-03-02", "00:00"), to: Fixture.at("2026-03-06", "23:59"))
    }

    func rebuild(_ settings: AppSettings = AppSettings()) throws -> [Worklog] {
        let builder = Fixture.builder(settings, repos: try db.repos(), projects: try db.projects())
        try DraftRebuilder(db: db, builder: builder).rebuild(week)
        return try db.worklogs(fromDay: "2026-03-01", toDay: "2026-03-31")
    }

    func commit(_ hash: String, _ day: String, _ hm: String = "10:00", subject: String = "TAO-1 Fix") throws {
        try db.saveCommits([Fixture.commit(hash, day: day, hm, subject: subject, repoId: repo.id)])
    }

    @Test func commitsBecomeADayOfDrafts() throws {
        try commit("a", "2026-03-03")
        let logs = try rebuild()
        #expect(logs.count == 1)
        #expect(logs.first?.day == "2026-03-03")
        #expect(logs.first?.issueKey == "TAO-1")
        #expect(logs.first?.duration == 8 * 3600)
        #expect(logs.first?.refs == ["a"])
    }

    @Test func rebuildingTwiceGivesTheSameDrafts() throws {
        try commit("a", "2026-03-03")
        try commit("b", "2026-03-05", subject: "TAO-2 Other")
        let first = try rebuild().map { [$0.day, $0.issueKey ?? "", $0.sourceRef] }
        let second = try rebuild().map { [$0.day, $0.issueKey ?? "", $0.sourceRef] }
        #expect(first == second)
        #expect(first.count == 2)
    }

    @Test func saturdayCommitJustOutsideTheRangeLandsOnFriday() throws {
        try commit("sat", "2026-03-07")
        let logs = try rebuild()
        #expect(logs.map(\.day) == ["2026-03-06"])
        #expect(logs.first?.refs == ["sat"])
    }

    @Test func lockedEntriesSurviveAndKeepTheirCommits() throws {
        try commit("a", "2026-03-03")
        var entry = try #require(try rebuild().first)
        entry.locked = true
        entry.description = "Edited by hand"
        try db.updateWorklog(entry)

        let logs = try rebuild()
        #expect(logs == [entry])
    }

    @Test func leaveDayGetsAbsenceAndItsCommitsMoveToTheDayBefore() throws {
        var settings = AppSettings()
        settings.absenceIssueKey = "HR-1"
        try db.importTimesheet(ParsedTimesheet(days: [
            DayRecord(day: "2026-03-04", status: .leave, label: "Szabadság", reportedWorked: nil, reportedBreaks: nil)]))
        try commit("a", "2026-03-04")

        let logs = try rebuild(settings)
        #expect(logs.map(\.day) == ["2026-03-03", "2026-03-04"])
        #expect(logs.map(\.source) == [.git, .absence])
        #expect(logs.last?.issueKey == "HR-1")
    }

    @Test func disabledRepositoriesAreIgnored() throws {
        try commit("a", "2026-03-03")
        var r = repo
        r.enabled = false
        try db.updateRepo(r)
        #expect(try rebuild().isEmpty)
    }

    @Test func entriesOutsideTheRangeAreLeftAlone() throws {
        let outside = Fixture.worklog(day: "2026-03-10", "09:00", duration: 3600, source: .git)
        try db.insertWorklog(outside)
        #expect(try rebuild().map(\.day) == ["2026-03-10"])
    }
}
