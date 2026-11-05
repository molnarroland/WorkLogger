import Foundation
import Testing
import WorkLoggerTestSupport

@testable import WorkLoggerCore

@Suite struct CommitEstimatorTests {
    init() { Fixture.pinTimeZone() }

    let estimator = CommitEstimator(settings: AppSettings())  // min 10, max 240, gap ≤ 180 minutes

    /// 10 lines in 1 file: 10 + 12·log2(2) + 4 = 26 minutes before keyword weighting.
    func commit(_ subject: String, files: Int = 1, insertions: Int = 10) -> StoredCommit {
        Fixture.commit(day: "2026-03-03", "10:00", subject: subject, files: files, insertions: insertions)
    }

    @Test func emptyCommitCostsTheBaseTen() {
        #expect(estimator.complexityMinutes(commit("Add feature", files: 0, insertions: 0)) == 10)
    }

    @Test(arguments: [
        ("Add feature", 26.0), ("Fix typo", 13.0), ("Refactor billing", 33.8),
        ("Fix bug in invoices", 29.9), ("Add tests", 28.6),
    ])
    func keywordsWeightTheSize(subject: String, expected: Double) {
        #expect(abs(estimator.complexityMinutes(commit(subject)) - expected) < 0.001)
    }

    @Test func withoutGapUsesComplexityRoundedToFiveMinutes() {
        #expect(estimator.estimateSeconds(commit("Add feature"), freeGap: nil) == 25 * 60)
    }

    @Test func gapIsBlendedWithComplexity() {
        // 0.6 · 60 + 0.4 · 26 = 46.4 → 45 minutes
        #expect(estimator.estimateSeconds(commit("Add feature"), freeGap: 3600) == 45 * 60)
    }

    @Test(arguments: [0.0, -60.0, 181.0 * 60])
    func zeroOrTooLongGapsAreIgnored(gap: TimeInterval) {
        #expect(estimator.estimateSeconds(commit("Add feature"), freeGap: gap) == 25 * 60)
    }

    @Test func clampsToMinimumAndMaximum() {
        #expect(estimator.estimateSeconds(commit("wip", files: 0, insertions: 0), freeGap: nil) == 10 * 60)
        #expect(estimator.estimateSeconds(commit("Add feature", files: 50, insertions: 100_000), freeGap: nil) == 240 * 60)
    }
}
