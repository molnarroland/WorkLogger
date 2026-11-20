package app

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/view"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

var update = flag.Bool("update", false, "rewrite the frontend's story fixtures")

// storyFixtures is where the frontend's FakeApi and stories read the sample data from, as the backend would
// send it. Regenerate with `task fixtures` (go test ./app -run TestStoryFixtures -update).
var storyFixtures = filepath.Join("..", "frontend", "src", "app", "api", "testing", "sample.json")

const storyToday = "2026-10-30"

type storyData struct {
	Today       string              `json:"today"`
	Months      map[string]MonthDTO `json:"months"`
	EmptyMonths map[string]MonthDTO `json:"emptyMonths"`
	// Weeks and EmptyWeeks by their Monday.
	Weeks      map[string]WeekDTO     `json:"weeks"`
	EmptyWeeks map[string]WeekDTO     `json:"emptyWeeks"`
	Sessions   map[string]SessionsDTO `json:"sessions"`
	Meetings   map[string]MeetingsDTO `json:"meetings"`
	Chats      map[string]ChatsDTO    `json:"chats"`
	// Setup is the setup steps of the story settings: configured (defaultSettings), fresh (freshSettings), and
	// the checklist and welcome-sheet stories' own.
	Setup   map[string][]SetupStepDTO `json:"setup"`
	Entries map[string]WorklogDTO     `json:"entries"`
	Days    map[string]DayDTO         `json:"days"`
}

// The story settings' period (fixtures.ts defaultSettings).
var storyPeriods = []core.ProjectPeriod{{ID: "p1", Title: "Map provider migration", IssueKey: "ORB-300", From: "2026-10-19", To: "2026-10-23"}}

// storySample is the sample data of every month the stories use, so weeks across a month end have both sides.
func storySample(empty bool) ([]view.DayInfo, []core.Worklog) {
	var in []view.DayInfo
	var all []core.Worklog
	for _, m := range []string{"2026-08", "2026-09", "2026-10", "2026-11", "2026-12"} {
		info, logs := ts.SampleMonth(m)
		for _, d := range info {
			in = append(in, view.DayInfo(d))
		}
		for _, w := range logs {
			w.ID = int64(len(all) + 1)
			all = append(all, w)
		}
	}
	if empty {
		all = nil
	}
	return in, all
}

func within(logs []core.Worklog, from, to string) []core.Worklog {
	var out []core.Worklog
	for _, w := range logs {
		if from <= w.Day && w.Day <= to {
			out = append(out, w)
		}
	}
	return out
}

func storyMonth(month string, empty bool) MonthDTO {
	info, logs := storySample(empty)
	from, to, _ := ts.Days.Month(month)
	m := view.BuildMonth(view.MonthInput{
		Month: month, Today: storyToday, Days: ts.Days, Info: monthInfo(info, from, to),
		Worklogs: within(logs, from, to), Periods: storyPeriods,
	})
	return monthDTO(m, []string{})
}

func monthInfo(info []view.DayInfo, from, to string) []view.DayInfo {
	var out []view.DayInfo
	for _, d := range info {
		if from <= d.Day && d.Day <= to {
			out = append(out, d)
		}
	}
	return out
}

func storyWeek(monday string, empty bool) WeekDTO {
	info, logs := storySample(empty)
	to := ts.Days.Shift(monday, 6)
	w := view.BuildWeek(view.WeekInput{
		Day: monday, Today: storyToday, Days: ts.Days, Info: monthInfo(info, monday, to),
		Worklogs: within(logs, monday, to), Periods: storyPeriods,
	})
	return weekDTO(w)
}

// storyBuilder applies the story's meeting rules. The standup gets its ticket from a rule rather than as the
// default meeting issue, so the 1:1s stay untagged and the Meetings page has a rule to suggest.
var storyBuilder = core.Builder{
	Days: ts.Days, Repos: map[int64]core.Repo{}, Projects: map[string]core.ProjectMapping{},
	Settings: core.Settings{
		MeetingRules: []core.MeetingRule{
			{ID: "m1", Contains: "Sprint planning", IssueKey: "TEAM-7"},
			{ID: "m2", Contains: "Daily standup", IssueKey: "TEAM-12"},
		},
		SkipMeetingKeywords: []string{"Lunch", "Focus time"},
	},
}

// storyChats is two conversations on the 28th (fixtures.ts sampleChats); the trip planner channel has an "Always" rule.
func storyChats(month string) []ChatDayDTO {
	day := month + "-28"
	return []ChatDayDTO{
		{
			ID: day + "|space/orb", Day: day, ConversationID: "space/orb", Name: "Trip planner", Type: "space", Count: 3,
			ActiveSeconds: 600, First: rfc(ts.At(day, "10:02")), Last: rfc(ts.At(day, "10:12")),
			IssueKey: core.Ptr("ORB-214"), RuleIssue: core.Ptr("ORB-214"),
		},
		{
			ID: day + "|dm/mira", Day: day, ConversationID: "dm/mira", Name: "Mira Lindqvist", Type: "dm", Count: 6,
			ActiveSeconds: 1080, First: rfc(ts.At(day, "14:40")), Last: rfc(ts.At(day, "14:58")),
		},
	}
}

func buildStoryData() storyData {
	d := storyData{
		Today: storyToday, Months: map[string]MonthDTO{}, EmptyMonths: map[string]MonthDTO{},
		Weeks: map[string]WeekDTO{}, EmptyWeeks: map[string]WeekDTO{}, Entries: map[string]WorklogDTO{}, Days: map[string]DayDTO{},
		Sessions: map[string]SessionsDTO{}, Setup: map[string][]SetupStepDTO{},
		Meetings: map[string]MeetingsDTO{}, Chats: map[string]ChatsDTO{},
	}
	all := []core.Module{core.ModuleGit, core.ModuleCalendar, core.ModuleWorkTimes, core.ModuleChats}
	google := core.Profile{ID: "g", Kind: core.ProfileGoogleChat, Email: "me@example.com"}
	for name, s := range map[string]core.Settings{
		"configured": {
			Modules: all, TempoByHand: true, WorkspaceRoots: []string{"~/Workspaces"},
			Profiles: []core.Profile{google, {ID: "h", Kind: core.ProfileHRmaster, URL: "https://example.hrmaster.hu"}},
		},
		"fresh":     {Modules: all, TempoByHand: true},
		"checklist": {Modules: []core.Module{core.ModuleGit, core.ModuleChats, core.ModuleWorkTimes}, WorkspaceRoots: []string{"~/Workspaces"}, Profiles: []core.Profile{google}},
		"welcome":   {Modules: []core.Module{core.ModuleGit, core.ModuleCalendar, core.ModuleWorkTimes}, WorkspaceRoots: []string{"~/Workspaces"}, Profiles: []core.Profile{google}},
	} {
		d.Setup[name] = setupDTOs(view.SetupSteps(s))
	}
	for _, m := range []string{"2026-09", "2026-10", "2026-11"} {
		d.Months[m] = storyMonth(m, false)
		d.EmptyMonths[m] = storyMonth(m, true)
		sessions, records := ts.SampleSessions(m)
		d.Sessions[m] = sessionsDTO(view.SessionDays(sessions, sessionRecords(records, ts.Days)), ts.Days)
		d.Meetings[m] = meetingsDTO(meetingRows(storyBuilder, ts.SampleMeetings(m)), ts.Days)
		d.Chats[m] = chatsDTO(storyChats(m))
		for _, week := range view.WeeksOfMonth(ts.Days, m) {
			d.Weeks[week[0]] = storyWeek(week[0], false)
			d.EmptyWeeks[week[0]] = storyWeek(week[0], true)
		}
	}

	// Single entries and days for component stories (fixtures.ts `fixtures`). Ids from 9001 so they don't clash.
	const day = "2026-10-27"
	entry := func(id int64, hm string, minutes int, key, text string, source core.Source, status core.Status, change func(*core.Worklog)) WorklogDTO {
		w := core.Worklog{ID: id, Day: day, Start: ts.At(day, hm), Duration: minutes * 60, Description: text, Source: source, Status: status, Estimated: minutes * 60}
		if key != "" {
			w.IssueKey = core.Ptr(key)
		}
		if status != core.StatusDraft {
			w.Comment = text
		}
		w.Locked = status == core.StatusExported
		if change != nil {
			change(&w)
		}
		return entryDTO(view.NewEntry(w, nil, ts.Days))
	}
	d.Entries["draftGitEntry"] = entry(9001, "09:15", 195, "ORB-214", "Offline mode for saved trips", core.SourceGit, core.StatusDraft, func(w *core.Worklog) { w.Estimated = 160 * 60 })
	d.Entries["lockedEntry"] = entry(9002, "12:30", 30, "ORB-227", "Faster search index rebuild", core.SourceGit, core.StatusReviewed, func(w *core.Worklog) { w.Locked = true })
	d.Entries["exportedEntry"] = entry(9003, "09:00", 15, "TEAM-12", "Daily standup", core.SourceMeeting, core.StatusExported, func(w *core.Worklog) { w.TempoID = core.Ptr[int64](4711) })
	d.Entries["needsTicketEntry"] = entry(9004, "13:00", 240, "", "Code review", core.SourceManual, core.StatusDraft, nil)
	d.Entries["leaveEntry"] = entry(9005, "08:00", 480, "LEAVE-1", "Szabadság", core.SourceAbsence, core.StatusExported, func(w *core.Worklog) {
		w.Day, w.Start = "2026-10-09", ts.At("2026-10-09", "08:00")
	})

	oct := d.Months["2026-10"]
	byDay := map[string]DayDTO{}
	for _, x := range oct.Days {
		byDay[x.Day] = x
	}
	work := byDay[day]
	work.Label = "Munkanap (Home Office)"
	d.Days["workday"] = work
	d.Days["leaveDay"] = byDay["2026-10-09"]
	d.Days["restDay"] = byDay["2026-10-24"]
	return d
}

func TestStoryFixtures(t *testing.T) {
	got, err := json.MarshalIndent(buildStoryData(), "", " ")
	must(t, err)
	got = append(got, '\n')
	if *update {
		must(t, os.MkdirAll(filepath.Dir(storyFixtures), 0o755))
		must(t, os.WriteFile(storyFixtures, got, 0o644))
		return
	}
	want, err := os.ReadFile(storyFixtures)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("%s is out of date: run `task fixtures` and commit the result (%v)", storyFixtures, err)
	}
}
