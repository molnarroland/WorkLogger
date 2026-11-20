// Package view turns stored data into what the screens show: each day's state, the month's to-dos, ticket
// groups, totals. Pure: inputs are core types plus "today", outputs are plain structs. The app package loads,
// calls a view, and returns the result; the frontend only formats and lays it out.
package view

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// DayState is a day's place in the review triage.
//   - empty: a past workday with nothing logged
//   - partial: a past workday logged short of its target
//   - noTicket: has time that can't go to Tempo
//   - draft: full, with entries still to review
//   - done: full and reviewed (or in Tempo)
//   - upcoming, free, absence: nothing to do yet / at all
type DayState string

const (
	StateEmpty    DayState = "empty"
	StatePartial  DayState = "partial"
	StateNoTicket DayState = "noTicket"
	StateDraft    DayState = "draft"
	StateDone     DayState = "done"
	StateUpcoming DayState = "upcoming"
	StateFree     DayState = "free"
	StateAbsence  DayState = "absence"
)

// Progress is how full a day is against its target (the day cards' colour).
type Progress string

const (
	ProgressAbsence  Progress = "absence"
	ProgressFree     Progress = "free"
	ProgressExtra    Progress = "extra"
	ProgressComplete Progress = "complete"
	ProgressUpcoming Progress = "upcoming"
	ProgressEmpty    Progress = "empty"
	ProgressPartial  Progress = "partial"
)

// PartNoTicket is the bar part for time without a ticket; every other part is named after its source.
const PartNoTicket = "noTicket"

// Part is one segment of a stacked bar: a source's time, or the time without a ticket.
type Part struct {
	Kind    string `json:"kind"`
	Seconds int    `json:"seconds"`
}

// DayInfo is what a day should add up to and what HR says about it.
type DayInfo struct {
	Day      string
	Expected int
	Label    string
	Absence  bool
}

// Entry is a worklog with what the screens need to know about it.
type Entry struct {
	core.Worklog
	NeedsIssue   bool
	NeedsComment bool
	TempoText    string
	// InTempo: exported, or Tempo already has its ticket-day (read-back).
	InTempo bool
	// EffectiveStatus is exported when InTempo, else the stored status.
	EffectiveStatus core.Status
	CanDelete       bool
	CanMove         bool
	StartMinute     int
	// Focus is where the entry's source can be seen; nil for an entry made by hand.
	Focus *Focus
	// PlanIssue is the ticket Tempo planned for the day, suggested when the entry needs one (else nil).
	PlanIssue *string
}

// Focus points at a page and the thing on it: Pane is the page's id, Day narrows it to a day, ID picks one item
// (a repository id, a calendar event id, a chat-day id). Empty fields mean the page as a whole.
type Focus struct {
	Pane string `json:"pane"`
	Day  string `json:"day"`
	ID   string `json:"id"`
}

// SourceFocus is where a worklog came from: the repository of its commits, its meeting or chat on its day, the
// HR record of an absence, or the project period that booked the day.
func SourceFocus(w core.Worklog) *Focus {
	switch w.Source {
	case core.SourceGit:
		if w.RepoID == nil {
			return nil
		}
		return &Focus{Pane: "repositories", ID: strconv.FormatInt(*w.RepoID, 10)}
	case core.SourceMeeting:
		return &Focus{Pane: "meetings", Day: w.Day, ID: w.SourceRef}
	case core.SourceChat:
		return &Focus{Pane: "chats", Day: w.Day, ID: strings.TrimPrefix(w.SourceRef, "chat:")}
	case core.SourceAbsence:
		return &Focus{Pane: "sessions", Day: w.Day}
	case core.SourcePeriod:
		return &Focus{Pane: "periods", Day: w.Day}
	}
	return nil
}

// NewEntry derives an entry's flags. inTempo holds "KEY|day" ticket-days Tempo already has.
func NewEntry(w core.Worklog, inTempo map[string]bool, days core.Days) Entry {
	e := Entry{
		Worklog:      w,
		NeedsIssue:   w.NeedsIssue(),
		NeedsComment: NeedsComment(w),
		TempoText:    w.TempoText(),
		InTempo:      isInTempo(w, inTempo),
		CanDelete:    w.TempoID == nil,
		CanMove:      w.Status != core.StatusExported,
		Focus:        SourceFocus(w),
	}
	e.EffectiveStatus = w.Status
	if e.InTempo {
		e.EffectiveStatus = core.StatusExported
	}
	local := w.Start.In(days.Loc)
	e.StartMinute = local.Hour()*60 + local.Minute()
	return e
}

// NeedsComment: a draft can't be marked reviewed until it has its comment for Tempo.
func NeedsComment(w core.Worklog) bool {
	return w.Status == core.StatusDraft && strings.TrimSpace(w.Comment) == ""
}

func isInTempo(w core.Worklog, inTempo map[string]bool) bool {
	return w.Status == core.StatusExported || (w.IssueKey != nil && inTempo[*w.IssueKey+"|"+w.Day])
}

// SourceTotal is the time logged from one source.
type SourceTotal struct {
	Source  core.Source `json:"source"`
	Seconds int         `json:"seconds"`
}

// TotalsBySource is the logged time per source, largest first; sources without time are left out.
func TotalsBySource(logs []core.Worklog) []SourceTotal {
	by := map[core.Source]int{}
	for _, w := range logs {
		by[w.Source] += w.Duration
	}
	var out []SourceTotal
	for s, n := range by {
		if n > 0 {
			out = append(out, SourceTotal{s, n})
		}
	}
	slices.SortFunc(out, func(a, b SourceTotal) int {
		if a.Seconds != b.Seconds {
			return b.Seconds - a.Seconds
		}
		return strings.Compare(string(a.Source), string(b.Source))
	})
	return out
}

// SourceParts is the logged time per source as bar parts.
func SourceParts(logs []core.Worklog) []Part {
	out := []Part{}
	for _, s := range TotalsBySource(logs) {
		out = append(out, Part{string(s.Source), s.Seconds})
	}
	return out
}

// BarParts is each source's ticketed time, then everything without a ticket last.
func BarParts(logs []core.Worklog) []Part {
	var ticketed, missing []core.Worklog
	for _, w := range logs {
		if w.NeedsIssue() {
			missing = append(missing, w)
		} else {
			ticketed = append(ticketed, w)
		}
	}
	parts := SourceParts(ticketed)
	if n := core.TotalDuration(missing); n > 0 {
		parts = append(parts, Part{PartNoTicket, n})
	}
	return parts
}

// DayProgress is how full a day is. future: the day is after today.
func DayProgress(logged, expected int, absence, future bool) Progress {
	switch {
	case absence:
		return ProgressAbsence
	case expected == 0 && logged > 0:
		return ProgressExtra
	case expected == 0:
		return ProgressFree
	case logged >= expected:
		return ProgressComplete
	case future:
		return ProgressUpcoming
	case logged == 0:
		return ProgressEmpty
	}
	return ProgressPartial
}

// PeriodIndex is the period that books a day, -1 for none; later periods win where they overlap.
func PeriodIndex(periods []core.ProjectPeriod, day string) int {
	for i := len(periods) - 1; i >= 0; i-- {
		if periods[i].Contains(day) {
			return i
		}
	}
	return -1
}

// WeeksOfMonth lays a "2006-01" month out in Monday-first weeks, padded with the neighbouring months' days.
func WeeksOfMonth(days core.Days, month string) [][]string {
	from, to, ok := days.Month(month)
	if !ok {
		return nil
	}
	start := days.Shift(from, -mondayIndex(days, from))
	end := days.Shift(to, 6-mondayIndex(days, to))
	var weeks [][]string
	for i, d := range days.Range(start, end) {
		if i%7 == 0 {
			weeks = append(weeks, nil)
		}
		weeks[len(weeks)-1] = append(weeks[len(weeks)-1], d)
	}
	return weeks
}

// WeekOf is Monday to Sunday of the week holding day.
func WeekOf(days core.Days, day string) []string {
	monday := days.Shift(day, -mondayIndex(days, day))
	return days.Range(monday, days.Shift(monday, 6))
}

// mondayIndex: 0 for Monday … 6 for Sunday.
func mondayIndex(days core.Days, day string) int { return (int(days.Date(day).Weekday()) + 6) % 7 }

// ShiftMonth moves a "2006-01" month by n months.
func ShiftMonth(days core.Days, month string, n int) string {
	t, err := time.ParseInLocation("2006-01", month, days.Loc)
	if err != nil {
		return month
	}
	return t.AddDate(0, n, 0).Format("2006-01")
}

// ByDay groups worklogs by day, each day's entries by start time.
func ByDay(logs []core.Worklog) map[string][]core.Worklog {
	out := map[string][]core.Worklog{}
	for _, w := range logs {
		out[w.Day] = append(out[w.Day], w)
	}
	for _, list := range out {
		slices.SortStableFunc(list, func(a, b core.Worklog) int { return a.Start.Compare(b.Start) })
	}
	return out
}
