package core

import (
	"math"
	"strings"
)

var (
	trivialWords   = []string{"typo", "bump", "version", "merge", "format", "lint", "cs fix", "cs-fix", "style:", "chore", "rename", "wip"}
	heavyWords     = []string{"refactor", "migrat", "rewrite", "redesign", "architecture"}
	debuggingWords = []string{"fix", "bug", "hotfix", "debug", "investigat"}
	testingWords   = []string{"test", "spec"}
)

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// ComplexityMinutes is the size/complexity-only estimate of a commit.
func ComplexityMinutes(c StoredCommit) float64 {
	churn := float64(c.Insertions + c.Deletions)
	minutes := 10 + 12*math.Log2(1+churn/10) + 4*float64(min(c.FilesChanged, 20))
	subject := strings.ToLower(c.Subject)
	switch { // first match wins, in this order
	case containsAny(subject, trivialWords):
		minutes *= 0.5
	case containsAny(subject, heavyWords):
		minutes *= 1.3
	case containsAny(subject, debuggingWords):
		minutes *= 1.15 // small diffs often hide long investigations
	case containsAny(subject, testingWords):
		minutes *= 1.1
	}
	return minutes
}

// EstimateSeconds is the final estimate. freeGap is working time (meetings and breaks removed) since the previous
// commit, in seconds; nil when unknown.
func EstimateSeconds(s Settings, c StoredCommit, freeGap *float64) int {
	complexity := ComplexityMinutes(c)
	minutes := complexity
	if freeGap != nil && *freeGap > 0 && *freeGap <= float64(s.MaxCommitGapMinutes*60) {
		// Real elapsed time is the best signal; complexity smooths out pauses we can't see.
		minutes = 0.6*(*freeGap/60) + 0.4*complexity
	}
	minutes = math.Min(float64(s.MaxCommitMinutes), math.Max(float64(s.MinCommitMinutes), minutes))
	return int(math.Round(minutes/5)) * 5 * 60
}
