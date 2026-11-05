import EventKit
import Foundation
import SwiftUI
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

/// Work sessions: HRmaster fetching and imported sessions.
extension AppState {
    // MARK: Work sessions

    func importTimesheet(_ t: ParsedTimesheet) {
        attempt("Importing work sessions") { try db.importTimesheet(t) }
        status = String(
            localized:
                "Imported \(t.sessions.count) sessions and breaks for \(t.dayKeys.count) days. Rebuild drafts to apply them.")
        reloadRange()
    }

    /// Reads the HRmaster work-time calendar for each month of the range in the background, with progress in the
    /// status bar. Returns true when you need to sign in first.
    @discardableResult
    func fetchHRmaster() async -> Bool {
        guard !isBusy else { return false }
        guard let base = HRmasterCollector.normalize(settings.hrmasterURL) else {
            status = String(localized: "Enter your company's HRmaster address on the Work sessions page first.")
            return false
        }
        isBusy = true
        defer { isBusy = false }
        do {
            let pages = try await HRmasterCollector(session: WebSession.hrmaster, base: base)
                .collect(HRmasterCollector.months(from: rangeStart, to: rangeEnd)) { self.status = $0 }
            var days = 0
            attempt("Importing work sessions") {
                for (m, html) in pages {
                    let parsed = HRmasterParser(fallbackYear: m.year, fallbackMonth: m.month).parse(html)
                    try db.importTimesheet(parsed)
                    days += parsed.dayKeys.count
                }
                status = String(localized: "HRmaster: imported \(days) days. Rebuild drafts to apply them.")
            }
            reloadRange()
        } catch WebSessionError.notSignedIn {
            status = String(localized: "HRmaster is signed out. Sign in in the HRmaster window; fetching continues after.")
            signInNeeded = .hrmaster
            return true
        } catch {
            status = error.localizedDescription
        }
        return false
    }

    func clearSessionsInRange() {
        attempt("Clearing sessions") { try db.deleteSessions(fromDay: fromDay, toDay: toDay) }
        reloadRange()
    }
}
