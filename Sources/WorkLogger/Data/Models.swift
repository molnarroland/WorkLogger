import Foundation

struct Repo: Identifiable, Hashable {
    var id: Int64
    var path: String
    var name: String
    var issueKey: String?
    var projectKey: String?
    var enabled: Bool
}

/// A Jira project a repository can be assigned to. Tempo can only log to issues,
/// so a project needs a catch-all issue (e.g. "PROJ-1 General development").
struct ProjectMapping: Identifiable, Hashable {
    var key: String
    var defaultIssue: String?
    var id: String { key }
}

struct FileStat: Hashable {
    var added: Int
    var deleted: Int
    var path: String
}

/// A commit as read from `git log`.
struct GitCommit: Hashable {
    var hash: String
    var date: Date
    var author: String
    var email: String
    var subject: String
    var body: String
    var ref: String
    var files: [FileStat]
}

/// A commit as stored in SQLite (ignored paths such as lockfiles already removed from the stats).
struct StoredCommit: Hashable {
    var repoId: Int64
    var hash: String
    var date: Date
    var author: String
    var email: String
    var subject: String
    var body: String
    var ref: String
    var filesChanged: Int
    var insertions: Int
    var deletions: Int

    init(repoId: Int64, hash: String, date: Date, author: String, email: String, subject: String,
         body: String, ref: String, filesChanged: Int, insertions: Int, deletions: Int) {
        self.repoId = repoId; self.hash = hash; self.date = date; self.author = author; self.email = email
        self.subject = subject; self.body = body; self.ref = ref
        self.filesChanged = filesChanged; self.insertions = insertions; self.deletions = deletions
    }

    init(_ c: GitCommit, repoId: Int64, ignoredPaths: [String]) {
        let files = c.files.filter { f in !ignoredPaths.contains { !$0.isEmpty && f.path.contains($0) } }
        self.init(repoId: repoId, hash: c.hash, date: c.date, author: c.author, email: c.email,
                  subject: c.subject, body: c.body, ref: c.ref,
                  filesChanged: files.count,
                  insertions: files.reduce(0) { $0 + $1.added },
                  deletions: files.reduce(0) { $0 + $1.deleted })
    }
}

enum SessionKind: String, Codable, CaseIterable {
    case work, pause
}

/// A work session or break imported from the work-time logger.
struct WorkSession: Identifiable, Hashable {
    var id: Int64?
    var day: String
    var start: Date
    var end: Date
    var kind: SessionKind
    var label: String
    var duration: Int { Int(end.timeIntervalSince(start)) }
}

struct Meeting: Identifiable, Hashable {
    var id: String
    var calendarId: String
    var title: String
    var details: String
    var start: Date
    var end: Date
    var skipped: Bool
    var duration: Int { Int(end.timeIntervalSince(start)) }
}

enum WorklogSource: String, Codable, CaseIterable {
    case git, meeting, absence, manual, period

    var icon: String {
        switch self {
        case .git: return "arrow.triangle.branch"
        case .meeting: return "person.2"
        case .absence: return "beach.umbrella"
        case .manual: return "pencil"
        case .period: return "flag"
        }
    }
    var label: String {
        switch self {
        case .git: return "Coding"
        case .meeting: return "Meetings"
        case .absence: return "Absence"
        case .manual: return "Manual"
        case .period: return "Periods"
        }
    }
}

enum WorklogStatus: String, Codable, CaseIterable {
    case draft, reviewed, exported
}

struct Worklog: Identifiable, Hashable {
    var id: Int64
    var day: String
    var start: Date
    var duration: Int          // seconds
    var issueKey: String?
    var description: String
    var source: WorklogSource
    var sourceRef: String      // commit hashes or calendar event id, comma separated
    var repoId: Int64?
    var estimated: Int         // raw estimate before daily rounding, seconds
    var status: WorklogStatus
    var locked: Bool           // kept when drafts are rebuilt
    var tempoId: Int64?

    var refs: [String] { sourceRef.split(separator: ",").map(String.init) }
    var end: Date { start.addingTimeInterval(TimeInterval(duration)) }

    func sameContent(as other: Worklog) -> Bool {
        start == other.start && duration == other.duration && issueKey == other.issueKey
            && description == other.description && status == other.status
    }
}
