import Foundation
import SwiftUI
import UniformTypeIdentifiers
import EventKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// Editing worklogs: saving, moving, bulk changes, ticking ticket-days off as entered in Tempo.
extension AppState {
    // MARK: Editing

    func markDirty(_ id: Int64) { dirty.insert(id) }

    func saveDirty() {
        guard !dirty.isEmpty else { return }
        attempt("Saving") {
            try db.transaction {
                for w in worklogs where dirty.contains(w.id) { try db.updateWorklog(w) }
            }
            status = "Saved \(dirty.count) changes."
        }
        dirty.removeAll()
    }

    func delete(_ w: Worklog) {
        attempt("Deleting") { try db.deleteWorklog(w.id) }
        worklogs.removeAll { $0.id == w.id }
        dirty.remove(w.id)
    }

    func addManual(day: String) {
        saveDirty()
        let lastEnd = worklogs.filter { $0.day == day }.map(\.end).max()
        let start = lastEnd ?? DayKey.at(day, hm: settings.defaultDayStart) ?? DayKey.date(day)
        let w = Worklog(id: 0, day: day, start: start, duration: 30 * 60, issueKey: nil, description: "",
                        source: .manual, sourceRef: "", repoId: nil, estimated: 0, status: .draft, locked: true, tempoId: nil)
        attempt("Adding") { try db.insertWorklog(w) }
        reloadRange()
    }

    /// Drag & drop on the timesheet: moves entries to another day, after its last entry, and locks them so a rebuild keeps them.
    func move(_ ids: Set<Int64>, to day: String) {
        saveDirty()
        var cursor = worklogs.filter { $0.day == day }.map(\.end).max()
            ?? DayKey.at(day, hm: settings.defaultDayStart) ?? DayKey.date(day)
        var moved = 0
        attempt("Moving") {
            try db.transaction {
                for var w in worklogs where ids.contains(w.id) && w.day != day && w.status != .exported {
                    w.day = day
                    w.start = cursor
                    w.locked = true
                    cursor = w.end
                    try db.updateWorklog(w)
                    moved += 1
                }
            }
            status = "Moved \(moved) \(moved == 1 ? "entry" : "entries") to \(DayKey.title(day))."
        }
        reloadRange()
    }

    /// A day can take dropped entries when it is a workday or has imported work sessions.
    func acceptsWork(_ day: String) -> Bool {
        isWorkday(day) || sessions.contains { $0.day == day && $0.kind == .work }
    }

    /// Ticking a ticket-day off after typing it into Tempo by hand: its worklogs count as in Tempo (exported
    /// without a Tempo id) and are locked so rebuilds keep them. Unticking puts them back to reviewed.
    func setEntered(_ day: TicketDay, _ entered: Bool) {
        saveDirty()
        attempt("Marking entered") {
            try db.transaction {
                for var w in worklogs where day.ids.contains(w.id) && (w.tempoId == nil || entered) {
                    w.status = entered ? .exported : .reviewed
                    w.locked = true
                    try db.updateWorklog(w)
                }
            }
        }
        reloadRange()
    }

    /// For the status bar: entries without a ticket, and ticket-days already in Tempo.
    var tempoProgress: (missing: Int, entered: Int, total: Int) {
        let groups = TicketPlan.groups(worklogs).filter { $0.key != nil }
        return (worklogs.missingIssue.count, groups.reduce(0) { $0 + $1.enteredDays }, groups.reduce(0) { $0 + $1.days.count })
    }

    /// Review's bulk bar: one ticket for several entries (they lock, like any edit). Entries already in Tempo stay.
    func setIssue(_ issue: String, for ids: Set<Int64>) {
        guard let key = JiraKey.normalize(issue) else { return }
        for i in worklogs.indices where ids.contains(worklogs[i].id) && worklogs[i].status != .exported {
            worklogs[i].issueKey = key
            worklogs[i].locked = true
            dirty.insert(worklogs[i].id)
        }
        saveDirty()
    }

    func setStatus(_ status: WorklogStatus, for ids: Set<Int64>) {
        for i in worklogs.indices where ids.contains(worklogs[i].id) && worklogs[i].status != .exported {
            worklogs[i].status = status
            worklogs[i].locked = true
            dirty.insert(worklogs[i].id)
        }
        saveDirty()
    }

    func setStatus(_ status: WorklogStatus, forAllIn day: String) {
        for i in worklogs.indices where worklogs[i].day == day && worklogs[i].status != .exported {
            worklogs[i].status = status
            worklogs[i].locked = true
            dirty.insert(worklogs[i].id)
        }
        saveDirty()
    }
}
