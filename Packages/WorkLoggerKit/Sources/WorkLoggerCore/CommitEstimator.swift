import Foundation

/// Estimates how long a commit took from its size, the kind of change and the free time since the previous commit.
public struct CommitEstimator: Sendable {
    public var settings: AppSettings

    private static let trivial = [
        "typo", "bump", "version", "merge", "format", "lint", "cs fix", "cs-fix",
        "style:", "chore", "rename", "wip",
    ]
    private static let heavy = ["refactor", "migrat", "rewrite", "redesign", "architecture"]
    private static let debugging = ["fix", "bug", "hotfix", "debug", "investigat"]
    private static let testing = ["test", "spec"]

    /// Size/complexity-only estimate in minutes.
    public func complexityMinutes(_ c: StoredCommit) -> Double {
        let churn = Double(c.insertions + c.deletions)
        var minutes = 10 + 12 * log2(1 + churn / 10) + 4 * Double(min(c.filesChanged, 20))
        let subject = c.subject.lowercased()
        if Self.trivial.contains(where: { subject.contains($0) }) {
            minutes *= 0.5
        } else if Self.heavy.contains(where: { subject.contains($0) }) {
            minutes *= 1.3
        } else if Self.debugging.contains(where: { subject.contains($0) }) {
            minutes *= 1.15  // small diffs often hide long investigations
        } else if Self.testing.contains(where: { subject.contains($0) }) {
            minutes *= 1.1
        }
        return minutes
    }

    /// Final estimate in seconds. `freeGap` is working time (meetings and breaks removed) since the previous commit.
    public func estimateSeconds(_ c: StoredCommit, freeGap: TimeInterval?) -> Int {
        let complexity = complexityMinutes(c)
        var minutes = complexity
        if let gap = freeGap, gap > 0, gap <= Double(settings.maxCommitGapMinutes * 60) {
            // Real elapsed time is the best signal; complexity smooths out pauses we can't see.
            minutes = 0.6 * (gap / 60) + 0.4 * complexity
        }
        minutes = min(Double(settings.maxCommitMinutes), max(Double(settings.minCommitMinutes), minutes))
        return Int((minutes / 5).rounded()) * 5 * 60
    }

    public init(settings: AppSettings) {
        self.settings = settings
    }
}
