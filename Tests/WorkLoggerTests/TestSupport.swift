import Foundation
@testable import WorkLogger

/// Shared builders for test data. Every suite calls `pinTimeZone()` in its `init`, so day keys are computed in
/// Budapest no matter where the tests run (all suites pin the same zone, so parallel tests never disagree).
enum Fixture {
    static let timeZone = TimeZone(identifier: "Europe/Budapest")!

    static func pinTimeZone() {
        DayKey.configure(timeZone: timeZone, locale: Locale(identifier: "hu_HU"))
    }

    /// A time on a day, e.g. `at("2026-03-03", "09:30")`.
    static func at(_ day: String, _ hm: String) -> Date {
        guard let d = DayKey.at(day, hm: hm) else { fatalError("Bad fixture time \(day) \(hm)") }
        return d
    }

    static func interval(_ day: String, _ from: String, _ to: String) -> Interval {
        Interval(start: at(day, from), end: at(day, to))
    }

    static func commit(_ hash: String = UUID().uuidString, day: String, _ hm: String, subject: String = "Work",
                       body: String = "", ref: String = "", repoId: Int64 = 1,
                       files: Int = 1, insertions: Int = 10, deletions: Int = 0) -> StoredCommit {
        StoredCommit(repoId: repoId, hash: hash, date: at(day, hm), author: "Me", email: "me@example.com",
                     subject: subject, body: body, ref: ref, filesChanged: files,
                     insertions: insertions, deletions: deletions)
    }

    static func meeting(_ id: String = UUID().uuidString, day: String, _ from: String, _ to: String,
                        title: String = "Meeting", calendarId: String = "cal", skipped: Bool = false) -> Meeting {
        Meeting(id: id, calendarId: calendarId, title: title, details: "", start: at(day, from), end: at(day, to),
                skipped: skipped)
    }

    static func session(day: String, _ from: String, _ to: String, kind: SessionKind = .work) -> WorkSession {
        WorkSession(id: nil, day: day, start: at(day, from), end: at(day, to), kind: kind, label: "")
    }

    static func worklog(day: String, _ hm: String, duration: Int, issue: String? = "TAO-1", source: WorklogSource = .git,
                        ref: String = "", status: WorklogStatus = .draft, locked: Bool = false) -> Worklog {
        Worklog(id: 0, day: day, start: at(day, hm), duration: duration, issueKey: issue, description: "Entry",
                source: source, sourceRef: ref, repoId: nil, estimated: duration, status: status, locked: locked,
                tempoId: nil)
    }

    static func builder(_ settings: AppSettings = AppSettings(), repos: [Repo] = [],
                        projects: [ProjectMapping] = []) -> WorklogBuilder {
        WorklogBuilder(settings: settings,
                       repos: Dictionary(uniqueKeysWithValues: repos.map { ($0.id, $0) }),
                       projects: Dictionary(uniqueKeysWithValues: projects.map { ($0.key, $0) }))
    }

    static func repo(id: Int64 = 1, name: String = "billing", issueKey: String? = nil, projectKey: String? = nil) -> Repo {
        Repo(id: id, path: "/tmp/\(name)", name: name, issueKey: issueKey, projectKey: projectKey, enabled: true)
    }
}

extension Array where Element == Worklog {
    var totalDuration: Int { reduce(0) { $0 + $1.duration } }
}
