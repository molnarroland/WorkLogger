package view

import "github.com/molnarroland/WorkLogger/internal/core"

// ExportChoice is what the export sheet would send for one setting of its two switches: the entries Tempo can
// take (time and an issue), their time, and how many are skipped for want of a ticket.
type ExportChoice struct {
	OnlyReviewed, IncludeExported bool
	IDs                           []int64
	Seconds                       int
	// Missing counts the chosen entries left out because they have no ticket.
	Missing int
}

// ExportChoices is every setting of the export sheet's switches. Without IncludeExported, entries already in
// Tempo stay out; with OnlyReviewed, drafts do too.
func ExportChoices(logs []core.Worklog) []ExportChoice {
	var out []ExportChoice
	for _, onlyReviewed := range []bool{false, true} {
		for _, includeExported := range []bool{false, true} {
			c := ExportChoice{OnlyReviewed: onlyReviewed, IncludeExported: includeExported, IDs: []int64{}}
			for _, w := range logs {
				if (!includeExported && w.Status == core.StatusExported) || (onlyReviewed && w.Status != core.StatusReviewed) {
					continue
				}
				switch {
				case w.Exportable():
					c.IDs = append(c.IDs, w.ID)
					c.Seconds += w.Duration
				case w.NeedsIssue():
					c.Missing++
				}
			}
			out = append(out, c)
		}
	}
	return out
}
