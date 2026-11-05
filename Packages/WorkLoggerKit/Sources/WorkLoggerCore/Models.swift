import Foundation

public struct Repo: Identifiable, Hashable, Sendable {
    public var id: Int64
    public var path: String
    public var name: String
    public var issueKey: String?
    public var projectKey: String?
    public var enabled: Bool

    public init(id: Int64, path: String, name: String, issueKey: String?, projectKey: String?, enabled: Bool) {
        self.id = id
        self.path = path
        self.name = name
        self.issueKey = issueKey
        self.projectKey = projectKey
        self.enabled = enabled
    }
}

/// A Jira project a repository can be assigned to. Tempo can only log to issues,
/// so a project needs a catch-all issue (e.g. "PROJ-1 General development").
public struct ProjectMapping: Identifiable, Hashable, Sendable {
    public var key: String
    public var defaultIssue: String?
    public var id: String { key }

    public init(key: String, defaultIssue: String?) {
        self.key = key
        self.defaultIssue = defaultIssue
    }
}

public struct FileStat: Hashable, Sendable {
    public var added: Int
    public var deleted: Int
    public var path: String

    public init(added: Int, deleted: Int, path: String) {
        self.added = added
        self.deleted = deleted
        self.path = path
    }
}

/// A commit as read from `git log`.
public struct GitCommit: Hashable, Sendable {
    public var hash: String
    public var date: Date
    public var author: String
    public var email: String
    public var subject: String
    public var body: String
    public var ref: String
    public var files: [FileStat]

    public init(hash: String, date: Date, author: String, email: String, subject: String, body: String, ref: String, files: [FileStat]) {
        self.hash = hash
        self.date = date
        self.author = author
        self.email = email
        self.subject = subject
        self.body = body
        self.ref = ref
        self.files = files
    }
}

/// A commit as stored in SQLite (ignored paths such as lockfiles already removed from the stats).
public struct StoredCommit: Hashable, Sendable {
    public var repoId: Int64
    public var hash: String
    public var date: Date
    public var author: String
    public var email: String
    public var subject: String
    public var body: String
    public var ref: String
    public var filesChanged: Int
    public var insertions: Int
    public var deletions: Int

    public init(repoId: Int64, hash: String, date: Date, author: String, email: String, subject: String,
         body: String, ref: String, filesChanged: Int, insertions: Int, deletions: Int) {
        self.repoId = repoId; self.hash = hash; self.date = date; self.author = author; self.email = email
        self.subject = subject; self.body = body; self.ref = ref
        self.filesChanged = filesChanged; self.insertions = insertions; self.deletions = deletions
    }

    public init(_ c: GitCommit, repoId: Int64, ignoredPaths: [String]) {
        let files = c.files.filter { f in !ignoredPaths.contains { !$0.isEmpty && f.path.contains($0) } }
        self.init(repoId: repoId, hash: c.hash, date: c.date, author: c.author, email: c.email,
                  subject: c.subject, body: c.body, ref: c.ref,
                  filesChanged: files.count,
                  insertions: files.reduce(0) { $0 + $1.added },
                  deletions: files.reduce(0) { $0 + $1.deleted })
    }
}

public enum SessionKind: String, Codable, CaseIterable, Sendable {
    case work, pause
}

/// A work session or break imported from the work-time logger.
public struct WorkSession: Identifiable, Hashable, Sendable {
    public var id: Int64?
    public var day: String
    public var start: Date
    public var end: Date
    public var kind: SessionKind
    public var label: String
    public var duration: Int { Int(end.timeIntervalSince(start)) }

    public init(id: Int64?, day: String, start: Date, end: Date, kind: SessionKind, label: String) {
        self.id = id
        self.day = day
        self.start = start
        self.end = end
        self.kind = kind
        self.label = label
    }
}

public struct Meeting: Identifiable, Hashable, Sendable {
    public var id: String
    public var calendarId: String
    public var title: String
    public var details: String
    public var start: Date
    public var end: Date
    public var skipped: Bool
    public var duration: Int { Int(end.timeIntervalSince(start)) }

    public init(id: String, calendarId: String, title: String, details: String, start: Date, end: Date, skipped: Bool) {
        self.id = id
        self.calendarId = calendarId
        self.title = title
        self.details = details
        self.start = start
        self.end = end
        self.skipped = skipped
    }
}

public enum WorklogSource: String, Codable, CaseIterable, Sendable {
    case git, meeting, absence, manual, period, chat

    public var icon: String {
        switch self {
        case .git: return "arrow.triangle.branch"
        case .meeting: return "person.2"
        case .absence: return "beach.umbrella"
        case .manual: return "pencil"
        case .period: return "flag"
        case .chat: return "bubble.left.and.bubble.right"
        }
    }
    public var label: String {
        switch self {
        case .git: return "Coding"
        case .meeting: return "Meetings"
        case .absence: return "Absence"
        case .manual: return "Manual"
        case .period: return "Periods"
        case .chat: return "Chats"
        }
    }
}

public enum WorklogStatus: String, Codable, CaseIterable, Sendable {
    case draft, reviewed, exported
    /// Exported covers both sent by the Tempo API and ticked off after entering it in Tempo by hand.
    public var label: String { self == .exported ? "In Tempo" : rawValue.capitalized }
}

public struct Worklog: Identifiable, Hashable, Sendable {
    public var id: Int64
    public var day: String
    public var start: Date
    public var duration: Int          // seconds
    public var issueKey: String?
    public var description: String
    public var source: WorklogSource
    public var sourceRef: String      // commit hashes or calendar event id, comma separated
    public var repoId: Int64?
    public var estimated: Int         // raw estimate before daily rounding, seconds
    public var status: WorklogStatus
    public var locked: Bool           // kept when drafts are rebuilt
    public var tempoId: Int64?

    public var refs: [String] { sourceRef.split(separator: ",").map(String.init) }
    public var end: Date { start.addingTimeInterval(TimeInterval(duration)) }

    public func sameContent(as other: Worklog) -> Bool {
        start == other.start && duration == other.duration && issueKey == other.issueKey
            && description == other.description && status == other.status
    }

    public init(id: Int64, day: String, start: Date, duration: Int, issueKey: String?, description: String, source: WorklogSource, sourceRef: String, repoId: Int64?, estimated: Int, status: WorklogStatus, locked: Bool, tempoId: Int64?) {
        self.id = id
        self.day = day
        self.start = start
        self.duration = duration
        self.issueKey = issueKey
        self.description = description
        self.source = source
        self.sourceRef = sourceRef
        self.repoId = repoId
        self.estimated = estimated
        self.status = status
        self.locked = locked
        self.tempoId = tempoId
    }
}
