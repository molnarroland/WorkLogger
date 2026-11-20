package core_test

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Rows with the exact markup of HRmaster's Munkaidő table.
func hrCell(repeater, from, to string) string {
	return `<td class="col-1 min-width-100 text-center" data-e2e-testing="Datacenter.Employee.Calendar.` + repeater + `"><div><div class="d-ops align-items-center">` +
		`<div class="ops-1" data-e2e-testing="Evolution.TimepickerSwitchInputContainer.From"><div tabindex="-1" class="timepicker disable-me disabled " data-e2e-testing="Evolution.TimepickerSwitchInput">` + from + `</div></div>` +
		`<div class="ops-1" data-e2e-testing="Evolution.TimepickerSwitchInputContainer.To"><div tabindex="-1" class="timepicker disable-me disabled " data-e2e-testing="Evolution.TimepickerSwitchInput">` + to + `</div></div></div></div></td>`
}

func hrRow(dd, typ, name, label, worked, unpaid string, work, breaks [][2]string) string {
	for len(work) < 3 {
		work = append(work, [2]string{})
	}
	for len(breaks) < 2 {
		breaks = append(breaks, [2]string{})
	}
	cells := ""
	for _, w := range work {
		cells += hrCell("WorkScheduleRepeater", w[0], w[1])
	}
	for _, b := range breaks {
		cells += hrCell("RestBreaksRepeater", b[0], b[1])
	}
	return `<tr class="calendar-closed-day " data-e2e-testing="Datacenter.Employee.Calendar.RowRepeater"><td class="col-0-5 min-width-200 table-sticky-column" style="position: relative;">` +
		`<div data-e2e-testing="Datacenter.Calendar.DayCell" class="
                    d-ops
                    Type_CalendarDayStatus_` + typ + `
                    null
                    calendar-date__container
                "><div class="calendar-date__day" data-e2e-testing="Datacenter.Calendar.Day">` + dd + `.</div><div class="calendar-date__content">` +
		`<div class="calendar-date__day-name" data-e2e-testing="Datacenter.Calendar.Day.Name">` + name + `</div><div class="calendar-date__status" data-e2e-testing="Datacenter.Calendar.Day.Status">` +
		`<div data-e2e-testing="Datacenter.Calendar.Day.Status.` + typ + `" id=":r43:" tabindex="0" aria-describedby=":r42:"><div class="LinesEllipsis  ">` + label + `<wbr></div></div></div></div></div></td>` +
		`<td class="col-1 text-center" data-e2e-testing="Datacenter.Employee.Calendar.FactWorkedHoursSum">` + worked + `</td>` +
		`<td class="col-1 text-center" data-e2e-testing="Datacenter.Employee.Calendar.UnpaidRestBreaksSum">` + unpaid + `</td>` + cells +
		`<td class="col-0-5 min-width-100 text-center" data-e2e-testing="Datacenter.Calendar.Actual.PaidRestBreak.Column"><div class="d-ops align-items-center"><div class="ops-1">` +
		`<input tabindex="-1" type="text" class="form-control disable-me text-center disable-me" maxlength="7" disabled="" data-e2e-testing="Datacenter.Employee.Calendar.PaidRestBreakInput" value="0"></div></div></td></tr>`
}

var march2026 = `<div class="cs-date-inner-toggle position-relative width-180" data-e2e-testing="Datacenter.Calendar.PeriodSelect">2026. március</div>` +
	`<div class="sd-element justify-content-center" data-e2e-testing="PeriodSelect.LeftRepeater.2027"><span>2027</span></div><span>április</span>` +
	"<table><thead><tr><th>Dátum</th><th>Munkaidő 1</th><th data-e2e-testing=\"Datacenter.Calendar.Actual.PaidRestBreak.Column\">Fizetett szünet</th></tr></thead><tbody>\n" +
	hrRow("01", "RestDay", "vasárnap", "Pihenőnap", "00:00", "00:00", nil, nil) +
	hrRow("02", "Workday", "hétfő", "Munkanap (Home Office)", "07:22", "00:56", [][2]string{{"08:11", "16:29"}}, [][2]string{{"11:19", "12:15"}}) +
	hrRow("04", "LabourCode", "szerda", "Szabadság", "08:00", "00:00", nil, nil) +
	hrRow("09", "Workday", "hétfő", "Munkanap (Home Office)", "09:33", "00:26", [][2]string{{"08:16", "15:17"}, {"20:06", "23:04"}}, [][2]string{{"12:50", "13:16"}}) +
	hrRow("12", "Workday", "csütörtök", "Munkanap", "10:08", "00:00", [][2]string{{"08:02", "11:27"}, {"13:07", "15:45"}, {"19:11", "23:16"}}, nil) +
	hrRow("15", "PublicHoliday", "vasárnap", "Ünnepnap", "00:00", "00:00", nil, nil) +
	hrRow("26", "Workday", "csütörtök", "Munkanap", "08:54", "04:45", [][2]string{{"07:58", "21:37"}}, [][2]string{{"11:34", "13:03"}, {"14:54", "18:10"}}) +
	`</tbody><tfoot><tr><td class="working-time-table-summary-label" colspan="1">Havi összesen:</td><td class="text-center" data-e2e-testing="Datacenter.Employee.AllPlanWorkingHour">172:37</td></tr></tfoot></table>`

func parseHR(raw string) core.ParsedTimesheet {
	return core.HRmasterParser{FallbackYear: 2026, FallbackMonth: 11, Days: ts.Days}.Parse(raw)
}

func fixedHours(absenceIssue string) core.Builder {
	s := def()
	s.RoundingMode = core.RoundingFixedHours
	s.AbsenceIssueKey = absenceIssue
	return ts.Builder(s, nil, nil)
}

func TestHRmasterParser(t *testing.T) {
	sheet := parseHR(march2026)
	var keys []string
	days := map[string]*core.DayRecord{}
	for i, d := range sheet.Days {
		keys = append(keys, d.Day)
		days[d.Day] = &sheet.Days[i]
	}
	want := []string{"2026-03-01", "2026-03-02", "2026-03-04", "2026-03-09", "2026-03-12", "2026-03-15", "2026-03-26"}
	if !reflect.DeepEqual(keys, want) || len(sheet.Warnings) != 0 {
		t.Fatalf("detectsFormatAndMonth: %v %v", keys, sheet.Warnings)
	}

	if days["2026-03-01"].Status != core.DayRestDay || days["2026-03-02"].Status != core.DayWorkday ||
		days["2026-03-02"].Label != "Munkanap (Home Office)" || days["2026-03-04"].Status != core.DayLeave ||
		*days["2026-03-04"].ReportedWorked != 8*3600 || days["2026-03-15"].Status != core.DayPublicHoliday {
		t.Error("dayStatusesAndLabels")
	}

	sessionsOf := func(day string) []core.WorkSession {
		var out []core.WorkSession
		for _, s := range sheet.Sessions {
			if s.Day == day {
				out = append(out, s)
			}
		}
		return out
	}
	count := func(day string, kind core.SessionKind) int {
		n := 0
		for _, s := range sessionsOf(day) {
			if s.Kind == kind {
				n++
			}
		}
		return n
	}
	for _, r := range sheet.Days {
		if count(r.Day, core.SessionWork) == 0 {
			continue
		}
		if net := int(core.Total(core.NetWork(sessionsOf(r.Day))).Seconds()); net != *r.ReportedWorked {
			t.Errorf("sessionsMatchHRmasterTotals %s: %d vs %d", r.Day, net, *r.ReportedWorked)
		}
	}
	if count("2026-03-12", core.SessionWork) != 3 || count("2026-03-26", core.SessionPause) != 2 ||
		ts.Days.HM(sheet.Sessions[0].Start) != "08:11" {
		t.Error("session counts")
	}

	rowsOnly := parseHR("<table>" + hrRow("05", "Workday", "csütörtök", "Munkanap", "08:00", "00:00", [][2]string{{"08:00", "16:00"}}, nil) + "</table>")
	if rowsOnly.Days[0].Day != "2026-11-05" || len(rowsOnly.Warnings) != 1 {
		t.Error("missingHeaderFallsBackToRangeMonth")
	}

	t.Run("targetsFollowDayStatus", func(t *testing.T) {
		b := fixedHours("HR-1")
		input := func(day string) core.DayInput {
			return core.DayInput{Day: day, Sessions: sessionsOf(day), Info: days[day]}
		}
		target := func(day string) *int {
			w, fromSessions := b.WorkWindows(input(day))
			return b.DailyTarget(day, days[day], w, fromSessions)
		}
		for day, want := range map[string]int{
			"2026-03-02": 7*3600 + 1800, "2026-03-09": 9*3600 + 2700, "2026-03-12": 10*3600 + 900, "2026-03-03": 8 * 3600,
		} {
			if got := target(day); got == nil || *got != want {
				t.Errorf("%s: %v", day, got)
			}
		}
		for _, day := range []string{"2026-03-01", "2026-03-04", "2026-03-15"} {
			if target(day) != nil {
				t.Errorf("%s has a target", day)
			}
		}
		leave := only(t, b.Build(input("2026-03-04")))
		if str(leave.IssueKey) != "HR-1" || leave.Duration != 8*3600 || leave.Source != core.SourceAbsence {
			t.Errorf("%+v", leave)
		}
		if len(b.Build(input("2026-03-15"))) != 0 {
			t.Error("holiday without hours")
		}
	})

	t.Run("leaveDayCommitsMoveToClosestWorkday", func(t *testing.T) {
		b := fixedHours("HR-1")
		info := func(d string) *core.DayRecord { return days[d] }
		no := func(string) bool { return false }
		if b.CommitDay("2026-03-04", info, no) != "2026-03-03" || b.CommitDay("2026-03-02", info, no) != "2026-03-02" ||
			b.CommitDay("2026-03-07", info, no) != "2026-03-06" || b.CommitDay("2026-03-08", info, no) != "2026-03-09" ||
			b.CommitDay("2026-03-07", info, func(d string) bool { return d == "2026-03-07" }) != "2026-03-07" {
			t.Error("commitDay")
		}
		commit := ts.Commit("2026-03-04", "10:00", ts.Hash("abc"), ts.Subject("PROJ-1 fix"))
		leave := only(t, b.Build(core.DayInput{Day: "2026-03-04", Commits: []core.StoredCommit{commit}, Info: days["2026-03-04"]}))
		if leave.Source != core.SourceAbsence || leave.Duration != 8*3600 {
			t.Errorf("%+v", leave)
		}
		moved := only(t, b.Build(core.DayInput{Day: "2026-03-03", Commits: []core.StoredCommit{commit}}))
		if moved.Source != core.SourceGit || moved.Duration != 8*3600 || moved.Day != "2026-03-03" {
			t.Errorf("%+v", moved)
		}
	})

	t.Run("periodFillsOnlyWhatIsMissingFromWorkdays", func(t *testing.T) {
		b := fixedHours("")
		b.Settings.Periods = []core.ProjectPeriod{{ID: "p1", Title: "Migration", IssueKey: "PROJ-200", From: "2026-03-02", To: "2026-03-06", Comment: " Migration work "}}
		empty := only(t, b.Build(core.DayInput{Day: "2026-03-03"}))
		if str(empty.IssueKey) != "PROJ-200" || empty.Duration != 8*3600 || empty.Description != "Migration" ||
			empty.Source != core.SourcePeriod || empty.Comment != "Migration work" {
			t.Errorf("%+v", empty)
		}
		// The commit keeps its ticket and its estimate; the period takes the rest of the 8 hours.
		commit := ts.Commit("2026-03-03", "10:00", ts.Hash("abc"), ts.Subject("PROJ-1 fix rounding"))
		logs := b.Build(core.DayInput{
			Day: "2026-03-03", Commits: []core.StoredCommit{commit},
			Meetings: []core.Meeting{ts.Meeting("2026-03-03", "13:00", "14:00", "Planning")},
		})
		byIssue := map[string]core.Worklog{}
		for _, w := range logs {
			byIssue[str(w.IssueKey)+"|"+string(w.Source)] = w
		}
		git, fill := byIssue["PROJ-1|git"], byIssue["PROJ-200|period"]
		if len(logs) != 3 || git.Duration != core.Allocate([]int{git.Estimated}, nil, g)[0] ||
			git.Duration+fill.Duration != 7*3600 || core.TotalDuration(logs) != 8*3600 {
			t.Errorf("%+v", logs)
		}
		// Work that runs over the day is squeezed to fit and leaves nothing to the period.
		long := ts.Commit("2026-03-03", "17:00", ts.Hash("def"), ts.Subject("PROJ-2 big change"))
		for _, w := range b.Build(core.DayInput{Day: "2026-03-03", Commits: []core.StoredCommit{long}, Kept: []core.Worklog{
			{Day: "2026-03-03", Start: ts.At("2026-03-03", "09:00"), Duration: 7*3600 + 1800, IssueKey: p("PROJ-9"), Source: core.SourceManual},
		}}) {
			if w.Source == core.SourcePeriod || w.Duration != 1800 {
				t.Errorf("fullDay: %+v", w)
			}
		}
		if len(b.Build(core.DayInput{Day: "2026-03-03", Kept: []core.Worklog{empty}})) != 0 ||
			len(b.Build(core.DayInput{Day: "2026-03-07"})) != 0 || len(b.Build(core.DayInput{Day: "2026-03-09"})) != 0 {
			t.Error("locked fill, weekend or outside period got entries")
		}
	})

	if core.DecodeEntities("a&nbsp;&#233;&#x151;&amp;") != "a éő&" {
		t.Error("decodeEntities")
	}
}

func TestSettings(t *testing.T) {
	id := func() string { return "NEW-ID" }
	s, ok := core.DecodeSettings([]byte(`{"fixedDailyHours": 6}`), id)
	if !ok || s.FixedDailyHours != 6 || s.GranularityMinutes != def().GranularityMinutes || len(s.JiraAccounts) != 0 {
		t.Error("missingFieldsFallBackToDefaults")
	}

	s, _ = core.DecodeSettings([]byte(`{"jiraBaseURL": "https://acme.atlassian.net/", "jiraEmail": "me@x.hu", "jiraAccountId": "a1"}`), id)
	a := s.DefaultJiraAccount()
	if len(s.JiraAccounts) != 1 || a == nil || *s.DefaultJiraAccountID != a.ID || a.Site() != "https://acme.atlassian.net" ||
		a.Email != "me@x.hu" || a.AccountID != "a1" || a.TempoBaseURL != "https://api.tempo.io/4" || a.Provider != core.ProviderTempo {
		t.Fatalf("theOldSingleJiraSiteBecomesTheDefaultAccount: %+v", s.JiraAccounts)
	}
	data, _ := json.Marshal(s)
	if again, _ := core.DecodeSettings(data, func() string { return "OTHER" }); !reflect.DeepEqual(again, s) {
		t.Error("migrated a second time")
	}

	s = def()
	work := core.NewJiraAccount("w")
	work.ProjectKeys = []string{"PROJ", "INT"}
	client := core.NewJiraAccount("c")
	client.Provider, client.ProjectKeys = core.ProviderJira, []string{"ACME"}
	s.JiraAccounts = []core.JiraAccount{work, client}
	s.DefaultJiraAccountID = p("c")
	if s.JiraAccountFor("PROJ-12").ID != "w" || s.JiraAccountFor("acme-3").ID != "c" || s.JiraAccountFor("HR-1").ID != "c" {
		t.Error("issuesGoToTheAccountOwningTheirProject")
	}
	s.DefaultJiraAccountID = nil
	if s.JiraAccountFor("HR-1").ID != "w" {
		t.Error("no default picked: the first")
	}

	s = def()
	s.AuthorEmails = []string{"me@example.com"}
	s.Periods = []core.ProjectPeriod{{ID: "p", Title: "Migration", IssueKey: "PROJ-200", From: "2026-03-02", To: "2026-03-13"}}
	s.MeetingRules = []core.MeetingRule{{ID: "r", Contains: "standup", IssueKey: "INT-12"}}
	s.KnownModules = core.AllModules
	data, _ = json.Marshal(s)
	if again, _ := core.DecodeSettings(data, id); !reflect.DeepEqual(again, s) {
		t.Error("roundTrip")
	}

	if s, _ := core.DecodeSettings([]byte(`{"roundingMode": "sessions"}`), id); s.RoundingMode != core.RoundingSessions {
		t.Error("legacyRoundingModeStillDecodes")
	}
	for _, j := range []string{"not json", "[1, 2]", `{"fixedDailyHours": "eight"}`} {
		if _, ok := core.DecodeSettings([]byte(j), id); ok {
			t.Errorf("invalidDataGivesNil %q", j)
		}
	}
	if s, _ := core.DecodeSettings([]byte(`{"knownModules":["git","calendar","workTimes"],"modules":["git"]}`), id); !s.Has(core.ModuleChats) || s.Has(core.ModuleCalendar) || s.Has(core.ModuleJira) {
		t.Errorf("modules added later start switched on, opt-in ones off: %v", s.Modules)
	}

	pp := core.ProjectPeriod{From: "2026-03-02", To: "2026-03-06"}
	if !pp.Contains("2026-03-02") || !pp.Contains("2026-03-06") || pp.Contains("2026-03-07") || pp.Contains("2026-03-01") {
		t.Error("periodsAreInclusive")
	}
}

const chatJSON = `{"source":"google-chat","version":4,"conversations":[
  {"id":"space/A","type":"space","name":null,"oneOnOne":false,"messages":[
    {"time":"2026-03-03T08:00:00.000Z","sender":"me@x","me":true,"text":"Ön: hi","extra":"PROJ | Project | , | a.png | b c.xlsx"},
    {"time":"2026-03-03T08:05:00.000Z","sender":null,"me":false,"text":"yo","extra":"PROJ | Project | ,"},
    {"time":"2026-03-03T09:00:00.000Z","sender":"me@x","me":true,"text":"later"}]},
  {"id":"dm/B","type":"dm","name":"Bea","oneOnOne":true,"messages":[
    {"time":"2026-03-04T10:00:00Z","sender":"me@x","me":true,"text":"ping","extra":"Busy"}]}]}`

// ChatPacked is what PHP's gzcompress + base64_encode produce, minus the adler32 trailer.
func ChatPacked() string {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte(chatJSON))
	_ = w.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes()[:buf.Len()-4])
}

func TestChatExport(t *testing.T) {
	a, err := core.ParseChatExport(ChatPacked())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := core.ParseChatExport(chatJSON)
	if a.Conversations[0].DisplayName() != "PROJ" || a.Conversations[1].DisplayName() != "Bea" ||
		len(b.Conversations[0].Messages) != 3 || len(b.Conversations[1].Messages) != 1 {
		t.Error("parsesBase64ZlibAndPlainJSON")
	}
	first := b.Conversations[0].Messages[0]
	if first.Body() != "hi" || !reflect.DeepEqual(first.Files(), []string{"a.png", "b c.xlsx"}) ||
		len(b.Conversations[0].Messages[1].Files()) != 0 {
		t.Errorf("body/files: %q %v", first.Body(), first.Files())
	}
	if _, err := core.ParseChatExport("not an export"); err == nil {
		t.Error("garbage parsed")
	}

	t0 := time.Unix(0, 0)
	day := core.ChatDay{Times: []time.Time{t0, t0.Add(300 * time.Second), t0.Add(3600 * time.Second)}, Gap: core.DefaultChatGap}
	if day.ActiveSeconds() != 300+60 {
		t.Errorf("activeTimeSplitsAtLongGaps: %d", day.ActiveSeconds())
	}
}
