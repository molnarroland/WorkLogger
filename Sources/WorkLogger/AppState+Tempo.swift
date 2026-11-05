import Foundation
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerTempo

/// Jira accounts and their worklogs read back: what is already logged (Tempo or Jira) for the selected dates, shown
/// next to WorkLogger's own entries.
extension AppState {
    /// Reads your worklogs for the range from every account that has a provider WorkLogger can read. Quietly skips
    /// accounts without their tokens; `tempoConnected` is true when at least one read worked.
    func refreshTempo() async {
        let range = self.range
        var logs: [TempoWorklog] = []
        var connected = false
        for account in settings.jiraAccounts where account.provider != .other {
            guard let client = try? services.makeTempo(account) else { continue }
            do {
                let me = try await accountId(of: account, client: client)
                if account.projectKeys.isEmpty { await refreshProjects(account.id, client: client) }
                logs += try await TempoReader(db: db, client: client)
                    .worklogs(accountId: me, fromDay: range.fromDay, toDay: range.toDay)
                connected = true
            } catch {
                status = String(localized: "\(account.title): \(error.localizedDescription)")
            }
        }
        guard range == self.range else { return }  // the range changed while reading; that read wins
        tempoLogs = logs
        tempoConnected = connected
        Log.tempo.info("Read \(logs.count) worklogs for \(range.fromDay, privacy: .public) – \(range.toDay, privacy: .public)")
    }

    /// Reads the account's project keys from Jira, so its issues are sent there.
    func refreshProjects(_ id: JiraAccount.ID, client: (any TempoService)? = nil) async {
        guard let account = settings.jiraAccounts.first(where: { $0.id == id }) else { return }
        do {
            let keys = try await (client ?? services.makeTempo(account)).projectKeys()
            update(id) { $0.projectKeys = keys.sorted() }
        } catch {
            status = String(localized: "\(account.title): \(error.localizedDescription)")
        }
    }

    /// Your Jira account id on the account's site, read once and kept.
    func accountId(of account: JiraAccount, client: any TempoService) async throws -> String {
        if !account.accountId.isEmpty { return account.accountId }
        let id = try await client.accountId()
        update(account.id) { $0.accountId = id }
        return id
    }

    func update(_ id: JiraAccount.ID, _ change: (inout JiraAccount) -> Void) {
        guard let i = settings.jiraAccounts.firstIndex(where: { $0.id == id }) else { return }
        change(&settings.jiraAccounts[i])
    }

    /// Before accounts, the tokens were stored once; they now belong to the account made from the old settings.
    func moveLegacyTokens() {
        let secrets = services.secrets
        guard let account = settings.defaultJiraAccount else { return }
        for (old, new) in [
            (SecretKey.legacyJiraToken, account.jiraTokenKey), (SecretKey.legacyTempoToken, account.tempoTokenKey),
        ] {
            guard let token = secrets.get(old) else { continue }
            if secrets.get(new) == nil { secrets.set(token, for: new) }
            secrets.set(nil, for: old)
        }
    }

    /// Time already in Tempo on a day.
    func tempoSeconds(day: String) -> Int { tempoLogs.filter { $0.day == day }.reduce(0) { $0 + $1.seconds } }

    /// The entry's ticket has its time for that day in Tempo (sent from here, ticked off, or found in Tempo).
    func isInTempo(_ w: Worklog) -> Bool {
        w.status == .exported || w.issueKey.map { tempoCovered.contains("\($0)|\(w.day)") } == true
    }
}
