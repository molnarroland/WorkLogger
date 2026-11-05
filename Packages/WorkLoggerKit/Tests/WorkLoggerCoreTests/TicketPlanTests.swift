import Foundation
import Testing
@testable import WorkLoggerCore
import WorkLoggerTestSupport

@Suite struct TicketPlanTests {
    func log(_ id: Int64, _ day: String, _ hm: String, _ minutes: Int, _ key: String?, _ text: String,
             status: WorklogStatus = .draft) -> Worklog {
        Worklog(id: id, day: day, start: Fixture.at(day, hm), duration: minutes * 60, issueKey: key, description: text,
                source: .meeting, sourceRef: "", repoId: nil, estimated: 0, status: status, locked: false, tempoId: nil)
    }

    @Test func oneRowPerTicketDayWithNoIssueFirst() {
        let logs = [log(1, "2026-03-03", "09:00", 15, "FLEX-273", "Standup"),
                    log(2, "2026-03-03", "13:00", 30, "FLEX-273", "Planning"),
                    log(3, "2026-03-04", "09:00", 15, "FLEX-273", "Standup", status: .exported),
                    log(4, "2026-03-03", "10:00", 240, "TAO-1", "Coding"),
                    log(5, "2026-03-03", "15:00", 60, nil, "Meetup"),
                    log(6, "2026-03-03", "16:00", 0, "TAO-1", "Empty")]
        let groups = TicketPlan.groups(logs)
        #expect(groups.map(\.key) == [nil, "TAO-1", "FLEX-273"])
        let flex = groups[2]
        #expect(flex.days.map(\.day) == ["2026-03-03", "2026-03-04"])
        #expect(flex.days[0].seconds == 45 * 60)
        #expect(flex.days[0].description == "Standup; Planning")
        #expect(flex.days.map(\.entered) == [false, true])
        #expect(flex.enteredDays == 1 && !flex.isDone)
        #expect(TicketPlan.text(flex) == "FLEX-273\n2026-03-03  45m      Standup; Planning\n2026-03-04  15m      Standup")
    }
}
