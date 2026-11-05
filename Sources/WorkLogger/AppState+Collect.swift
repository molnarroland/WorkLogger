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

/// Collect: reading git and calendars for the range, and rebuilding drafts.
extension AppState {
    // MARK: Collect

    /// Reads git history and the calendar for the selected range, then rebuilds the drafts.
    func collect() async {
        guard !isBusy else { return }
        isBusy = true
        defer { isBusy = false }

        let useGit = settings.modules.contains(.git)
        var authors = settings.authorEmails
        if useGit, authors.isEmpty, let email = services.git.globalUserEmail() {
            authors = [email]
            settings.authorEmails = [email]
        }
        guard !useGit || !authors.isEmpty else {
            status = "Add your commit email address under Repositories so only your commits are counted."
            return
        }

        let enabled = useGit ? repos.filter(\.enabled) : []
        let start = rangeStart, end = rangeEnd
        let ignored = settings.ignoredPathPatterns
        status = "Reading git history of \(enabled.count) repositories…"
        let (results, errors) = await GitHistory.read(enabled, since: start, until: end, authors: authors, git: services.git)

        var commitCount = 0
        attempt("Saving commits") {
            try db.transaction {
                for (repoId, commits) in results {
                    commitCount += commits.count
                    try db.saveCommits(commits.map { StoredCommit($0, repoId: repoId, ignoredPaths: ignored) })
                }
            }
        }

        var meetingCount = 0
        if canReadMeetings {
            status = "Reading meetings from your calendars…"
            do { meetingCount = try await fetchMeetings() } catch { status = "Calendar: \(error.localizedDescription)" }
        }

        rebuild()
        var parts: [String] = []
        if useGit { parts.append("\(commitCount) commits") }
        if canReadMeetings { parts.append("\(meetingCount) meetings") }
        var summary = parts.isEmpty ? "Nothing to collect" : "Collected " + parts.joined(separator: " and ")
        summary += " for \(fromDay) – \(toDay)."
        if !errors.isEmpty { summary += " Skipped: " + errors.joined(separator: "; ") }
        status = summary
    }

    /// Rebuilds unlocked drafts from stored commits, meetings and sessions (no network or git access).
    func rebuild() {
        saveDirty()
        attempt("Building worklogs") {
            try DraftRebuilder(db: db, builder: makeBuilder()).rebuild(range)
            status = "Drafts rebuilt for \(fromDay) – \(toDay)."
        }
        reloadRange()
        applyChatRules()
    }
}
