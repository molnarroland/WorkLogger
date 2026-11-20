package view

import (
	"slices"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Week is Monday to Sunday around a day: each day's totals and states, and the entries.
type Week struct {
	// Day is the day asked for; Days run Monday to Sunday.
	Day     string
	Days    []Day
	Entries []Entry
	Logged  int
	// Expected is the week's target.
	Expected int
	// Prev and Next are the same weekday a week earlier and later.
	Prev, Next string
	// Month is the week's month as ISO weeks count it: its Thursday's.
	Month string
}

// WeekInput: Info has the week's seven days; Worklogs are the week's entries.
type WeekInput struct {
	Day      string
	Today    string
	Days     core.Days
	Info     []DayInfo
	Worklogs []core.Worklog
	InTempo  map[string]bool
	Periods  []core.ProjectPeriod
	// Planned is Tempo Planner's time by "KEY|day" (seconds; may be nil).
	Planned map[string]int
}

// BuildWeek computes the week view.
func BuildWeek(in WeekInput) Week {
	days := WeekOf(in.Days, in.Day)
	w := Week{
		Day: in.Day, Days: []Day{}, Entries: []Entry{}, Prev: in.Days.Shift(in.Day, -7),
		Next: in.Days.Shift(in.Day, 7), Month: days[3][:7],
	}
	logs := slices.Clone(in.Worklogs)
	slices.SortStableFunc(logs, func(a, b core.Worklog) int { return a.Start.Compare(b.Start) })
	for _, l := range logs {
		e := NewEntry(l, in.InTempo, in.Days)
		if e.NeedsIssue {
			e.PlanIssue = PlanIssue(in.Planned, l.Day)
		}
		w.Entries = append(w.Entries, e)
	}
	info := map[string]DayInfo{}
	for _, d := range in.Info {
		info[d.Day] = d
	}
	byDay := ByDay(logs)
	for _, day := range days {
		i, ok := info[day]
		if !ok {
			i = DayInfo{Day: day}
		}
		d := TriageDay(i, byDay[day], in.Today)
		d.PeriodIndex = PeriodIndex(in.Periods, day)
		d.PrevDay, d.NextDay = in.Days.Shift(day, -1), in.Days.Shift(day, 1)
		w.Days = append(w.Days, d)
		w.Logged += d.Logged
		w.Expected += d.Expected
	}
	return w
}
