package core_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Week used throughout: Mon 2026-03-02 … Fri 2026-03-06, Sat 03-07, Sun 03-08.

const g = 900 // 15 minutes

func p[T any](v T) *T { return &v }

func def() core.Settings { return core.DefaultSettings() }

func builder() core.Builder { return ts.Builder(def(), nil, nil) }

func TestAllocate(t *testing.T) {
	if got := core.Allocate(nil, p(28800), g); len(got) != 0 {
		t.Errorf("noWeightsGiveNoAllocations: %v", got)
	}
	cases := []struct {
		name    string
		weights []int
		target  *int
		want    []int
	}{
		{"withoutTargetEachEstimateIsRoundedToAtLeastOneUnit", []int{100, 1000, 2000}, nil, []int{900, 900, 1800}},
		{"splitsProportionally", []int{1, 1, 2}, p(28800), []int{7200, 7200, 14400}},
		{"everyBlockGetsAUnitWhenThereIsRoom", []int{1, 1000}, p(3600), []int{900, 2700}},
		{"targetBelowOneUnitGivesZeros/0", []int{10, 20}, p(0), []int{0, 0}},
		{"targetBelowOneUnitGivesZeros/600", []int{10, 20}, p(600), []int{0, 0}},
	}
	for _, c := range cases {
		if got := core.Allocate(c.weights, c.target, g); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	// alwaysSumsToTheTarget
	for _, w := range [][]int{{1, 1, 1}, {5, 3, 2}, {700, 1, 1, 1}, {1234, 4321}} {
		sum := 0
		for _, a := range core.Allocate(w, p(3600), g) {
			sum += a
			if a%g != 0 {
				t.Errorf("%v: %d is not whole units", w, a)
			}
		}
		if sum != 3600 {
			t.Errorf("%v sums to %d", w, sum)
		}
	}
}

func TestPlace(t *testing.T) {
	day := "2026-03-03"
	free := []core.Interval{ts.Interval(day, "09:00", "12:00"), ts.Interval(day, "13:00", "17:00")}
	cases := []struct {
		name       string
		seconds    int
		from       string
		free       []core.Interval
		start, end string
	}{
		{"fitsInsideTheFirstSlot", 7200, "09:00", free, "09:00", "11:00"},
		{"spillsOverTheLunchBreak", 7200, "11:00", free, "11:00", "14:00"},
		{"startsAtTheNextFreeSlot", 3600, "12:30", free, "13:00", "14:00"},
		{"overflowsPastTheLastWindow", 7200, "16:00", free, "16:00", "18:00"},
		{"withoutFreeTimeStartsAtTheCursor", 3600, "09:00", nil, "09:00", "10:00"},
	}
	for _, c := range cases {
		s, e := core.Place(c.seconds, ts.At(day, c.from), c.free)
		if ts.Days.HM(s) != c.start || ts.Days.HM(e) != c.end {
			t.Errorf("%s: got %s–%s", c.name, ts.Days.HM(s), ts.Days.HM(e))
		}
	}
}

func record(day string, status core.DayStatus, worked *int) *core.DayRecord {
	return &core.DayRecord{Day: day, Status: status, ReportedWorked: worked}
}

func TestDayRules(t *testing.T) {
	b := builder()
	if !b.IsWorkday("2026-03-02", nil) || b.IsWorkday("2026-03-07", nil) {
		t.Error("calendarDecidesWithoutHRData")
	}
	if !b.IsWorkday("2026-03-07", record("2026-03-07", core.DayWorkday, nil)) ||
		b.IsWorkday("2026-03-02", record("2026-03-02", core.DayPublicHoliday, nil)) ||
		!b.IsWorkday("2026-03-02", record("2026-03-02", core.DayOther, nil)) {
		t.Error("hrStatusOverridesTheCalendar")
	}

	s := def()
	s.RoundingMode = core.RoundingEstimate
	if ts.Builder(s, nil, nil).DailyTarget("2026-03-02", nil, nil, false) != nil {
		t.Error("estimateModeHasNoTarget")
	}
	windows := []core.Interval{ts.Interval("2026-03-02", "08:00", "15:22")}
	if got := b.DailyTarget("2026-03-02", nil, windows, true); got == nil || *got != 7*3600+30*60 {
		t.Errorf("sessionsAreRoundedUp: %v", got)
	}
	windows = []core.Interval{ts.Interval("2026-03-02", "08:00", "16:00")}
	info := record("2026-03-02", core.DayWorkday, p(7*3600+5*60))
	if got := b.DailyTarget("2026-03-02", info, windows, true); got == nil || *got != 7*3600+15*60 {
		t.Errorf("reportedHoursWinOverSessions: %v", got)
	}
	if got := b.DailyTarget("2026-03-02", nil, nil, false); got == nil || *got != 8*3600 {
		t.Error("workdaysUseTheFixedHours")
	}
	s = def()
	if ts.Builder(s, nil, nil).DailyTarget("2026-03-07", nil, nil, false) != nil {
		t.Error("weekendsOnlyWhenIncluded: excluded")
	}
	s.IncludeWeekends = true
	if got := ts.Builder(s, nil, nil).DailyTarget("2026-03-07", nil, nil, false); got == nil || *got != 8*3600 {
		t.Error("weekendsOnlyWhenIncluded: included")
	}
	if ts.Builder(s, nil, nil).DailyTarget("2026-03-07", record("2026-03-07", core.DayLeave, nil), nil, false) != nil {
		t.Error("weekendsOnlyWhenIncluded: leave")
	}
}

func TestCommitDay(t *testing.T) {
	none := func(string) *core.DayRecord { return nil }
	no := func(string) bool { return false }
	b := builder()
	if got := b.CommitDay("2026-03-03", none, no); got != "2026-03-03" {
		t.Errorf("workdayKeepsItsCommits: %s", got)
	}
	if b.CommitDay("2026-03-07", none, no) != "2026-03-06" || b.CommitDay("2026-03-08", none, no) != "2026-03-09" {
		t.Error("saturdayGoesToFridayAndSundayToMonday")
	}
	if b.CommitDay("2026-03-07", none, func(d string) bool { return d == "2026-03-07" }) != "2026-03-07" {
		t.Error("workedRestDayKeepsItsCommits")
	}
	s := def()
	s.IncludeWeekends = true
	if ts.Builder(s, nil, nil).CommitDay("2026-03-07", none, no) != "2026-03-07" {
		t.Error("includedWeekendsKeepTheirCommits")
	}
	records := map[string]*core.DayRecord{
		"2026-03-04": record("2026-03-04", core.DayLeave, nil), "2026-03-02": record("2026-03-02", core.DayLeave, nil),
	}
	info := func(d string) *core.DayRecord { return records[d] }
	if b.CommitDay("2026-03-04", info, no) != "2026-03-03" || b.CommitDay("2026-03-02", info, no) != "2026-03-03" {
		t.Error("leaveGoesToTheClosestWorkdayEarlierFirst")
	}
}

func TestWorkWindows(t *testing.T) {
	day := "2026-03-03"
	in := core.DayInput{Day: day, Sessions: []core.WorkSession{
		ts.Session(day, "08:00", "16:30", core.SessionWork), ts.Session(day, "12:00", "12:30", core.SessionPause),
	}}
	w, fromSessions := builder().WorkWindows(in)
	want := []core.Interval{ts.Interval(day, "08:00", "12:00"), ts.Interval(day, "12:30", "16:30")}
	if !fromSessions || !reflect.DeepEqual(w, want) {
		t.Errorf("sessionsMinusBreaks: %v %v", w, fromSessions)
	}
	w, fromSessions = builder().WorkWindows(core.DayInput{Day: day})
	if fromSessions || !reflect.DeepEqual(w, []core.Interval{ts.Interval(day, "09:00", "17:30")}) {
		t.Errorf("defaultWorkdayWithoutSessions: %v", w)
	}
	s := def()
	s.DefaultDayEnd = "08:00"
	if w, _ := ts.Builder(s, nil, nil).WorkWindows(core.DayInput{Day: day}); len(w) != 0 {
		t.Error("invalidDefaultWorkdayGivesNoWindows")
	}
}

func str(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestIssueResolution(t *testing.T) {
	day := "2026-03-03"
	withIssue := ts.Builder(def(), []core.Repo{ts.Repo("billing", p("PROJ-1"), nil)}, nil)
	if got := withIssue.CommitIssue(ts.Commit(day, "10:00", ts.Subject("PROJ-5 Fix totals"))); str(got) != "PROJ-5" {
		t.Errorf("commitMessageWins: %s", str(got))
	}
	c := ts.Commit(day, "10:00", ts.Subject("Fix totals"), ts.Ref("refs/heads/feature/proj-7-login"))
	if got := withIssue.CommitIssue(c); str(got) != "PROJ-7" {
		t.Errorf("branchNameIsSecond: %s", str(got))
	}
	c = ts.Commit(day, "10:00", ts.Subject("Fix totals"))
	if str(withIssue.CommitIssue(c)) != "PROJ-1" {
		t.Error("repositoryIssue")
	}
	b := ts.Builder(def(), []core.Repo{ts.Repo("billing", nil, p("PROJ"))},
		[]core.ProjectMapping{{Key: "PROJ", DefaultIssue: p("PROJ-99")}})
	if str(b.CommitIssue(c)) != "PROJ-99" || builder().CommitIssue(c) != nil {
		t.Error("projectDefault")
	}
	b = ts.Builder(def(), []core.Repo{ts.Repo("billing", nil, p("PROJ"))}, nil)
	if str(b.CommitIssue(ts.Commit(day, "10:00", ts.Subject("FOO-1 and PROJ-2")))) != "PROJ-2" {
		t.Error("knownProjectPrefixBeatsEarlierUnknownKey")
	}

	s := def()
	s.MeetingRules = []core.MeetingRule{{Contains: "standup", IssueKey: "int-12"}}
	s.DefaultMeetingIssue = "int-1"
	b = ts.Builder(s, nil, nil)
	for title, want := range map[string]string{"PROJ-3 planning": "PROJ-3", "Daily Standup": "INT-12", "1:1": "INT-1"} {
		if got := str(b.MeetingIssue(ts.Meeting(day, "09:00", "10:00", title))); got != want {
			t.Errorf("meetingIssues %q: %s", title, got)
		}
	}

	s = def()
	s.SkipMeetingKeywords = []string{"lunch", ""}
	b = ts.Builder(s, nil, nil)
	skipped := ts.Meeting(day, "09:00", "10:00", "Meeting")
	skipped.Skipped = true
	if !b.IsSkipped(ts.Meeting(day, "12:00", "13:00", "Team Lunch")) || !b.IsSkipped(skipped) ||
		b.IsSkipped(ts.Meeting(day, "09:00", "10:00", "Planning")) {
		t.Error("skippedMeetings")
	}

	s = def()
	s.AbsenceRules = []core.MeetingRule{{Contains: "beteg", IssueKey: "hr-3"}}
	s.AbsenceIssueKey = "hr-1"
	b = ts.Builder(s, nil, nil)
	if str(b.AbsenceIssue(core.DayRecord{Day: day, Status: core.DayLeave, Label: "Betegszabadság"})) != "HR-3" ||
		str(b.AbsenceIssue(core.DayRecord{Day: day, Status: core.DayLeave, Label: "Szabadság"})) != "HR-1" {
		t.Error("absenceIssue")
	}

	s = def()
	s.Periods = []core.ProjectPeriod{
		{ID: "1", Title: "Old", IssueKey: "PROJ-1", From: "2026-03-01", To: "2026-03-31"},
		{ID: "2", Title: "New", IssueKey: "PROJ-2", From: "2026-03-02", To: "2026-03-06"},
		{ID: "3", Title: "Empty", IssueKey: " ", From: "2026-03-01", To: "2026-03-31"},
	}
	b = ts.Builder(s, nil, nil)
	if b.Period("2026-03-03").Title != "New" || b.Period("2026-03-10").Title != "Old" || b.Period("2026-04-01") != nil {
		t.Error("latestPeriodWinsAndPeriodsWithoutIssueAreIgnored")
	}
}

func only(t *testing.T, logs []core.Worklog) core.Worklog {
	t.Helper()
	if len(logs) != 1 {
		t.Fatalf("want one entry, got %d: %+v", len(logs), logs)
	}
	return logs[0]
}

func TestBuild(t *testing.T) {
	day := "2026-03-03" // Tuesday
	if logs := builder().Build(core.DayInput{Day: day}); len(logs) != 0 {
		t.Error("emptyWorkdayHasNoEntries")
	}

	t.Run("leaveDayIsOneAbsenceEntry", func(t *testing.T) {
		s := def()
		s.AbsenceIssueKey = "HR-1"
		info := &core.DayRecord{Day: day, Status: core.DayLeave, Label: "Szabadság"}
		e := only(t, ts.Builder(s, nil, nil).Build(core.DayInput{
			Day:     day,
			Commits: []core.StoredCommit{ts.Commit(day, "10:00")}, Info: info,
		}))
		if e.Source != core.SourceAbsence || str(e.IssueKey) != "HR-1" || e.Duration != 8*3600 ||
			e.Description != "Szabadság" || e.SourceRef != "absence:"+day {
			t.Errorf("%+v", e)
		}
	})

	t.Run("leaveDayWithoutAbsenceIssueIsEmpty", func(t *testing.T) {
		info := &core.DayRecord{Day: day, Status: core.DayLeave, Label: "Szabadság"}
		if logs := builder().Build(core.DayInput{Day: day, Info: info}); len(logs) != 0 {
			t.Error(logs)
		}
	})

	t.Run("meetingsKeepTheirSlotAndCommitsFillTheRestOfTheDay", func(t *testing.T) {
		s := def()
		s.MeetingRules = []core.MeetingRule{{Contains: "standup", IssueKey: "INT-12"}}
		logs := ts.Builder(s, nil, nil).Build(core.DayInput{
			Day: day,
			Commits: []core.StoredCommit{
				ts.Commit(day, "11:00", ts.Subject("PROJ-5 Invoices")), ts.Commit(day, "15:00", ts.Subject("PROJ-6: Reports")),
			},
			Meetings: []core.Meeting{ts.Meeting(day, "09:00", "09:15", "Daily standup")},
		})
		if core.TotalDuration(logs) != 8*3600 {
			t.Errorf("total %d", core.TotalDuration(logs))
		}
		m := logs[0]
		if m.Source != core.SourceMeeting || str(m.IssueKey) != "INT-12" || ts.Days.HM(m.Start) != "09:00" || m.Duration != 900 {
			t.Errorf("meeting %+v", m)
		}
		var git []core.Worklog
		for _, l := range logs {
			if l.Source == core.SourceGit {
				git = append(git, l)
			}
		}
		if len(git) != 2 || str(git[0].IssueKey) != "PROJ-5" || str(git[1].IssueKey) != "PROJ-6" ||
			git[0].Description != "Invoices" || git[1].Description != "Reports" {
			t.Fatalf("git %+v", git)
		}
		if ts.Days.HM(git[0].Start) != "09:15" || !git[1].Start.Equal(git[0].End()) {
			t.Errorf("placement %s %s", git[0].Start, git[1].Start)
		}
	})

	t.Run("commitsOfTheSameIssueAreGrouped", func(t *testing.T) {
		e := only(t, builder().Build(core.DayInput{Day: day, Commits: []core.StoredCommit{
			ts.Commit(day, "10:00", ts.Hash("a"), ts.Subject("PROJ-5 First")),
			ts.Commit(day, "11:00", ts.Hash("b"), ts.Subject("PROJ-5 Second")),
			ts.Commit(day, "12:00", ts.Hash("c"), ts.Subject("PROJ-5 First")),
		}}))
		if !reflect.DeepEqual(e.Refs(), []string{"a", "b", "c"}) || e.Description != "First; Second" || e.Duration != 8*3600 {
			t.Errorf("%+v", e)
		}
	})

	t.Run("keptEntriesOwnTheirCommitsAndReduceTheTarget", func(t *testing.T) {
		kept := []core.Worklog{
			ts.Worklog(day, "09:00", 3600, core.SourceManual, "", true),
			ts.Worklog(day, "10:00", 1800, core.SourceGit, "claimed", true),
		}
		logs := builder().Build(core.DayInput{Day: day, Kept: kept, Commits: []core.StoredCommit{
			ts.Commit(day, "10:20", ts.Hash("claimed"), ts.Subject("PROJ-1 Old")),
			ts.Commit(day, "14:00", ts.Hash("new"), ts.Subject("PROJ-2 New")),
		}})
		if len(logs) != 1 || logs[0].SourceRef != "new" || core.TotalDuration(logs) != 8*3600-3600-1800 {
			t.Fatalf("%+v", logs)
		}
		for _, k := range kept {
			if logs[0].Start.Before(k.End()) && k.Start.Before(logs[0].End()) {
				t.Errorf("overlaps kept entry at %s", k.Start)
			}
		}
	})

	t.Run("skippedMeetingsAreNotLogged", func(t *testing.T) {
		m := ts.Meeting(day, "10:00", "11:00", "Meeting")
		m.Skipped = true
		for _, l := range builder().Build(core.DayInput{Day: day, Meetings: []core.Meeting{m}}) {
			if l.Source == core.SourceMeeting {
				t.Error(l)
			}
		}
	})

	t.Run("commitsWithoutIssueAreGroupedPerRepository", func(t *testing.T) {
		b := ts.Builder(def(), []core.Repo{ts.Repo("billing", nil, nil)}, nil)
		e := only(t, b.Build(core.DayInput{Day: day, Commits: []core.StoredCommit{ts.Commit(day, "10:00", ts.Subject("Tidy up"))}}))
		if e.IssueKey != nil || e.Description != "[billing] Tidy up" {
			t.Errorf("%+v", e)
		}
	})

	t.Run("longDescriptionsAreTruncated", func(t *testing.T) {
		subject := "PROJ-1 " + strings.Repeat("a", 300)
		e := only(t, builder().Build(core.DayInput{Day: day, Commits: []core.StoredCommit{ts.Commit(day, "10:00", ts.Subject(subject))}}))
		if len([]rune(e.Description)) != 253 || !strings.HasSuffix(e.Description, "…") {
			t.Errorf("%d", len([]rune(e.Description)))
		}
	})

	t.Run("estimateModeKeepsRoundedEstimates", func(t *testing.T) {
		s := def()
		s.RoundingMode = core.RoundingEstimate
		// First commit of the day: the gap from 09:00 is 60 minutes → 45 minute estimate.
		e := only(t, ts.Builder(s, nil, nil).Build(core.DayInput{
			Day:     day,
			Commits: []core.StoredCommit{ts.Commit(day, "10:00", ts.Subject("PROJ-1 Add feature"))},
		}))
		if e.Estimated != 45*60 || e.Duration != 45*60 {
			t.Errorf("%+v", e)
		}
	})

	t.Run("sessionsDriveTheTarget", func(t *testing.T) {
		logs := builder().Build(core.DayInput{
			Day:      day,
			Sessions: []core.WorkSession{ts.Session(day, "08:00", "15:22", core.SessionWork)},
			Commits:  []core.StoredCommit{ts.Commit(day, "10:00", ts.Subject("PROJ-1 x"))},
		})
		if core.TotalDuration(logs) != 7*3600+30*60 || ts.Days.HM(logs[0].Start) != "08:00" {
			t.Errorf("%+v", logs)
		}
	})
}

func chatDay(day, conv string, times ...string) core.ChatDay {
	c := core.ChatDay{ConversationID: conv, Name: conv, Type: "dm", Day: day, Gap: core.DefaultChatGap}
	for _, hm := range times {
		c.Times = append(c.Times, ts.At(day, hm))
	}
	return c
}

func TestChatDrafts(t *testing.T) {
	day := "2026-03-03"
	long := chatDay(day, "dm/A", "10:00", "10:08", "10:16", "10:20") // 20 minutes of chatting
	short := chatDay(day, "dm/B", "11:00", "11:05")                  // 5 minutes: Chats page only
	logs := builder().Build(core.DayInput{
		Day: day, Chats: []core.ChatDay{long, short},
		Commits: []core.StoredCommit{ts.Commit(day, "15:00", ts.Subject("PROJ-1 Fix"))},
	})
	var chat []core.Worklog
	for _, w := range logs {
		if w.Source == core.SourceChat {
			chat = append(chat, w)
		}
	}
	if len(chat) != 1 || chat[0].IssueKey != nil || chat[0].Duration != 15*60 || chat[0].Estimated != 20*60 ||
		ts.Days.HM(chat[0].Start) != "10:00" || chat[0].SourceRef != "chat:dm/A|"+day || chat[0].Status != core.StatusDraft {
		t.Fatalf("one unticketed draft for the long chat: %+v", chat)
	}
	if core.TotalDuration(logs) != 8*3600 {
		t.Errorf("git fills the rest of the day: %d", core.TotalDuration(logs))
	}
	if git := core.TotalDuration(logs) - chat[0].Duration; git != 8*3600-15*60 {
		t.Errorf("git gets the day minus the chat: %d", git)
	}

	s := def()
	s.ChatRules = []core.ChatRule{{ConversationID: "dm/A", IssueKey: "int-5"}}
	only := ts.Builder(s, nil, nil).Build(core.DayInput{Day: day, Chats: []core.ChatDay{long}})
	if len(only) != 1 || str(only[0].IssueKey) != "INT-5" {
		t.Errorf("an Always rule gives the ticket: %+v", only)
	}
	kept := only[0]
	kept.Locked = true
	if again := ts.Builder(s, nil, nil).Build(core.DayInput{Day: day, Chats: []core.ChatDay{long}, Kept: []core.Worklog{kept}}); len(again) != 0 {
		t.Errorf("a kept chat entry isn't made twice: %+v", again)
	}
}

func TestJiraTickets(t *testing.T) {
	day := "2026-03-03"
	ticket := func(key, from, to string) core.JiraTicket {
		return core.JiraTicket{Day: day, IssueKey: key, Summary: "Spec " + key, First: ts.At(day, from), Last: ts.At(day, to)}
	}
	// aJiraOnlyDayFillsTheRestOfTheTargetAroundMeetings: 8h, a 1h meeting, two tickets weighted 1h : 2h (capped).
	logs := builder().Build(core.DayInput{
		Day:      day,
		Meetings: []core.Meeting{ts.Meeting(day, "09:00", "10:00", "Refinement")},
		Jira:     []core.JiraTicket{ticket("PM-1", "10:30", "11:30"), ticket("PM-2", "13:00", "17:00")},
	})
	got := map[string]core.Worklog{}
	for _, w := range logs {
		got[str(w.IssueKey)+"|"+string(w.Source)] = w
	}
	a, b := got["PM-1|jira"], got["PM-2|jira"]
	if len(logs) != 3 || a.Duration+b.Duration != 7*3600 || a.Duration*2 > b.Duration+g ||
		a.Description != "Spec PM-1" || a.SourceRef != "jira:PM-1:"+day || !a.Start.Before(b.Start) {
		t.Errorf("jiraOnlyDay: %+v", logs)
	}

	// aTicketWithCommitsKeepsTheCommitsEstimateAndOnlyAddsItsRef.
	e := only(t, builder().Build(core.DayInput{
		Day:     day,
		Commits: []core.StoredCommit{ts.Commit(day, "11:00", ts.Hash("a"), ts.Subject("PM-1 Code"))},
		Jira:    []core.JiraTicket{ticket("PM-1", "10:00", "16:00")},
	}))
	if e.Source != core.SourceGit || e.SourceRef != "a,jira:PM-1:"+day || e.Description != "Code" {
		t.Errorf("mergesWithCommits: %+v", e)
	}

	// aClaimedTicketDayIsNotRebuilt.
	kept := core.Worklog{Day: day, Start: ts.At(day, "09:00"), Duration: 3600, IssueKey: p("PM-1"), SourceRef: "jira:PM-1:" + day}
	for _, w := range builder().Build(core.DayInput{Day: day, Jira: []core.JiraTicket{ticket("PM-1", "10:00", "11:00")}, Kept: []core.Worklog{kept}}) {
		if w.Source == core.SourceJira {
			t.Errorf("claimed: %+v", w)
		}
	}
}

func TestConfluencePages(t *testing.T) {
	day := "2026-03-03"
	page := func(id, title, from, to string) core.ConfluencePage {
		return core.ConfluencePage{Day: day, PageID: id, Title: title, Space: "Product", First: ts.At(day, from), Last: ts.At(day, to)}
	}
	logs := builder().Build(core.DayInput{
		Day:        day,
		Jira:       []core.JiraTicket{{Day: day, IssueKey: "PROJ-1", Summary: "Spec", First: ts.At(day, "09:00"), Last: ts.At(day, "10:00")}},
		Confluence: []core.ConfluencePage{page("7", "PROJ-1 Spec page", "09:30", "10:00"), page("8", "Roadmap", "13:00", "14:00")},
	})
	if len(logs) != 2 || core.TotalDuration(logs) != 8*3600 {
		t.Fatalf("%+v", logs)
	}
	// A page with the ticket in its title joins that ticket's entry; one without becomes its own, ticketless.
	proj, roadmap := logs[0], logs[1]
	if str(proj.IssueKey) != "PROJ-1" || proj.SourceRef != "jira:PROJ-1:"+day+",confluence:7:"+day {
		t.Errorf("merged: %+v", proj)
	}
	if roadmap.IssueKey != nil || roadmap.Source != core.SourceConfluence || roadmap.Description != "[Product] Roadmap" ||
		roadmap.SourceRef != "confluence:8:"+day {
		t.Errorf("own: %+v", roadmap)
	}
}

func TestEmailThreads(t *testing.T) {
	day := "2026-03-03"
	thread := func(subject string, n int, from string) core.EmailThread {
		return core.EmailThread{Day: day, Subject: subject, Count: n, First: ts.At(day, from), Last: ts.At(day, from)}
	}
	// No target: each thread keeps its weight, 10 minutes a mail up to an hour.
	b := builder()
	b.Settings.RoundingMode = core.RoundingEstimate
	logs := b.Build(core.DayInput{Day: day, Email: []core.EmailThread{thread("PROJ-1 prices", 2, "09:00"), thread("Offsite", 9, "13:00")}})
	if len(logs) != 2 || str(logs[0].IssueKey) != "PROJ-1" || logs[0].Estimated != 20*60 || logs[0].Source != core.SourceEmail ||
		logs[1].IssueKey != nil || logs[1].Estimated != 3600 || logs[1].Description != "[Email] Offsite" {
		t.Errorf("%+v", logs)
	}
	if core.ThreadSubject("RE: Fwd: VS: Offsite ") != "Offsite" || core.ThreadSubject("Re:") != "(no subject)" {
		t.Error("thread subject")
	}
	if thread("Offsite", 1, "09:00").Ref() != thread("offsite", 3, "10:00").Ref() || strings.Contains(thread("a,b", 1, "09:00").Ref(), ",") {
		t.Error("ref")
	}
}
