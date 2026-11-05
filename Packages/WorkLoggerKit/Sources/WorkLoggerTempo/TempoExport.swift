import Foundation
import WorkLoggerCore
import WorkLoggerStore

/// The body Tempo REST API v4 expects for POST /4/worklogs (issueKey kept for readability; issueId is resolved on push).
public struct TempoWorklogPayload: Codable, Sendable {
    public var issueKey: String
    public var issueId: Int64?
    public var authorAccountId: String?
    public var startDate: String
    public var startTime: String
    public var timeSpentSeconds: Int
    public var description: String

    public init(issueKey: String, issueId: Int64?, authorAccountId: String?, startDate: String, startTime: String, timeSpentSeconds: Int, description: String) {
        self.issueKey = issueKey
        self.issueId = issueId
        self.authorAccountId = authorAccountId
        self.startDate = startDate
        self.startTime = startTime
        self.timeSpentSeconds = timeSpentSeconds
        self.description = description
    }
}

public enum TempoExport {
    public static func exportable(_ logs: [Worklog]) -> [Worklog] {
        logs.filter { $0.duration > 0 && $0.issueKey != nil }
    }

    /// CSV in the format of Tempo's worklog importer ("Issue Key, Date Started, Time Spent, Comment").
    public static func csv(_ logs: [Worklog]) -> String {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.timeZone = TimeZone(identifier: "UTC")
        f.dateFormat = "yyyy-MM-dd HH:mm:ss'Z'"
        var lines = ["Issue Key,Date Started,Time Spent,Comment"]
        for w in exportable(logs) {
            lines.append([w.issueKey!, f.string(from: w.start), DurationFormat.string(w.duration), quote(w.description)]
                .joined(separator: ","))
        }
        return lines.joined(separator: "\n") + "\n"
    }

    public static func payloads(_ logs: [Worklog], accountId: String?) -> [TempoWorklogPayload] {
        let date = DateFormatter()
        date.locale = Locale(identifier: "en_US_POSIX")
        date.timeZone = DayKey.timeZone
        date.dateFormat = "yyyy-MM-dd"
        let time = DateFormatter()
        time.locale = Locale(identifier: "en_US_POSIX")
        time.timeZone = DayKey.timeZone
        time.dateFormat = "HH:mm:ss"
        return exportable(logs).map {
            TempoWorklogPayload(issueKey: $0.issueKey!, issueId: nil,
                                authorAccountId: (accountId?.isEmpty ?? true) ? nil : accountId,
                                startDate: date.string(from: $0.start), startTime: time.string(from: $0.start),
                                timeSpentSeconds: $0.duration, description: $0.description)
        }
    }

    public static func json(_ logs: [Worklog], accountId: String?) throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        return try encoder.encode(payloads(logs, accountId: accountId))
    }

    /// A checklist Claude (with Claude in Chrome) can follow to enter the worklogs in Tempo by hand.
    public static func markdown(_ logs: [Worklog], from: String, to: String, jiraBaseURL: String) -> String {
        let ready = exportable(logs).sorted { $0.start < $1.start }
        let days = Dictionary(grouping: ready, by: \.day)
        let total = ready.totalDuration
        let site = jiraBaseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ "))
        let myWork = site.isEmpty ? "Jira → Apps → Tempo → My Work"
            : "\(site)/plugins/servlet/ac/io.tempo.jira/tempo-app#!/my-work/week?type=TIME&date=\(from)"

        var md = """
        # Tempo timesheet \(from) – \(to)

        \(ready.count) worklogs · \(DurationFormat.string(total)) in total · \(days.count) days

        ## Instructions for Claude

        You are filling in my Tempo timesheet in Jira Cloud with Claude in Chrome. I am already signed in.

        1. Open Tempo **My Work** in a new tab: \(myWork)
        2. Go through the days below in order. Before logging a day, look at the worklogs already shown for it in Tempo.
           Skip an entry that already exists there (same issue, same duration), so nothing is logged twice.
        3. For each remaining entry, click the day (or **Log Time**) and fill in the dialog:
           - **Issue**: type the issue key, then pick the matching issue from the list
           - **Date**: the day of the section
           - **Start time** and **Duration** exactly as in the table (duration format like `1h 30m`)
           - **Description**: the text from the table
           - Leave every other field (account, attributes, billable time) at its default, then click **Log**.
        4. When a day is done, check that its total in Tempo matches the **Day total** below.
        5. Never edit or delete worklogs that were already in Tempo, and do not submit the timesheet for approval.
        6. If an issue can't be found, you lack permission, or anything looks unexpected, skip that entry and keep going.
           Do not guess a different issue.
        7. At the end, reply with a table of every entry: logged, skipped (already there) or failed (with the reason).

        ## Overview

        | Day | Day total | Entries |
        | --- | --- | --- |

        """
        for day in days.keys.sorted() {
            let list = days[day]!
            md += "| \(day) | \(DurationFormat.string(list.totalDuration)) | \(list.count) |\n"
        }
        for day in days.keys.sorted() {
            let list = days[day]!
            md += """

            ## \(DayKey.title(day)) — \(day)

            Day total: **\(DurationFormat.string(list.totalDuration))**

            | # | Issue | Start | Duration | Description |
            | --- | --- | --- | --- | --- |

            """
            for (i, w) in list.enumerated() {
                let text = w.description.replacingOccurrences(of: "|", with: "\\|").replacingOccurrences(of: "\n", with: " ")
                md += "| \(i + 1) | \(w.issueKey!) | \(DayKey.hm(w.start)) | \(DurationFormat.string(w.duration)) | \(text) |\n"
            }
        }
        return md
    }

    private static func quote(_ s: String) -> String {
        "\"" + s.replacingOccurrences(of: "\"", with: "\"\"").replacingOccurrences(of: "\n", with: " ") + "\""
    }
}
