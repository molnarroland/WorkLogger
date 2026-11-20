package view

import (
	"fmt"
	"slices"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Period is how a report groups time.
type Period string

const (
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

// IssueTotal is the time logged to one issue; Issue nil is "No issue".
type IssueTotal struct {
	Issue   *string
	Seconds int
}

// ReportGroup is the time of one day, ISO week or month.
type ReportGroup struct {
	// ID is "2026-03-02", "2026-W10" or "2026-03".
	ID string
	// Start is the day its label is made from: the day, the week's Monday, or the month's first day.
	Start   string
	Total   int
	ByIssue []IssueTotal
	// BySource lists the sources with time, in core.AllSources order.
	BySource []SourceTotal
}

// Report groups worklogs by day, ISO week or month, oldest first; issues by time, largest first.
func Report(days core.Days, logs []core.Worklog, period Period) []ReportGroup {
	sorted := slices.Clone(logs)
	slices.SortStableFunc(sorted, func(a, b core.Worklog) int { return a.Start.Compare(b.Start) })
	var ids []string
	groups := map[string][]core.Worklog{}
	starts := map[string]string{}
	for _, w := range sorted {
		id, start := reportKey(days, w.Day, period)
		if _, ok := groups[id]; !ok {
			ids = append(ids, id)
			starts[id] = start
		}
		groups[id] = append(groups[id], w)
	}
	slices.Sort(ids)
	out := []ReportGroup{}
	for _, id := range ids {
		items := groups[id]
		g := ReportGroup{ID: id, Start: starts[id], Total: core.TotalDuration(items), ByIssue: []IssueTotal{}, BySource: []SourceTotal{}}
		index := map[string]int{}
		for _, w := range items {
			k := "\x00"
			if w.IssueKey != nil {
				k = *w.IssueKey
			}
			i, ok := index[k]
			if !ok {
				i = len(g.ByIssue)
				index[k] = i
				g.ByIssue = append(g.ByIssue, IssueTotal{Issue: w.IssueKey})
			}
			g.ByIssue[i].Seconds += w.Duration
		}
		slices.SortStableFunc(g.ByIssue, func(a, b IssueTotal) int { return b.Seconds - a.Seconds })
		for _, s := range core.AllSources {
			n := 0
			for _, w := range items {
				if w.Source == s {
					n += w.Duration
				}
			}
			if n > 0 {
				g.BySource = append(g.BySource, SourceTotal{s, n})
			}
		}
		out = append(out, g)
	}
	return out
}

func reportKey(days core.Days, day string, period Period) (id, start string) {
	switch period {
	case PeriodWeek:
		y, w := days.Date(day).ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w), WeekOf(days, day)[0]
	case PeriodMonth:
		return day[:7], day[:7] + "-01"
	}
	return day, day
}
