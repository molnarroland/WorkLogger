import Foundation

public enum RoundingMode: String, Codable, CaseIterable, Identifiable, Sendable {
    /// `sessions` behaves like `fixedHours` and is kept only so older saved settings still decode.
    case estimate, fixedHours, sessions
    public var id: String { rawValue }
    public static let choices: [RoundingMode] = [.fixedHours, .estimate]
}

public struct MeetingRule: Codable, Hashable, Identifiable, Sendable {
    public var id = UUID()
    public var contains: String
    public var issueKey: String

    public init(id: UUID = UUID(), contains: String, issueKey: String) {
        self.id = id
        self.contains = contains
        self.issueKey = issueKey
    }
}

/// A stretch of days booked to one issue, e.g. a two-week migration project.
/// A conversation that is always logged to the same ticket (Chats → "always").
public struct ChatRule: Codable, Hashable, Identifiable, Sendable {
    public var conversationId: String
    public var name: String
    public var issueKey: String
    public var id: String { conversationId }

    public init(conversationId: String, name: String, issueKey: String) {
        self.conversationId = conversationId
        self.name = name
        self.issueKey = issueKey
    }
}

public struct ProjectPeriod: Codable, Hashable, Identifiable, Sendable {
    public var id = UUID()
    public var title: String
    public var issueKey: String
    public var from: String  // day keys, inclusive
    public var to: String

    public func contains(_ day: String) -> Bool { from <= day && day <= to }

    public init(id: UUID = UUID(), title: String, issueKey: String, from: String, to: String) {
        self.id = id
        self.title = title
        self.issueKey = issueKey
        self.from = from
        self.to = to
    }
}

/// Optional data sources, switched on the welcome screen (names: the app's DisplayNames). Off means the source is neither read nor used for drafts.
public enum Module: String, Codable, CaseIterable, Identifiable, Sendable {
    case git, calendar, workTimes, chats
    public var id: String { rawValue }
    public var icon: String {
        switch self {
        case .git: return "externaldrive.connected.to.line.below"
        case .calendar: return "person.2"
        case .workTimes: return "clock.badge.checkmark"
        case .chats: return "bubble.left.and.bubble.right"
        }
    }
}

/// Where an account's worklogs live.
public enum WorklogProvider: String, Codable, CaseIterable, Identifiable, Sendable {
    /// Tempo Timesheets (Tempo REST API).
    case tempo
    /// Jira's own worklogs (Log work on the issue).
    case jira
    /// Another tool: WorkLogger only exports files and checklists for it.
    case other
    public var id: String { rawValue }
}

/// One Jira site with its own sign-in and worklog provider; issues go to the account that owns their project key.
public struct JiraAccount: Codable, Hashable, Identifiable, Sendable {
    public var id = UUID()
    public var name = ""
    public var baseURL = ""
    public var email = ""
    /// Your Jira account id on this site, filled in automatically.
    public var accountId = ""
    public var provider: WorklogProvider = .tempo
    public var tempoBaseURL = "https://api.tempo.io/4"
    /// The site's project keys (read from Jira), used to pick the account for an issue key.
    public var projectKeys: [String] = []

    public var site: String { baseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/ ")) }
    public var title: String { name.isEmpty ? (URL(string: site)?.host() ?? site) : name }
    /// Keychain account names of this account's tokens.
    public var jiraTokenKey: String { "jira.apiToken.\(id.uuidString)" }
    public var tempoTokenKey: String { "tempo.apiToken.\(id.uuidString)" }

    public init(
        id: UUID = UUID(), name: String = "", baseURL: String = "", email: String = "", provider: WorklogProvider = .tempo
    ) {
        self.id = id
        self.name = name
        self.baseURL = baseURL
        self.email = email
        self.provider = provider
    }
}

public struct AppSettings: Codable, Equatable, Sendable {
    // Modules and first-run welcome
    public var modules: Set<Module> = Set(Module.allCases)
    /// Modules that existed when settings were last saved; modules added in later versions start switched on.
    public var knownModules: Set<Module> = [.git, .calendar, .workTimes]
    /// Logging in Tempo by hand (By ticket) rather than with the Tempo API; the API then only asks for its key.
    public var tempoByHand = true
    public var welcomed = false

    // Git
    public var workspaceRoot: String = ""
    public var authorEmails: [String] = []
    public var excludedFolders: [String] = []  // folder names or paths skipped when scanning
    public var ignoredPathPatterns: [String] = [
        "package-lock.json", "composer.lock", "yarn.lock", "pnpm-lock.yaml", "Podfile.lock",
        ".min.js", ".min.css", "/dist/", "/vendor/", "node_modules/", ".snap", ".svg",
    ]

    // Estimation
    public var minCommitMinutes: Int = 10
    public var maxCommitMinutes: Int = 240
    public var maxCommitGapMinutes: Int = 180

    // Daily totals
    public var roundingMode: RoundingMode = .fixedHours
    public var fixedDailyHours: Double = 8
    public var granularityMinutes: Int = 15
    public var dayRoundUpMinutes: Int = 15  // worked time is rounded up: 7h 22m → 7h 30m
    public var defaultDayStart: String = "09:00"
    public var defaultDayEnd: String = "17:30"
    public var includeWeekends: Bool = false

    // Menu bar item with today's time and quick actions
    public var showMenuBarItem = true

    // Chats
    public var chatGapMinutes: Int = 10  // messages further apart start a new stretch of chatting
    public var chatRules: [ChatRule] = []

    // In-app sign-in sources
    public var chatEmail: String = ""  // your Google Chat address; the first commit email when empty
    public var hrmasterURL: String = ""

    // Absences (leave, public holidays from the HR system)
    public var absenceIssueKey: String = ""
    public var absenceRules: [MeetingRule] = []  // day label contains → issue, e.g. "Betegszabadság" → HR-3

    // Project periods: workdays inside are logged to the period's issue
    public var periods: [ProjectPeriod] = []

    // Calendars read through macOS (any account in Internet Accounts)
    public var macCalendarIds: [String] = []

    // Google Calendar API (optional, needs an own OAuth client)
    public var googleClientId: String = ""
    public var googleCalendarId: String = "primary"
    public var defaultMeetingIssue: String = ""
    public var meetingRules: [MeetingRule] = []
    public var skipMeetingKeywords: [String] = []

    // Jira sites and their worklog providers
    public var jiraAccounts: [JiraAccount] = []
    /// Gets issues whose project no account lists, and the exports' links.
    public var defaultJiraAccountId: UUID?

    /// The default account (the first one when none is picked).
    public var defaultJiraAccount: JiraAccount? {
        jiraAccounts.first { $0.id == defaultJiraAccountId } ?? jiraAccounts.first
    }

    /// The account whose project keys include the issue's project ("TAO-12" → TAO), else the default.
    public func jiraAccount(for issueKey: String) -> JiraAccount? {
        let project = issueKey.split(separator: "-").first.map { $0.uppercased() } ?? ""
        return jiraAccounts.first { $0.projectKeys.contains(project) } ?? defaultJiraAccount
    }

    /// Decodes stored settings on top of the defaults so new fields never break old databases.
    public static func decodeMerging(_ data: Data) -> AppSettings? {
        guard let defaultsData = try? JSONEncoder().encode(AppSettings()),
            var base = (try? JSONSerialization.jsonObject(with: defaultsData)) as? [String: Any],
            let stored = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
        else { return nil }
        base.merge(stored) { _, new in new }
        guard let merged = try? JSONSerialization.data(withJSONObject: base),
            var settings = try? JSONDecoder().decode(AppSettings.self, from: merged)
        else { return nil }
        // Before accounts there was one Jira site in flat fields; it becomes the first account.
        if stored["jiraAccounts"] == nil, let site = stored["jiraBaseURL"] as? String, !site.isEmpty {
            var account = JiraAccount(baseURL: site, email: stored["jiraEmail"] as? String ?? "")
            account.accountId = stored["jiraAccountId"] as? String ?? ""
            account.tempoBaseURL = stored["tempoBaseURL"] as? String ?? account.tempoBaseURL
            settings.jiraAccounts = [account]
            settings.defaultJiraAccountId = account.id
        }
        return settings
    }

    public init() {}
}
