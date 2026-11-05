import Foundation
import Testing
@testable import WorkLoggerGit
import WorkLoggerCore
import WorkLoggerTestSupport

@Suite struct GitHistoryTests {
    final class Git: GitService, @unchecked Sendable {
        func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit] {
            if repoPath.hasSuffix("broken") { throw FakeError(message: "not a git repository") }
            return [GitCommit(hash: repoPath, date: since, author: "Me", email: authors[0], subject: "x", body: "", ref: "", files: [])]
        }
        func globalUserEmail() -> String? { nil }
        func findRepos(root: URL, excluded: [String]) -> [URL] { [] }
    }

    @Test func readsEachRepositoryAndReportsTheBrokenOnes() async {
        let repos = [Repo(id: 1, path: "/w/billing", name: "billing", issueKey: nil, projectKey: nil, enabled: true),
                     Repo(id: 2, path: "/w/broken", name: "broken", issueKey: nil, projectKey: nil, enabled: true)]
        let r = await GitHistory.read(repos, since: Date(timeIntervalSince1970: 0), until: Date(), authors: ["me@x.hu"], git: Git())
        #expect(r.commits.map(\.repoId) == [1])
        #expect(r.commits[0].commits.map(\.hash) == ["/w/billing"])
        #expect(r.errors == ["broken: not a git repository"])
    }
}
