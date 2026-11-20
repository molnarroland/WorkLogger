package view

import (
	"reflect"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

var days = ts.Days

func log(day, hm string, seconds int, key string, opts ...func(*core.Worklog)) core.Worklog {
	w := core.Worklog{Day: day, Start: ts.At(day, hm), Duration: seconds, Source: core.SourceGit, Status: core.StatusDraft}
	if key != "" {
		w.IssueKey = core.Ptr(key)
	}
	for _, o := range opts {
		o(&w)
	}
	return w
}

func status(s core.Status) func(*core.Worklog) { return func(w *core.Worklog) { w.Status = s } }
func source(s core.Source) func(*core.Worklog) { return func(w *core.Worklog) { w.Source = s } }
func text(d string) func(*core.Worklog)        { return func(w *core.Worklog) { w.Description = d } }

func eq[T any](t *testing.T, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
}

func TestTriageDay(t *testing.T) {
	const today = "2026-10-30"
	full := func(d string) DayInfo { return DayInfo{Day: d, Expected: 8 * 3600} }
	cases := []struct {
		name string
		info DayInfo
		logs []core.Worklog
		want DayState
	}{
		{"flags a past workday with nothing logged as empty", full("2026-10-29"), nil, StateEmpty},
		{"leaves a future workday alone", full("2026-10-31"), nil, StateUpcoming},
		{"flags a short day as partial", full("2026-10-28"), []core.Worklog{log("2026-10-28", "08:00", 5*3600, "PROJ-1")}, StatePartial},
		{
			"puts a missing ticket before everything else, even on a full day", full("2026-10-27"),
			[]core.Worklog{log("2026-10-27", "08:00", 4*3600, "PROJ-1"), log("2026-10-27", "12:00", 4*3600, "")},
			StateNoTicket,
		},
		{"counts a full day with drafts as draft", full("2026-10-26"), []core.Worklog{log("2026-10-26", "08:00", 8*3600, "PROJ-1")}, StateDraft},
		{
			"counts a reviewed full day as done", full("2026-10-26"),
			[]core.Worklog{log("2026-10-26", "08:00", 8*3600, "PROJ-1", status(core.StatusReviewed))},
			StateDone,
		},
		{"treats leave as nothing to do", DayInfo{Day: "2026-10-09", Expected: 8 * 3600, Absence: true}, nil, StateAbsence},
		{"treats weekends and holidays as nothing to do", DayInfo{Day: "2026-10-31"}, nil, StateFree},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { eq(t, TriageDay(c.info, c.logs, today).State, c.want) })
	}
	d := TriageDay(full("2026-10-27"), cases[3].logs, today)
	eq(t, [2]int{d.Missing, d.Logged}, [2]int{1, 8 * 3600})
}

func sampleOctober() Month {
	info, logs := ts.SampleMonth("2026-10")
	var in []DayInfo
	for _, d := range info {
		in = append(in, DayInfo(d))
	}
	return BuildMonth(MonthInput{Month: "2026-10", Today: "2026-10-30", Days: days, Info: in, Worklogs: logs})
}

func TestTriageOfTheSampleOctober(t *testing.T) {
	m := sampleOctober()
	t.Run("needs attention on the 27th (no ticket) and the empty 29th–30th", func(t *testing.T) {
		eq(t, m.Attention, []string{"2026-10-27", "2026-10-29", "2026-10-30"})
		for _, d := range m.Days {
			if d.Day == "2026-10-27" {
				eq(t, d.State, StateNoTicket)
			}
		}
	})
	t.Run("has the second half of the month as drafts, attention first in the queue", func(t *testing.T) {
		if !contains(m.Drafts, "2026-10-16") || contains(m.Drafts, "2026-10-15") {
			t.Errorf("drafts: %v", m.Drafts)
		}
		eq(t, m.Queue[:3], []string{"2026-10-27", "2026-10-29", "2026-10-30"})
	})
	t.Run("sums up the month and lists what is left in order", func(t *testing.T) {
		s := m.Summary
		eq(t, s.Behind, 16*3600)
		eq(t, len(s.Missing), 1)
		eq(t, s.Logged, s.InTempo+s.ToLog)
		eq(t, s.Short, max(0, s.Expected-s.Logged))
		if s.Short == 0 {
			t.Error("the sample month is short (the 29th and 30th are empty)")
		}
		var got [][2]string
		for _, x := range m.Todos {
			got = append(got, [2]string{string(x.Kind), deref(x.Day)})
		}
		eq(t, got, [][2]string{{"fillDays", "2026-10-29"}, {"addTickets", "2026-10-27"}, {"reviewDrafts", "2026-10-16"}, {"logInTempo", ""}})
	})
	t.Run("knows its grid, neighbours and the queue's next day", func(t *testing.T) {
		eq(t, [4]string{m.From, m.To, m.Prev, m.Next}, [4]string{"2026-10-01", "2026-10-31", "2026-09", "2026-11"})
		eq(t, m.CurrentMonth, "2026-10")
		eq(t, len(m.Weeks), 5)
		for _, d := range m.Days {
			if d.Day == "2026-10-30" {
				eq(t, deref(d.Next), "2026-10-27") // nothing later is queued: wraps to the front
			}
		}
	})
	t.Run("counts the entries without a ticket for the status bar", func(t *testing.T) { eq(t, m.NeedsTicket, 1) })
	t.Run("opens the Days view on the first queued day, the Week view on today", func(t *testing.T) {
		eq(t, [2]string{m.OpenDay, m.AnchorDay}, [2]string{"2026-10-27", "2026-10-30"})
		eq(t, [2]string{m.Days[0].PrevDay, m.Days[30].NextDay}, [2]string{"2026-09-30", "2026-11-01"})
	})
	t.Run("opens a month without today and without work on its first day", func(t *testing.T) {
		n := BuildMonth(MonthInput{Month: "2026-12", Today: "2026-10-30", Days: days})
		eq(t, [2]string{n.OpenDay, n.AnchorDay}, [2]string{"2026-12-01", "2026-12-01"})
	})
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestNextInQueue(t *testing.T) {
	q := []string{"a1", "b2", "c3"}
	eq(t, deref(NextInQueue(q, "b2")), "c3")
	eq(t, deref(NextInQueue(q, "c3")), "a1")
	eq(t, NextInQueue([]string{"b2"}, "b2"), (*string)(nil))
	eq(t, deref(NextInQueue([]string{"a1", "c3"}, "b2")), "c3")
}

func TestWeeksRunMondayToSundayAcrossMonthEnds(t *testing.T) {
	eq(t, WeekOf(days, "2026-10-29"), []string{"2026-10-26", "2026-10-27", "2026-10-28", "2026-10-29", "2026-10-30", "2026-10-31", "2026-11-01"})
	eq(t, days.Shift("2026-10-31", 1), "2026-11-01")
}

func TestWeeksOfMonth(t *testing.T) {
	w := WeeksOfMonth(days, "2026-10")
	eq(t, len(w), 5)
	eq(t, w[0][0], "2026-09-28")
	eq(t, w[4][6], "2026-11-01")
	for _, week := range w {
		eq(t, len(week), 7)
	}
}

func TestMonth(t *testing.T) {
	t.Run("shifts across year boundaries", func(t *testing.T) {
		eq(t, ShiftMonth(days, "2026-12", 1), "2027-01")
		eq(t, ShiftMonth(days, "2026-01", -1), "2025-12")
		eq(t, ShiftMonth(days, "2026-10", 0), "2026-10")
	})
	t.Run("knows the last day, leap years included", func(t *testing.T) {
		_, to, _ := days.Month("2028-02")
		eq(t, to, "2028-02-29")
		_, to, _ = days.Month("2026-02")
		eq(t, to, "2026-02-28")
	})
}

func TestSummaries(t *testing.T) {
	const day = "2026-03-03"
	logs := []core.Worklog{
		log(day, "09:00", 900, "INT-1", source(core.SourceMeeting)),
		log(day, "09:15", 7200, "PROJ-1"),
		log(day, "11:15", 1800, ""),
		log(day, "11:45", 0, "", source(core.SourceManual)),
	}
	t.Run("totals by source, largest first, without empty sources", func(t *testing.T) {
		eq(t, TotalsBySource(logs), []SourceTotal{{core.SourceGit, 9000}, {core.SourceMeeting, 900}})
	})
	t.Run("draws time without a ticket last", func(t *testing.T) {
		eq(t, BarParts(logs), []Part{{"git", 7200}, {"meeting", 900}, {PartNoTicket, 1800}})
	})
}

func TestDayProgress(t *testing.T) {
	cases := []struct {
		logged, expected int
		absence, future  bool
		want             Progress
	}{
		{0, 0, true, false, ProgressAbsence},
		{0, 0, false, false, ProgressFree},
		{3600, 0, false, false, ProgressExtra},
		{28800, 28800, false, false, ProgressComplete},
		{30000, 28800, false, true, ProgressComplete},
		{0, 28800, false, true, ProgressUpcoming},
		{0, 28800, false, false, ProgressEmpty},
		{3600, 28800, false, false, ProgressPartial},
	}
	for _, c := range cases {
		eq(t, DayProgress(c.logged, c.expected, c.absence, c.future), c.want)
	}
}

func TestTicketGroups(t *testing.T) {
	m := func(day, hm string, minutes int, key, description string, st core.Status) core.Worklog {
		return log(day, hm, minutes*60, key, text(description), status(st), source(core.SourceMeeting))
	}
	t.Run(`makes one row per ticket-day, "No issue" first`, func(t *testing.T) {
		logs := []core.Worklog{
			m("2026-03-03", "09:00", 15, "OPS-273", "Standup", core.StatusDraft),
			m("2026-03-03", "13:00", 30, "OPS-273", "Planning", core.StatusDraft),
			m("2026-03-04", "09:00", 15, "OPS-273", "Standup", core.StatusExported),
			m("2026-03-03", "10:00", 240, "PROJ-1", "Coding", core.StatusDraft),
			m("2026-03-03", "15:00", 60, "", "Meetup", core.StatusDraft),
			m("2026-03-03", "16:00", 0, "PROJ-1", "Empty", core.StatusDraft),
		}
		groups := TicketGroups(logs, nil, nil)
		eq(t, []string{deref(groups[0].Key), deref(groups[1].Key), deref(groups[2].Key)}, []string{"", "PROJ-1", "OPS-273"})
		ops := groups[2]
		eq(t, []string{ops.Days[0].Day, ops.Days[1].Day}, []string{"2026-03-03", "2026-03-04"})
		eq(t, ops.Days[0].Seconds, 45*60)
		eq(t, ops.Days[0].Description, "Standup; Planning")
		eq(t, []bool{ops.Days[0].Entered, ops.Days[1].Entered}, []bool{false, true})
		eq(t, ops.EnteredDays, 1)
		eq(t, [2]int{ops.EnteredSeconds, ops.RemainingSeconds}, [2]int{15 * 60, 45 * 60})
		eq(t, ops.Done, false)
		eq(t, ops.Text, "OPS-273\n2026-03-03  45m      Standup; Planning\n2026-03-04  15m      Standup")
		eq(t, TempoProgressOf(groups), TempoProgress{Entered: 1, Total: 3})
	})
	t.Run("counts ticket-days Tempo already has as entered", func(t *testing.T) {
		g := TicketGroups([]core.Worklog{log("2026-03-03", "09:00", 600, "PROJ-1")}, map[string]bool{"PROJ-1|2026-03-03": true}, nil)
		eq(t, g[0].Days[0].Entered, true)
		eq(t, g[0].Days[0].Found, true)
		eq(t, g[0].Done, true)
	})
	t.Run("copies the Tempo comment rather than the generated description", func(t *testing.T) {
		w := log("2026-03-02", "09:00", 3600, "PROJ-1", text("abc fix"))
		w.Comment = "Billing fix"
		eq(t, TicketGroups([]core.Worklog{w}, nil, nil)[0].Days[0].Description, "Billing fix")
	})
}

func TestPeriodIndex(t *testing.T) {
	p := []core.ProjectPeriod{{From: "2026-10-05", To: "2026-10-16"}, {From: "2026-10-12", To: "2026-10-23"}}
	for day, want := range map[string]int{"2026-10-04": -1, "2026-10-05": 0, "2026-10-12": 1, "2026-10-23": 1, "2026-10-24": -1} {
		eq(t, PeriodIndex(p, day), want)
	}
}

func TestEntryFlags(t *testing.T) {
	sent := log("2026-03-03", "09:30", 600, "PROJ-1", status(core.StatusExported))
	sent.TempoID = core.Ptr[int64](7)
	read := log("2026-03-04", "10:00", 600, "PROJ-2", status(core.StatusReviewed))
	draft := log("2026-03-05", "11:15", 600, "")
	in := map[string]bool{"PROJ-2|2026-03-04": true}

	e := NewEntry(sent, in, days)
	eq(t, [3]bool{e.InTempo, e.CanDelete, e.CanMove}, [3]bool{true, false, false})
	eq(t, e.StartMinute, 9*60+30)
	e = NewEntry(read, in, days)
	eq(t, [3]bool{e.InTempo, e.CanDelete, e.CanMove}, [3]bool{true, true, true})
	eq(t, e.EffectiveStatus, core.StatusExported)
	e = NewEntry(draft, in, days)
	eq(t, [3]bool{e.NeedsIssue, e.NeedsComment, e.InTempo}, [3]bool{true, true, false})
}

func TestWeek(t *testing.T) {
	info, logs := ts.SampleMonth("2026-10")
	var in []DayInfo
	for _, d := range info {
		in = append(in, DayInfo(d))
	}
	t.Run("runs Monday to Sunday across the month end, the Thursday deciding its month", func(t *testing.T) {
		w := BuildWeek(WeekInput{Day: "2026-10-29", Today: "2026-10-30", Days: days, Info: in, Worklogs: logs})
		eq(t, []string{w.Days[0].Day, w.Days[6].Day}, []string{"2026-10-26", "2026-11-01"})
		eq(t, w.Month, "2026-10")
		eq(t, [2]string{w.Prev, w.Next}, [2]string{"2026-10-22", "2026-11-05"})
		eq(t, w.Expected, 5*8*3600)
		eq(t, w.Days[1].State, StateNoTicket)
		eq(t, w.Days[6].State, StateFree) // a day outside the info: nothing expected
	})
	t.Run("belongs to the next month when its Thursday does", func(t *testing.T) {
		eq(t, BuildWeek(WeekInput{Day: "2026-12-30", Today: "2026-10-30", Days: days}).Month, "2026-12")
		eq(t, BuildWeek(WeekInput{Day: "2026-09-28", Today: "2026-10-30", Days: days}).Month, "2026-10")
	})
}

func TestReport(t *testing.T) {
	logs := []core.Worklog{
		log("2026-03-02", "09:00", 3600, "PROJ-1"), // week 10
		log("2026-03-06", "09:00", 7200, "PROJ-2", source(core.SourceMeeting)),
		log("2026-03-09", "09:00", 1800, ""), // week 11
		log("2026-04-01", "09:00", 900, "PROJ-1"),
	}
	ids := func(g []ReportGroup) (out []string) {
		for _, x := range g {
			out = append(out, x.ID)
		}
		return out
	}
	t.Run("groups by ISO week", func(t *testing.T) {
		g := Report(days, logs, PeriodWeek)
		eq(t, ids(g), []string{"2026-W10", "2026-W11", "2026-W14"})
		eq(t, []int{g[0].Total, g[1].Total, g[2].Total}, []int{10800, 1800, 900})
		eq(t, []string{deref(g[0].ByIssue[0].Issue), deref(g[0].ByIssue[1].Issue)}, []string{"PROJ-2", "PROJ-1"})
		eq(t, g[0].BySource, []SourceTotal{{core.SourceGit, 3600}, {core.SourceMeeting, 7200}})
		eq(t, g[1].ByIssue, []IssueTotal{{nil, 1800}})
		eq(t, []string{g[0].Start, g[2].Start}, []string{"2026-03-02", "2026-03-30"})
	})
	t.Run("groups by day and by month", func(t *testing.T) {
		eq(t, ids(Report(days, logs, PeriodDay)), []string{"2026-03-02", "2026-03-06", "2026-03-09", "2026-04-01"})
		m := Report(days, logs, PeriodMonth)
		eq(t, ids(m), []string{"2026-03", "2026-04"})
		eq(t, []int{m[0].Total, m[1].Total}, []int{12600, 900})
		eq(t, m[1].Start, "2026-04-01")
	})
	t.Run("has no groups without worklogs", func(t *testing.T) { eq(t, Report(days, nil, PeriodWeek), []ReportGroup{}) })
	t.Run("numbers ISO weeks across the year boundary", func(t *testing.T) {
		for day, want := range map[string]string{"2026-01-01": "2026-W01", "2027-01-01": "2026-W53", "2026-10-26": "2026-W44"} {
			id, _ := reportKey(days, day, PeriodWeek)
			eq(t, id, want)
		}
	})
}

func TestSessionDays(t *testing.T) {
	const day = "2026-03-03"
	s := func(from, to string, kind core.SessionKind) core.WorkSession {
		return core.WorkSession{Day: day, Start: ts.At(day, from), End: ts.At(day, to), Kind: kind}
	}
	t.Run("takes breaks out of work sessions", func(t *testing.T) {
		eq(t, NetSeconds([]core.WorkSession{s("08:00", "16:30", core.SessionWork), s("12:00", "12:30", core.SessionPause)}), 8*3600)
	})
	t.Run("adds several work sessions", func(t *testing.T) {
		eq(t, NetSeconds([]core.WorkSession{s("08:00", "12:00", core.SessionWork), s("13:00", "17:00", core.SessionWork)}), 8*3600)
	})
	t.Run("is zero without work", func(t *testing.T) {
		eq(t, NetSeconds([]core.WorkSession{s("12:00", "12:30", core.SessionPause)}), 0)
	})
	t.Run("flags a day whose net differs from HR's by more than a minute", func(t *testing.T) {
		got := SessionDays(
			[]core.WorkSession{s("13:00", "17:40", core.SessionWork), s("08:30", "12:00", core.SessionWork)},
			[]SessionRecord{{Day: day, Status: "workday", Label: "Munkanap", Reported: 8 * 3600}, {Day: "2026-03-02", Status: "leave", Label: "Szabadság", Reported: 8 * 3600}},
		)
		eq(t, len(got), 2)
		eq(t, [3]any{got[0].Day, got[0].Status, got[0].Mismatch}, [3]any{"2026-03-02", "leave", false})
		eq(t, [3]any{got[1].Net, got[1].Mismatch, got[1].Sessions[0].Start.Hour()}, [3]any{8*3600 + 10*60, true, 8})
	})
}

func TestSetupSteps(t *testing.T) {
	ids := func(steps []SetupStep) (out []string) {
		for _, s := range steps {
			out = append(out, s.ID)
		}
		return out
	}
	t.Run("asks only for what the chosen modules need, in order", func(t *testing.T) {
		steps := SetupSteps(core.Settings{Modules: []core.Module{core.ModuleChats, core.ModuleGit, core.ModuleWorkTimes, core.ModuleCalendar}})
		eq(t, ids(steps), []string{"repositories", "google", "calendar", "hrmaster", "jira", "tempo"})
		for _, s := range steps {
			eq(t, s.Done, false)
		}
		eq(t, SetupSteps(core.Settings{TempoByHand: true}), []SetupStep{})
	})
	t.Run("marks a step done once its profile is filled in", func(t *testing.T) {
		steps := SetupSteps(core.Settings{
			Modules: []core.Module{core.ModuleChats, core.ModuleCalendar, core.ModuleWorkTimes}, TempoByHand: true,
			Profiles: []core.Profile{{Kind: core.ProfileGoogleChat, Email: "me@x.hu"}, {Kind: core.ProfileHRmaster, URL: " "}},
		})
		done := map[string]bool{}
		for _, s := range steps {
			done[s.ID] = s.Done
		}
		eq(t, done, map[string]bool{"google": true, "calendar": true, "hrmaster": false})
	})
	t.Run("says where each step is done", func(t *testing.T) {
		steps := SetupSteps(core.Settings{Modules: []core.Module{core.ModuleGit, core.ModuleWorkTimes, core.ModuleCalendar}, TempoByHand: true})
		eq(t, [3]string{steps[0].Page, steps[1].Page + "/" + steps[1].Tab, string(steps[2].Profile)}, [3]string{"repositories", "settings/calendars", "hrmaster"})
	})
}

func TestExportChoices(t *testing.T) {
	logs := []core.Worklog{
		log("2026-03-02", "09:00", 3600, "PROJ-1"),
		log("2026-03-02", "10:00", 1800, "PROJ-1", status(core.StatusReviewed)),
		log("2026-03-02", "11:00", 900, "PROJ-2", status(core.StatusExported)),
		log("2026-03-02", "12:00", 600, ""),
		log("2026-03-02", "13:00", 0, "PROJ-3"),
	}
	for i := range logs {
		logs[i].ID = int64(i + 1)
	}
	got := map[[2]bool]ExportChoice{}
	for _, c := range ExportChoices(logs) {
		got[[2]bool{c.OnlyReviewed, c.IncludeExported}] = c
	}
	eq(t, len(got), 4)
	eq(t, got[[2]bool{false, false}], ExportChoice{IDs: []int64{1, 2}, Seconds: 5400, Missing: 1})
	eq(t, got[[2]bool{false, true}].IDs, []int64{1, 2, 3})
	eq(t, got[[2]bool{true, false}], ExportChoice{OnlyReviewed: true, IDs: []int64{2}, Seconds: 1800})
}

func TestEvidence(t *testing.T) {
	row := func(title string, key string, skipped bool) MeetingRow {
		r := MeetingRow{Meeting: core.Meeting{Title: title, Skipped: skipped}}
		if key != "" {
			r.IssueKey = core.Ptr(key)
		}
		return r
	}
	t.Run("suggests the most frequent untagged title, from twice on", func(t *testing.T) {
		rows := []MeetingRow{row("Retro", "", false), row("1:1", "", false), row("Retro", "", false), row("1:1", "", false), row("1:1", "", true), row("Standup", "INT-1", false)}
		eq(t, MeetingSuggestion(rows), &Suggestion{"Retro", 2})
		eq(t, MeetingSuggestion(rows[:2]), (*Suggestion)(nil))
	})
	t.Run("groups by day with each day's time", func(t *testing.T) {
		type c struct {
			day string
			s   int
		}
		g := GroupByDay([]c{{"2026-03-04", 60}, {"2026-03-03", 120}, {"2026-03-04", 30}}, func(x c) string { return x.day }, func(x c) int { return x.s })
		eq(t, []string{g[0].Day, g[1].Day}, []string{"2026-03-03", "2026-03-04"})
		eq(t, []int{g[0].Seconds, g[1].Seconds, len(g[1].Items)}, []int{120, 90, 2})
	})
}

func TestSourceFocus(t *testing.T) {
	repo := int64(7)
	cases := []struct {
		w    core.Worklog
		want *Focus
	}{
		{core.Worklog{Source: core.SourceGit, Day: "2026-10-27", RepoID: &repo, SourceRef: "abc,def"}, &Focus{Pane: "repositories", ID: "7"}},
		{core.Worklog{Source: core.SourceGit, Day: "2026-10-27"}, nil},
		{core.Worklog{Source: core.SourceMeeting, Day: "2026-10-27", SourceRef: "evt-1"}, &Focus{Pane: "meetings", Day: "2026-10-27", ID: "evt-1"}},
		{core.Worklog{Source: core.SourceChat, Day: "2026-10-27", SourceRef: "chat:2026-10-27|dm/mira"}, &Focus{Pane: "chats", Day: "2026-10-27", ID: "2026-10-27|dm/mira"}},
		{core.Worklog{Source: core.SourceAbsence, Day: "2026-10-27", SourceRef: "absence:2026-10-27"}, &Focus{Pane: "sessions", Day: "2026-10-27"}},
		{core.Worklog{Source: core.SourcePeriod, Day: "2026-10-27"}, &Focus{Pane: "periods", Day: "2026-10-27"}},
		{core.Worklog{Source: core.SourceManual, Day: "2026-10-27"}, nil},
	}
	for _, c := range cases {
		if got := SourceFocus(c.w); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.w.Source, got, c.want)
		}
	}
}
