package view

import (
	"fmt"
	"slices"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// TicketDay is one ticket's time on one day: one row to type into Tempo.
type TicketDay struct {
	Key         *string
	Day         string
	Seconds     int
	Description string
	IDs         []int64
	// Entered: all of the day's entries are in Tempo (sent, ticked off by hand, or found by read-back).
	Entered bool
	// Found: Tempo's read-back has it (so it can't be ticked off by hand).
	Found bool
	// Planned is the time Tempo Planner has for the ticket on the day (seconds; 0 when none or not read).
	Planned int
}

// TicketGroup is a ticket's days. Key nil is "No issue".
type TicketGroup struct {
	Key         *string
	Days        []TicketDay
	Seconds     int
	EnteredDays int
	// EnteredSeconds is the time of the entered days; RemainingSeconds the rest.
	EnteredSeconds, RemainingSeconds int
	// Done: a ticket whose days are all in Tempo.
	Done bool
	// Text is the plain list for a note or for Claude: "2026-11-03  1h 15m  Daily standup".
	Text string
}

// TempoProgress counts ticket-days in Tempo; "No issue" doesn't count.
type TempoProgress struct {
	Entered, Total int
}

// TicketGroups makes one row per ticket-day: "No issue" first, then tickets by time, largest first. Entries
// without time are left out.
// planned holds Tempo's planned seconds by "KEY|day" (may be nil).
func TicketGroups(logs []core.Worklog, inTempo map[string]bool, planned map[string]int) []TicketGroup {
	byKey := map[string][]core.Worklog{}
	keys := map[string]*string{}
	for _, w := range logs {
		if w.Duration <= 0 {
			continue
		}
		k := "\x00" // "No issue"
		if w.IssueKey != nil {
			k = *w.IssueKey
		}
		byKey[k] = append(byKey[k], w)
		keys[k] = w.IssueKey
	}
	groups := []TicketGroup{}
	for k, list := range byKey {
		g := TicketGroup{Key: keys[k]}
		for day, items := range ByDay(list) {
			td := TicketDay{Key: g.Key, Day: day, Seconds: core.TotalDuration(items), Entered: true}
			var texts []string
			for _, w := range items {
				td.IDs = append(td.IDs, w.ID)
				if t := w.TempoText(); t != "" && !slices.Contains(texts, t) {
					texts = append(texts, t)
				}
				if w.Status != core.StatusExported {
					td.Entered = false
				}
			}
			if g.Key != nil && inTempo[*g.Key+"|"+day] {
				td.Entered, td.Found = true, true
			}
			if g.Key != nil {
				td.Planned = planned[*g.Key+"|"+day]
			}
			td.Description = strings.Join(texts, "; ")
			g.Days = append(g.Days, td)
		}
		slices.SortFunc(g.Days, func(a, b TicketDay) int { return compare(a.Day, b.Day) })
		for _, d := range g.Days {
			g.Seconds += d.Seconds
			if d.Entered {
				g.EnteredDays++
				g.EnteredSeconds += d.Seconds
			}
		}
		g.RemainingSeconds = g.Seconds - g.EnteredSeconds
		g.Done = g.Key != nil && g.EnteredDays == len(g.Days)
		g.Text = ticketText(g)
		groups = append(groups, g)
	}
	slices.SortFunc(groups, func(a, b TicketGroup) int {
		if (a.Key == nil) != (b.Key == nil) {
			if a.Key == nil {
				return -1
			}
			return 1
		}
		if a.Seconds != b.Seconds {
			return b.Seconds - a.Seconds
		}
		return compare(deref(a.Key), deref(b.Key))
	})
	return groups
}

func ticketText(g TicketGroup) string {
	lines := []string{deref(g.Key)}
	if g.Key == nil {
		lines[0] = "No issue"
	}
	for _, d := range g.Days {
		lines = append(lines, fmt.Sprintf("%s  %-7s  %s", d.Day, core.FormatDuration(d.Seconds), d.Description))
	}
	return strings.Join(lines, "\n")
}

// TempoProgressOf counts ticket-days in Tempo across the groups.
func TempoProgressOf(groups []TicketGroup) TempoProgress {
	var p TempoProgress
	for _, g := range groups {
		if g.Key != nil {
			p.Entered += g.EnteredDays
			p.Total += len(g.Days)
		}
	}
	return p
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// PlanIssue is the ticket Tempo planned the most time for on the day ("KEY|day" keys; the first key wins a tie),
// suggested for the day's entries without a ticket. Nil when nothing is planned.
func PlanIssue(planned map[string]int, day string) *string {
	best, most := "", 0
	for ref, seconds := range planned {
		key, d, ok := strings.Cut(ref, "|")
		if !ok || d != day || seconds <= 0 {
			continue
		}
		if seconds > most || seconds == most && key < best {
			best, most = key, seconds
		}
	}
	if best == "" {
		return nil
	}
	return &best
}
