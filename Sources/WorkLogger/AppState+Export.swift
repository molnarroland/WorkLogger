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

/// Export: Tempo API, CSV, JSON and the Markdown checklist.
extension AppState {
    // MARK: Export

    func exportCSV(_ logs: [Worklog]) {
        save(TempoExport.csv(logs).data(using: .utf8) ?? Data(), name: "tempo-\(fromDay)_\(toDay).csv", type: .commaSeparatedText)
    }

    func exportJSON(_ logs: [Worklog]) {
        attempt("Exporting") {
            save(
                try TempoExport.json(logs, accountId: settings.jiraAccountId), name: "tempo-\(fromDay)_\(toDay).json", type: .json
            )
        }
    }

    func markdown(_ logs: [Worklog]) -> String {
        TempoExport.markdown(logs, from: fromDay, to: toDay, jiraBaseURL: settings.jiraBaseURL)
    }

    func exportMarkdown(_ logs: [Worklog]) {
        save(
            Data(markdown(logs).utf8), name: "tempo-\(fromDay)_\(toDay).md",
            type: UTType(filenameExtension: "md") ?? .plainText)
    }

    func copyMarkdown(_ logs: [Worklog]) {
        services.files.copyToPasteboard(markdown(logs))
        status = String(
            localized:
                "Copied the Tempo checklist for \(TempoExport.exportable(logs).count) worklogs. Paste it to Claude with Claude in Chrome."
        )
    }

    private func save(_ data: Data, name: String, type: UTType) {
        guard let url = services.files.saveLocation(suggestedName: name, type: type) else { return }
        attempt("Writing file") {
            try data.write(to: url, options: .atomic)
            status = String(localized: "Saved \(url.lastPathComponent).")
        }
    }

    func markExported(_ logs: [Worklog]) {
        let ids = Set(logs.map(\.id))
        for i in worklogs.indices where ids.contains(worklogs[i].id) {
            worklogs[i].status = .exported
            worklogs[i].locked = true
            dirty.insert(worklogs[i].id)
        }
        saveDirty()
    }

    /// Creates the worklogs in Tempo, one request per entry, and marks the successful ones as exported.
    func pushToTempo(_ logs: [Worklog]) async {
        isBusy = true
        defer { isBusy = false }
        saveDirty()
        do {
            let client = try services.makeTempo(settings)
            if settings.jiraAccountId.isEmpty {
                status = String(localized: "Reading your Jira account…")
                settings.jiraAccountId = try await client.accountId()
            }
            let outcome = await TempoExporter(db: db, client: client).push(logs, accountId: settings.jiraAccountId) {
                self.status = String(localized: "Sending \($0) of \($1) to Tempo…")
            }
            reloadRange()
            await refreshTempo()
            status =
                String(localized: "Sent \(outcome.sent) worklogs to Tempo.")
                + (outcome.failures.isEmpty
                    ? "" : " " + String(localized: "Failed: \(outcome.failures.joined(separator: " | "))"))
        } catch {
            status = error.localizedDescription
        }
    }
}
