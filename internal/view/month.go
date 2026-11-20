package view

import (
	"slices"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Day is one day of the month with its totals and states.
type Day struct {
	Day      string
	Expected int
	Label    string
	Absence  bool
	Logged   int
	// Short is the target left: Expected minus Logged, never below zero.
	Short int
	// Missing counts entries without a ticket; NoComment drafts still without their comment for Tempo.
	Missing, NoComment, Entries, Drafts int
	State                               DayState
	Progress                            Progress
	// AcceptsWork: entries may be dropped on it (not leave or a holiday). MoveTarget: it is offered in
	// "Move to…" (a workday that takes work).
	AcceptsWork, MoveTarget bool
	// PeriodIndex is the project period booking it, -1 for none.
	PeriodIndex int
	Parts       []Part
	// Sources is the logged time per source, largest first (no-ticket time counted with its source).
	Sources []SourceTotal
	// Next is the day after it in the review queue, wrapping; nil when nothing else is queued.
	Next *string
	// PrevDay and NextDay are the calendar neighbours.
	PrevDay, NextDay string
}

// Summary is the month at a glance (Overview).
type Summary struct {
	Logged, Expected int
	// Short is what is still to log this month: Expected minus Logged, never below zero.
	Short int
	// Behind is the target left on past workdays (today included): what's late, not what's upcoming.
	Behind     int
	BehindDays []string
	InTempo    int
	// InTempoDays counts days whose entries are all in Tempo.
	InTempoDays int
	// ToLog is logged but not in Tempo yet (drafts and reviewed).
	ToLog     int
	DraftDays []string
	// Missing lists the entries without a ticket, oldest first; MissingDays their days.
	Missing        []int64
	MissingDays    []string
	MissingSeconds int
	// Parts is the logged time per source; BarParts the same with the time without a ticket split off, last.
	Parts, BarParts []Part
}

// TodoKind names one of the Overview's "What's left" rows.
type TodoKind string

const (
	TodoFillDays     TodoKind = "fillDays"
	TodoAddTickets   TodoKind = "addTickets"
	TodoReviewDrafts TodoKind = "reviewDrafts"
	TodoLogInTempo   TodoKind = "logInTempo"
)

// Todo is one row of "What's left". Day is where its action opens; nil means the By ticket page.
type Todo struct {
	Kind    TodoKind
	Count   int
	Seconds int
	Day     *string
}

// Month is everything the month's screens show.
type Month struct {
	Month, Today, CurrentMonth string
	From, To, Prev, Next       string
	// OpenDay is where the Days view opens: the first queued day, else today when it is in the month, else the
	// first. AnchorDay is the Week view's: today when in the month, else the first.
	OpenDay, AnchorDay string
	Weeks              [][]string
	Days               []Day
	Entries            []Entry
	Summary            Summary
	Todos              []Todo
	// Attention: past days short of time, and any day with an entry without a ticket. Drafts: full days with
	// drafts left. Queue: the order "Mark reviewed and go to next" walks, attention first.
	Attention, Drafts, Queue []string
	Tickets                  []TicketGroup
	TempoProgress            TempoProgress
	// Reports are the Totals page's groups for each period.
	Reports map[Period][]ReportGroup
	// Exports is what the export sheet sends for each setting of its switches.
	Exports     []ExportChoice
	NeedsTicket int
}

// MonthInput is what a month view is built from. Info has every day of the month; InTempo holds the
// "KEY|day" ticket-days Tempo already has.
type MonthInput struct {
	Month    string
	Today    string
	Days     core.Days
	Info     []DayInfo
	Worklogs []core.Worklog
	InTempo  map[string]bool
	Periods  []core.ProjectPeriod
	// Planned is Tempo Planner's time by "KEY|day" (seconds; may be nil).
	Planned map[string]int
}

// BuildMonth computes the month view.
func BuildMonth(in MonthInput) Month {
	from, to, _ := in.Days.Month(in.Month)
	m := Month{
		Month: in.Month, Today: in.Today, CurrentMonth: in.Today[:7], From: from, To: to,
		Prev: ShiftMonth(in.Days, in.Month, -1), Next: ShiftMonth(in.Days, in.Month, 1),
		Weeks: WeeksOfMonth(in.Days, in.Month), Days: []Day{}, Entries: []Entry{},
		Attention: []string{}, Drafts: []string{}, Queue: []string{},
	}
	logs := slices.Clone(in.Worklogs)
	slices.SortStableFunc(logs, func(a, b core.Worklog) int { return a.Start.Compare(b.Start) })
	for _, w := range logs {
		e := NewEntry(w, in.InTempo, in.Days)
		if e.NeedsIssue {
			m.NeedsTicket++
			e.PlanIssue = PlanIssue(in.Planned, w.Day)
		}
		m.Entries = append(m.Entries, e)
	}

	info := slices.Clone(in.Info)
	slices.SortFunc(info, func(a, b DayInfo) int { return compare(a.Day, b.Day) })
	byDay := ByDay(logs)
	for _, d := range info {
		day := TriageDay(d, byDay[d.Day], in.Today)
		day.PeriodIndex = PeriodIndex(in.Periods, d.Day)
		m.Days = append(m.Days, day)
		switch day.State {
		case StateEmpty, StatePartial, StateNoTicket:
			m.Attention = append(m.Attention, d.Day)
		case StateDraft:
			m.Drafts = append(m.Drafts, d.Day)
		}
	}
	m.Queue = append(append(m.Queue, m.Attention...), m.Drafts...)
	for i := range m.Days {
		m.Days[i].Next = NextInQueue(m.Queue, m.Days[i].Day)
		m.Days[i].PrevDay, m.Days[i].NextDay = in.Days.Shift(m.Days[i].Day, -1), in.Days.Shift(m.Days[i].Day, 1)
	}
	m.AnchorDay = from
	if from <= in.Today && in.Today <= to {
		m.AnchorDay = in.Today
	}
	m.OpenDay = m.AnchorDay
	if len(m.Queue) > 0 {
		m.OpenDay = m.Queue[0]
	}
	m.Summary = summarize(m.Days, logs, in.InTempo)
	m.Todos = Todos(m.Summary)
	m.Tickets = TicketGroups(logs, in.InTempo, in.Planned)
	m.TempoProgress = TempoProgressOf(m.Tickets)
	m.Exports = ExportChoices(logs)
	m.Reports = map[Period][]ReportGroup{}
	for _, p := range []Period{PeriodDay, PeriodWeek, PeriodMonth} {
		m.Reports[p] = Report(in.Days, logs, p)
	}
	return m
}

// TriageDay totals one day's entries (sorted by start) against its target and classifies it.
func TriageDay(info DayInfo, logs []core.Worklog, today string) Day {
	d := Day{
		Day: info.Day, Expected: info.Expected, Label: info.Label, Absence: info.Absence,
		Logged: core.TotalDuration(logs), Entries: len(logs), AcceptsWork: !info.Absence, PeriodIndex: -1,
		Parts: BarParts(logs), Sources: TotalsBySource(logs),
	}
	if d.Sources == nil {
		d.Sources = []SourceTotal{}
	}
	d.Short = max(0, d.Expected-d.Logged)
	d.MoveTarget = d.AcceptsWork && d.Expected > 0
	for _, w := range logs {
		if w.NeedsIssue() {
			d.Missing++
		}
		if NeedsComment(w) {
			d.NoComment++
		}
		if w.Status == core.StatusDraft {
			d.Drafts++
		}
	}
	d.State = stateOf(d, today)
	d.Progress = DayProgress(d.Logged, d.Expected, d.Absence, d.Day > today)
	return d
}

func stateOf(d Day, today string) DayState {
	switch {
	case d.Absence:
		return StateAbsence
	case d.Missing > 0:
		return StateNoTicket
	case d.Expected == 0 && d.Logged == 0:
		return StateFree
	case d.Logged < d.Expected && d.Day > today:
		return StateUpcoming
	case d.Logged < d.Expected && d.Logged == 0:
		return StateEmpty
	case d.Logged < d.Expected:
		return StatePartial
	case d.Drafts > 0:
		return StateDraft
	}
	return StateDone
}

// NextInQueue is the day after day in the queue, wrapping; nil when the queue is empty or only holds day.
func NextInQueue(queue []string, day string) *string {
	var first *string
	for _, d := range queue {
		if d == day {
			continue
		}
		if first == nil {
			first = core.Ptr(d)
		}
		if d > day {
			return core.Ptr(d)
		}
	}
	return first
}

func summarize(days []Day, logs []core.Worklog, inTempo map[string]bool) Summary {
	s := Summary{
		Logged: core.TotalDuration(logs), BehindDays: []string{}, DraftDays: []string{}, Missing: []int64{},
		MissingDays: []string{}, Parts: SourceParts(logs), BarParts: BarParts(logs),
	}
	for _, d := range days {
		s.Expected += d.Expected
		switch d.State {
		case StateEmpty, StatePartial:
			s.Behind += max(0, d.Expected-d.Logged)
			s.BehindDays = append(s.BehindDays, d.Day)
		case StateDraft:
			s.DraftDays = append(s.DraftDays, d.Day)
		}
	}
	allIn := map[string]bool{}
	for _, w := range logs { // sorted by start
		in := isInTempo(w, inTempo)
		if in {
			s.InTempo += w.Duration
		} else {
			s.ToLog += w.Duration
		}
		if seen, ok := allIn[w.Day]; !ok || seen {
			allIn[w.Day] = in
		}
		if w.NeedsIssue() {
			s.Missing = append(s.Missing, w.ID)
			s.MissingSeconds += w.Duration
			if !slices.Contains(s.MissingDays, w.Day) {
				s.MissingDays = append(s.MissingDays, w.Day)
			}
		}
	}
	for _, in := range allIn {
		if in {
			s.InTempoDays++
		}
	}
	s.Short = max(0, s.Expected-s.Logged)
	return s
}

// Todos is the Overview's "What's left", in the order the work gets done. Empty when the month is finished.
func Todos(s Summary) []Todo {
	todos := []Todo{}
	if len(s.BehindDays) > 0 {
		todos = append(todos, Todo{TodoFillDays, len(s.BehindDays), s.Behind, core.Ptr(s.BehindDays[0])})
	}
	if len(s.Missing) > 0 {
		todos = append(todos, Todo{TodoAddTickets, len(s.Missing), s.MissingSeconds, core.Ptr(s.MissingDays[0])})
	}
	if len(s.DraftDays) > 0 {
		todos = append(todos, Todo{TodoReviewDrafts, len(s.DraftDays), 0, core.Ptr(s.DraftDays[0])})
	}
	if s.ToLog > 0 {
		todos = append(todos, Todo{TodoLogInTempo, 0, s.ToLog, nil})
	}
	return todos
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
