import Foundation
import Testing
import WorkLoggerCore
import WorkLoggerTestSupport

@testable import WorkLoggerGit

@Suite struct GitLogReaderTests {
    /// One record in the `--pretty` format `GitLogReader.log` asks git for, followed by `--numstat` lines.
    func record(
        _ hash: String, date: String = "2026-03-03T10:00:00+01:00", subject: String = "TAO-1 Fix",
        body: String = "Body text\n", ref: String = "refs/heads/main", numstat: [String] = []
    ) -> String {
        "\u{1e}" + [hash, date, "Roland", "me@example.com", subject, body, ref].joined(separator: "\u{1f}")
            + "\u{1d}\n" + numstat.joined(separator: "\n") + "\n"
    }

    @Test func parsesHeaderAndFileStats() throws {
        let output = record("abc123", numstat: ["5\t2\tsrc/a.swift", "-\t-\timg.png"])
        let commit = try #require(GitLogReader.parse(output).first)
        #expect(commit.hash == "abc123")
        #expect(commit.date == ISO8601DateFormatter().date(from: "2026-03-03T09:00:00Z"))
        #expect(commit.author == "Roland")
        #expect(commit.email == "me@example.com")
        #expect(commit.subject == "TAO-1 Fix")
        #expect(commit.body == "Body text")
        #expect(commit.ref == "refs/heads/main")
        #expect(
            commit.files == [
                FileStat(added: 5, deleted: 2, path: "src/a.swift"),
                FileStat(added: 0, deleted: 0, path: "img.png"),
            ])  // binary
    }

    @Test func pathsWithTabsStayWhole() throws {
        let commit = try #require(GitLogReader.parse(record("a", numstat: ["1\t1\tdir/with\ttab.txt"])).first)
        #expect(commit.files.map(\.path) == ["dir/with\ttab.txt"])
    }

    @Test func duplicatesFromSeveralBranchesAreDropped() {
        let output = record("a", ref: "refs/heads/main") + record("a", ref: "refs/heads/feature") + record("b")
        #expect(GitLogReader.parse(output).map(\.hash) == ["a", "b"])
    }

    @Test func malformedRecordsAreSkipped() {
        let output = record("bad-date", date: "yesterday") + "\u{1e}no header end" + record("ok")
        #expect(GitLogReader.parse(output).map(\.hash) == ["ok"])
    }

    @Test func emptyOutput() {
        #expect(GitLogReader.parse("").isEmpty)
    }
}

@Suite struct StoredCommitTests {
    @Test func ignoredPathsAreRemovedFromTheStats() {
        let git = GitCommit(
            hash: "a", date: Date(), author: "", email: "", subject: "", body: "", ref: "",
            files: [
                FileStat(added: 5, deleted: 2, path: "src/a.swift"),
                FileStat(added: 1000, deleted: 500, path: "package-lock.json"),
                FileStat(added: 3, deleted: 0, path: "web/dist/app.js"),
            ])
        let stored = StoredCommit(git, repoId: 7, ignoredPaths: ["package-lock.json", "/dist/", ""])
        #expect(stored.repoId == 7)
        #expect(stored.filesChanged == 1)
        #expect(stored.insertions == 5)
        #expect(stored.deletions == 2)
    }
}

@Suite struct WorklogModelTests {
    init() { Fixture.pinTimeZone() }

    @Test func refsAndEnd() {
        let w = Fixture.worklog(day: "2026-03-03", "09:00", duration: 5400, ref: "a,b")
        #expect(w.refs == ["a", "b"])
        #expect(DayKey.hm(w.end) == "10:30")
        #expect(Fixture.worklog(day: "2026-03-03", "09:00", duration: 60).refs.isEmpty)
    }
}
