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
                try TempoExport.json(logs, accountId: settings.defaultJiraAccount?.accountId),
                name: "tempo-\(fromDay)_\(toDay).json", type: .json
            )
        }
    }

    func markdown(_ logs: [Worklog]) -> String {
        TempoExport.markdown(logs, from: fromDay, to: toDay, jiraBaseURL: settings.defaultJiraAccount?.site ?? "")
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

    /// Creates the worklogs, one request per entry, each in the account that owns its project (Tempo or Jira), and
    /// marks the successful ones as exported.
    func pushToTempo(_ logs: [Worklog]) async {
        isBusy = true
        defer { isBusy = false }
        saveDirty()
        let ready = TempoExport.exportable(logs)
        guard !settings.jiraAccounts.isEmpty || ready.isEmpty else {
            status = TempoError.missing(String(localized: "a Jira account")).localizedDescription
            return
        }
        var sent = 0
        var failures: [String] = []
        let groups = Dictionary(grouping: ready) { settings.jiraAccount(for: $0.issueKey ?? "")?.id }
        for account in settings.jiraAccounts {
            guard let group = groups[account.id] else { continue }
            guard account.provider != .other else {
                failures.append(String(localized: "\(account.title): logged in another tool, not sent"))
                continue
            }
            do {
                let client = try services.makeTempo(account)
                status = String(localized: "Reading your Jira account…")
                let me = try await accountId(of: account, client: client)
                let outcome = await TempoExporter(db: db, client: client).push(group, accountId: me) {
                    self.status = String(localized: "Sending \($0) of \($1) to Tempo…")
                }
                sent += outcome.sent
                failures += outcome.failures
            } catch {
                failures.append("\(account.title): \(error.localizedDescription)")
            }
        }
        reloadRange()
        await refreshTempo()
        status =
            String(localized: "Sent \(sent) worklogs to Tempo.")
            + (failures.isEmpty ? "" : " " + String(localized: "Failed: \(failures.joined(separator: " | "))"))
    }
}
