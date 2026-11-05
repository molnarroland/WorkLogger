import Foundation
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

extension AppState {
    /// An AppState backed by a throwaway SQLite file with a few sample rows, for SwiftUI previews.
    @MainActor
    static var preview: AppState {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("WorkLogger-preview-\(UUID().uuidString).sqlite")
        let state = AppState(databaseURL: url)
        let db = state.db
        let today = DayKey.key(Date())
        func at(_ hm: String) -> Date { DayKey.at(today, hm: hm) ?? Date() }

        try? db.addRepo(path: "~/Workspaces/billing-api", name: "billing-api")
        try? db.addRepo(path: "~/Workspaces/web-frontend", name: "web-frontend")
        try? db.upsertProject(ProjectMapping(key: "TAO", defaultIssue: "TAO-1"))
        try? db.importTimesheet(ParsedTimesheet(
            sessions: [
                WorkSession(id: nil, day: today, start: at("08:00"), end: at("16:30"), kind: .work, label: "Munkaidő 1"),
                WorkSession(id: nil, day: today, start: at("12:00"), end: at("12:30"), kind: .pause, label: "Ebéd"),
            ],
            days: [DayRecord(day: today, status: .workday, label: "Munkanap (Home Office)", reportedWorked: 8 * 3600, reportedBreaks: 1800)],
            format: "HRmaster"))
        let samples: [(String, Int, String?, String, WorklogSource, WorklogStatus)] = [
            ("09:00", 900, "INT-12", "Daily standup", .meeting, .reviewed),
            ("09:15", 9900, "TAO-123", "Fix invoice rounding; add tests", .git, .draft),
            ("12:30", 7200, "TAO-131", "Refactor payment client", .git, .draft),
            ("14:30", 3600, nil, "Code review", .manual, .draft),
        ]
        for (hm, duration, issue, text, source, status) in samples {
            _ = try? db.insertWorklog(Worklog(id: 0, day: today, start: at(hm), duration: duration, issueKey: issue,
                                              description: text, source: source, sourceRef: "", repoId: nil,
                                              estimated: duration, status: status, locked: false, tempoId: nil))
        }
        // One chat conversation, and one ticket already ticked off as entered in Tempo.
        _ = state.importChats("""
        {"source":"google-chat","conversations":[{"id":"space/TAO","type":"space","name":"TAO","messages":[
          {"time":"\(ISO8601DateFormatter().string(from: at("10:02")))","sender":"me@x","me":true,"text":"Deploying the fix to test"},
          {"time":"\(ISO8601DateFormatter().string(from: at("10:05")))","sender":"bea@x","me":false,"text":"Thanks!","extra":"TAO | , | log.txt"}]}]}
        """)
        if let standup = state.worklogs.first(where: { $0.issueKey == "INT-12" }) {
            var w = standup
            w.status = .exported
            try? db.updateWorklog(w)
        }
        state.settings.welcomed = true
        state.reloadAll()
        return state
    }

    /// A fresh install: no repositories, sessions or worklogs. Shows each screen's empty state.
    @MainActor
    static var empty: AppState {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("WorkLogger-empty-\(UUID().uuidString).sqlite")
        let state = AppState(databaseURL: url)
        state.settings.welcomed = true
        return state
    }
}
