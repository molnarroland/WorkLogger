import Foundation
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerTempo

/// Tempo read back: what is already logged there for the selected dates, shown next to WorkLogger's own entries.
extension AppState {
    /// Reads your Tempo worklogs for the range. Quietly does nothing without a Tempo token (`tempoConnected` false).
    func refreshTempo() async {
        guard let client = try? services.makeTempo(settings) else {
            tempoConnected = false
            tempoLogs = []
            return
        }
        let range = self.range
        do {
            if settings.jiraAccountId.isEmpty { settings.jiraAccountId = try await client.accountId() }
            let logs = try await TempoReader(db: db, client: client)
                .worklogs(accountId: settings.jiraAccountId, fromDay: range.fromDay, toDay: range.toDay)
            guard range == self.range else { return }  // the range changed while reading; that read wins
            tempoLogs = logs
            tempoConnected = true
            Log.tempo.info(
                "Read \(logs.count) Tempo worklogs for \(range.fromDay, privacy: .public) – \(range.toDay, privacy: .public)")
        } catch {
            tempoConnected = false
            status = String(localized: "Tempo: \(error.localizedDescription)")
        }
    }

    /// Time already in Tempo on a day.
    func tempoSeconds(day: String) -> Int { tempoLogs.filter { $0.day == day }.reduce(0) { $0 + $1.seconds } }

    /// The entry's ticket has its time for that day in Tempo (sent from here, ticked off, or found in Tempo).
    func isInTempo(_ w: Worklog) -> Bool {
        w.status == .exported || w.issueKey.map { tempoCovered.contains("\($0)|\(w.day)") } == true
    }
}
