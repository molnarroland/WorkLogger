import Foundation
import WorkLoggerCore

/// Reading a range of history from several repositories at once.
public enum GitHistory {
    /// Your commits per repository in [since, until), read off the main thread. A repository that can't be read is
    /// reported by name in `errors` instead of stopping the others.
    public static func read(
        _ repos: [Repo], since: Date, until: Date, authors: [String],
        git: any GitService
    ) async -> (commits: [(repoId: Int64, commits: [GitCommit])], errors: [String]) {
        await Task.detached(priority: .userInitiated) {
            var ok: [(repoId: Int64, commits: [GitCommit])] = []
            var failed: [String] = []
            for repo in repos {
                do {
                    ok.append((repo.id, try git.log(repoPath: repo.path, since: since, until: until, authors: authors)))
                } catch {
                    failed.append("\(repo.name): \(error.localizedDescription)")
                    Log.git.error(
                        "Reading \(repo.path, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
                }
            }
            Log.git.info("Read \(ok.reduce(0) { $0 + $1.commits.count }) commits from \(ok.count) repositories")
            return (ok, failed)
        }.value
    }
}
