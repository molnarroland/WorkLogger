package view

import (
	"slices"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// PlanKind says where a ticket-day's outside time comes from: Tempo Planner ("plan") or a worklog Tempo already has
// that WorkLogger didn't send ("tempo", e.g. one Jira or Tempo logged by itself when the ticket changed status).
type PlanKind string

const (
	PlanKindPlan  PlanKind = "plan"
	PlanKindTempo PlanKind = "tempo"
)

// Planned is time Tempo has for a ticket on a day, as read back.
type Planned struct {
	Kind        PlanKind
	Key, Day    string
	Seconds     int
	Description string
}

// PlanItem is one ticket-day from Tempo with WorkLogger's own entries on the same ticket and day (its commits,
// chats, meetings…).
type PlanItem struct {
	Kind        PlanKind
	Key, Day    string
	Seconds     int
	Description string
	// Logged is the time of Entries.
	Logged  int
	Entries []core.Worklog
}

// PlanDay is a day's plans, with the day's entries that still need a ticket and the ticket suggested for them.
type PlanDay struct {
	Day string
	// Planned is the day's Tempo Planner time (worklogs found in Tempo don't count).
	Planned  int
	Items    []PlanItem
	Untagged []core.Worklog
	Suggest  *string
}

// BuildPlans puts the plans and outside worklogs under their days (oldest first), each ticket-day once per kind,
// largest first. Days with neither are left out.
func BuildPlans(planned []Planned, logs []core.Worklog) []PlanDay {
	type ref struct {
		kind     PlanKind
		key, day string
	}
	items := map[ref]*PlanItem{}
	var order []ref
	for _, p := range planned {
		if p.Key == "" || p.Seconds <= 0 {
			continue
		}
		r := ref{p.Kind, p.Key, p.Day}
		it, ok := items[r]
		if !ok {
			it = &PlanItem{Kind: p.Kind, Key: p.Key, Day: p.Day}
			items[r] = it
			order = append(order, r)
		}
		it.Seconds += p.Seconds
		if d := strings.TrimSpace(p.Description); d != "" && !strings.Contains(it.Description, d) {
			it.Description = strings.TrimPrefix(it.Description+"; "+d, "; ")
		}
	}
	byKeyDay := map[string][]core.Worklog{}
	untagged := map[string][]core.Worklog{}
	for _, w := range logs {
		if w.Duration <= 0 {
			continue
		}
		if w.IssueKey == nil {
			untagged[w.Day] = append(untagged[w.Day], w)
			continue
		}
		byKeyDay[*w.IssueKey+"|"+w.Day] = append(byKeyDay[*w.IssueKey+"|"+w.Day], w)
	}
	days := map[string]*PlanDay{}
	plannedByRef := map[string]int{}
	for _, r := range order {
		it := items[r]
		it.Entries = byKeyDay[r.key+"|"+r.day]
		it.Logged = core.TotalDuration(it.Entries)
		d, ok := days[r.day]
		if !ok {
			d = &PlanDay{Day: r.day}
			days[r.day] = d
		}
		if r.kind == PlanKindPlan {
			d.Planned += it.Seconds
			plannedByRef[r.key+"|"+r.day] += it.Seconds
		}
		d.Items = append(d.Items, *it)
	}
	out := []PlanDay{}
	for _, d := range days {
		slices.SortStableFunc(d.Items, func(a, b PlanItem) int {
			if a.Kind != b.Kind {
				return compare(string(a.Kind), string(b.Kind)) // plans before Tempo's worklogs
			}
			if a.Seconds != b.Seconds {
				return b.Seconds - a.Seconds
			}
			return compare(a.Key, b.Key)
		})
		d.Untagged = untagged[d.Day]
		if len(d.Untagged) > 0 {
			d.Suggest = PlanIssue(plannedByRef, d.Day)
		}
		out = append(out, *d)
	}
	slices.SortFunc(out, func(a, b PlanDay) int { return compare(a.Day, b.Day) })
	return out
}

// PlannedByRef sums Tempo Planner time by "KEY|day" (worklogs found in Tempo left out).
func PlannedByRef(planned []Planned) map[string]int {
	out := map[string]int{}
	for _, p := range planned {
		if p.Kind == PlanKindPlan && p.Key != "" && p.Seconds > 0 {
			out[p.Key+"|"+p.Day] += p.Seconds
		}
	}
	return out
}
