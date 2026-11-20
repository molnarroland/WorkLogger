package view

import (
	"math"
	"slices"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// SessionRecord is HR's record of a day, its status already named for the screen (RecordStatus).
type SessionRecord struct {
	Day, Status, Label string
	// Reported is HR's worked time, seconds.
	Reported int
}

// SessionDay is one day of the Work sessions page.
type SessionDay struct {
	Day, Label, Status string
	// Sessions are by start time.
	Sessions []core.WorkSession
	// Net is the work sessions minus the breaks; Reported is HR's figure.
	Net, Reported int
	// Mismatch: both are known and differ by more than a minute.
	Mismatch bool
}

// RecordStatus names HR's day status for the screen: workday, restday, holiday or leave. An unknown status is a
// rest day on weekends and a workday otherwise.
func RecordStatus(st core.DayStatus, day string, d core.Days) string {
	switch st {
	case core.DayWorkday:
		return "workday"
	case core.DayRestDay:
		return "restday"
	case core.DayPublicHoliday:
		return "holiday"
	case core.DayLeave:
		return "leave"
	}
	if d.IsWeekend(day) {
		return "restday"
	}
	return "workday"
}

// SessionDays puts each day's sessions next to HR's record, oldest day first.
func SessionDays(sessions []core.WorkSession, records []SessionRecord) []SessionDay {
	byDay := map[string][]core.WorkSession{}
	for _, s := range sessions {
		byDay[s.Day] = append(byDay[s.Day], s)
	}
	rec := map[string]SessionRecord{}
	var keys []string
	for _, r := range records {
		rec[r.Day] = r
		keys = append(keys, r.Day)
	}
	for day := range byDay {
		if _, ok := rec[day]; !ok {
			keys = append(keys, day)
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	out := []SessionDay{}
	for _, day := range keys {
		list := slices.Clone(byDay[day])
		slices.SortStableFunc(list, func(a, b core.WorkSession) int { return a.Start.Compare(b.Start) })
		r, ok := rec[day]
		d := SessionDay{Day: day, Sessions: list, Status: "workday", Net: NetSeconds(list)}
		if list == nil {
			d.Sessions = []core.WorkSession{}
		}
		if ok {
			d.Label, d.Status, d.Reported = r.Label, r.Status, r.Reported
		}
		d.Mismatch = len(list) > 0 && d.Reported > 0 && math.Abs(float64(d.Reported-d.Net)) > 60
		out = append(out, d)
	}
	return out
}

// NetSeconds is the worked time of a day's sessions: work sessions minus breaks (core.NetWork).
func NetSeconds(sessions []core.WorkSession) int {
	return int(core.Total(core.NetWork(sessions)).Seconds())
}
