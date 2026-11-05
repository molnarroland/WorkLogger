import Foundation
import WorkLoggerCore

/// Reads repositories and history. The app runs the `git` command line tool.
public protocol GitService: Sendable {
    func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit]
    func globalUserEmail() -> String?
    func findRepos(root: URL, excluded: [String]) -> [URL]
}

public struct SystemGit: GitService {
    public init() {}
    public func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit] {
        try GitLogReader.log(repoPath: repoPath, since: since, until: until, authors: authors)
    }
    public func globalUserEmail() -> String? { GitScanner.globalUserEmail() }
    public func findRepos(root: URL, excluded: [String]) -> [URL] { GitScanner.findRepos(root: root, excluded: excluded) }
}
