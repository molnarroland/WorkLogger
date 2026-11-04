import Foundation

enum RoundingMode: String, Codable, CaseIterable, Identifiable {
    /// `sessions` behaves like `fixedHours` and is kept only so older saved settings still decode.
    case estimate, fixedHours, sessions
    var id: String { rawValue }
    static let choices: [RoundingMode] = [.fixedHours, .estimate]
    var label: String {
        switch self {
        case .estimate: return "Keep commit estimates"
        case .fixedHours, .sessions: return "Scale each day to its worked time"
        }
    }
}

struct MeetingRule: Codable, Hashable, Identifiable {
    var id = UUID()
    var contains: String
    var issueKey: String
}

/// A stretch of days booked to one issue, e.g. a two-week migration project.
struct ProjectPeriod: Codable, Hashable, Identifiable {
    var id = UUID()
    var title: String
    var issueKey: String
    var from: String   // day keys, inclusive
    var to: String

    func contains(_ day: String) -> Bool { from <= day && day <= to }
}

/// Optional data sources, switched on the welcome screen. Off means the source is neither read nor used for drafts.
enum Module: String, Codable, CaseIterable, Identifiable {
    case git, calendar, workTimes
    var id: String { rawValue }
    var title: String {
        switch self {
        case .git: return "Git repositories"
        case .calendar: return "Calendar meetings"
        case .workTimes: return "Work times"
        }
    }
    var icon: String {
        switch self {
        case .git: return "externaldrive.connected.to.line.below"
        case .calendar: return "person.2"
        case .workTimes: return "clock.badge.checkmark"
        }
    }
    var summary: String {
        switch self {
        case .git: return "Turns your commits into worklogs. Leave off if you rarely code."
        case .calendar: return "Logs meetings from any calendar synced to macOS: Google, Outlook / Exchange, iCloud, CalDAV."
        case .workTimes: return "Imports clock-in times, leave and holidays from HRmaster or another work-time logger."
        }
    }
}

struct AppSettings: Codable, Equatable {
    // Modules and first-run welcome
    var modules: Set<Module> = Set(Module.allCases)
    var welcomed = false

    // Git
    var workspaceRoot: String = ""
    var authorEmails: [String] = []
    var excludedFolders: [String] = []   // folder names or paths skipped when scanning
    var ignoredPathPatterns: [String] = [
        "package-lock.json", "composer.lock", "yarn.lock", "pnpm-lock.yaml", "Podfile.lock",
        ".min.js", ".min.css", "/dist/", "/vendor/", "node_modules/", ".snap", ".svg"
    ]

    // Estimation
    var minCommitMinutes: Int = 10
    var maxCommitMinutes: Int = 240
    var maxCommitGapMinutes: Int = 180

    // Daily totals
    var roundingMode: RoundingMode = .fixedHours
    var fixedDailyHours: Double = 8
    var granularityMinutes: Int = 15
    var dayRoundUpMinutes: Int = 15        // worked time is rounded up: 7h 22m → 7h 30m
    var defaultDayStart: String = "09:00"
    var defaultDayEnd: String = "17:30"
    var includeWeekends: Bool = false

    // Work-time logger import
    var breakKeywords: [String] = ["lunch", "ebéd", "break", "szünet", "pause", "pihenő"]
    var shortDateMonthFirst: Bool = true   // "11.03." = 3 November (Hungarian order)

    // Absences (leave, public holidays from the HR system)
    var absenceIssueKey: String = ""
    var absenceRules: [MeetingRule] = []   // day label contains → issue, e.g. "Betegszabadság" → HR-3

    // Project periods: workdays inside are logged to the period's issue
    var periods: [ProjectPeriod] = []

    // Calendars read through macOS (any account in Internet Accounts)
    var macCalendarIds: [String] = []

    // Google Calendar API (optional, needs an own OAuth client)
    var googleClientId: String = ""
    var googleCalendarId: String = "primary"
    var defaultMeetingIssue: String = ""
    var meetingRules: [MeetingRule] = []
    var skipMeetingKeywords: [String] = []

    // Jira / Tempo
    var jiraBaseURL: String = ""
    var jiraEmail: String = ""
    var jiraAccountId: String = ""
    var tempoBaseURL: String = "https://api.tempo.io/4"

    /// Decodes stored settings on top of the defaults so new fields never break old databases.
    static func decodeMerging(_ data: Data) -> AppSettings? {
        guard let defaultsData = try? JSONEncoder().encode(AppSettings()),
              var base = (try? JSONSerialization.jsonObject(with: defaultsData)) as? [String: Any],
              let stored = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
        else { return nil }
        base.merge(stored) { _, new in new }
        guard let merged = try? JSONSerialization.data(withJSONObject: base) else { return nil }
        return try? JSONDecoder().decode(AppSettings.self, from: merged)
    }
}
