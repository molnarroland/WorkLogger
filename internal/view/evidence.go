package view

import (
	"slices"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// MeetingRow is a meeting with what the rules made of it.
type MeetingRow struct {
	core.Meeting
	IssueKey         *string
	SkippedByKeyword bool
}

// Suggestion is the most frequent untagged meeting title, worth a rule.
type Suggestion struct {
	Title string
	Count int
}

// MeetingSuggestion is the title that most often has no ticket among the meetings that count (not skipped by hand
// or by keyword), when it comes up at least twice; the earliest such title wins a tie.
func MeetingSuggestion(rows []MeetingRow) *Suggestion {
	var titles []string
	counts := map[string]int{}
	for _, m := range rows {
		if m.IssueKey != nil || m.Skipped || m.SkippedByKeyword {
			continue
		}
		if counts[m.Title] == 0 {
			titles = append(titles, m.Title)
		}
		counts[m.Title]++
	}
	var best *Suggestion
	for _, t := range titles {
		if counts[t] >= 2 && (best == nil || counts[t] > best.Count) {
			best = &Suggestion{t, counts[t]}
		}
	}
	return best
}

// DayGroup is one day's items with their time.
type DayGroup[T any] struct {
	Day     string
	Seconds int
	Items   []T
}

// GroupByDay puts items under their days, oldest day first, each day with its total time; the input order is
// kept within a day.
func GroupByDay[T any](items []T, day func(T) string, seconds func(T) int) []DayGroup[T] {
	index := map[string]int{}
	var out []DayGroup[T]
	for _, it := range items {
		d := day(it)
		i, ok := index[d]
		if !ok {
			i = len(out)
			index[d] = i
			out = append(out, DayGroup[T]{Day: d})
		}
		out[i].Items = append(out[i].Items, it)
		out[i].Seconds += seconds(it)
	}
	slices.SortStableFunc(out, func(a, b DayGroup[T]) int { return compare(a.Day, b.Day) })
	return out
}
