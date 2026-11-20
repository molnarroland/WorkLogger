package core

import (
	"testing"
	"time"
)

func TestMergeTimesheetsTakesEachDayFromWhereItWasWorked(t *testing.T) {
	worked := 8 * 3600
	at := func(day string, h int) time.Time {
		d, _ := time.Parse("2006-01-02", day)
		return d.Add(time.Duration(h) * time.Hour)
	}
	a := ParsedTimesheet{
		Sessions: []WorkSession{{Day: "2026-10-05", Start: at("2026-10-05", 9), End: at("2026-10-05", 17), Kind: SessionWork}},
		Days:     []DayRecord{{Day: "2026-10-05", Status: DayWorkday, ReportedWorked: &worked}, {Day: "2026-10-06", Status: DayRestDay}},
	}
	b := ParsedTimesheet{
		Sessions: []WorkSession{{Day: "2026-10-06", Start: at("2026-10-06", 8), End: at("2026-10-06", 12), Kind: SessionWork}},
		Days:     []DayRecord{{Day: "2026-10-05", Status: DayLeave}, {Day: "2026-10-06", Status: DayWorkday}},
	}
	m := MergeTimesheets([]ParsedTimesheet{a, b})
	if len(m.Sessions) != 2 || m.Sessions[0].Day != "2026-10-05" || m.Sessions[1].Day != "2026-10-06" {
		t.Fatalf("sessions %+v", m.Sessions)
	}
	status := map[string]DayStatus{}
	for _, d := range m.Days {
		status[d.Day] = d.Status
	}
	if status["2026-10-05"] != DayWorkday || status["2026-10-06"] != DayWorkday || len(m.Days) != 2 {
		t.Fatalf("days %+v", m.Days)
	}
}

func TestSyncProfilesLinksTempoAndMirrorsLegacyFields(t *testing.T) {
	s := DefaultSettings()
	s.JiraAccounts = []JiraAccount{NewJiraAccount("a"), NewJiraAccount("b")}
	s.Profiles = []Profile{
		{ID: "t1", Kind: ProfileTempo, JiraAccountID: "a", URL: "https://api.eu.tempo.io/4"},
		{ID: "t2", Kind: ProfileTempo, JiraAccountID: "a"},       // second for the same account: dropped
		{ID: "t3", Kind: ProfileTempo, JiraAccountID: "missing"}, // no such account: dropped
		{ID: "g", Kind: ProfileGoogleChat, Email: "me@x.hu"},
		{ID: "h", Kind: ProfileHRmaster, URL: "https://x.hrmaster.hu"},
	}
	s.WorkspaceRoots = []string{" ~/Work ", "", "~/Work", "~/Other"}
	s.SyncProfiles()
	if len(s.Profiles) != 3 {
		t.Fatalf("profiles %+v", s.Profiles)
	}
	if s.JiraAccounts[0].Provider != ProviderTempo || s.JiraAccounts[0].TempoBaseURL != "https://api.eu.tempo.io/4" {
		t.Fatalf("linked account %+v", s.JiraAccounts[0])
	}
	if s.JiraAccounts[1].Provider != ProviderJira {
		t.Fatalf("unlinked account kept Tempo: %+v", s.JiraAccounts[1])
	}
	if s.ChatEmail != "me@x.hu" || s.HrmasterURL != "https://x.hrmaster.hu" {
		t.Fatalf("legacy fields %q %q", s.ChatEmail, s.HrmasterURL)
	}
	if len(s.WorkspaceRoots) != 2 || s.WorkspaceRoot != "~/Work" {
		t.Fatalf("roots %v %q", s.WorkspaceRoots, s.WorkspaceRoot)
	}
}

func TestDecodeSettingsMigratesToProfiles(t *testing.T) {
	data := []byte(`{"hrmasterURL":"https://x.hrmaster.hu","chatEmail":"me@x.hu","workspaceRoot":"~/Work",
		"jiraAccounts":[{"id":"a","provider":"tempo","tempoBaseURL":"https://api.tempo.io/4","projectKeys":[]}]}`)
	s, ok := DecodeSettings(data, func() string { return "new" })
	if !ok {
		t.Fatal("decode failed")
	}
	if _, ok := s.Profile("hrmaster"); !ok {
		t.Fatal("no HRmaster profile")
	}
	if g, ok := s.Profile("googleChat"); !ok || g.Email != "me@x.hu" {
		t.Fatalf("google profile %+v", g)
	}
	if tp := s.ProfilesOf(ProfileTempo); len(tp) != 1 || tp[0].JiraAccountID != "a" {
		t.Fatalf("tempo profiles %+v", tp)
	}
	if len(s.WorkspaceRoots) != 1 || s.WorkspaceRoots[0] != "~/Work" {
		t.Fatalf("roots %v", s.WorkspaceRoots)
	}
}

func TestTempoTextPrefersTheComment(t *testing.T) {
	w := Worklog{Description: "abc1234 fix login"}
	if w.TempoText() != "abc1234 fix login" {
		t.Fatal(w.TempoText())
	}
	w.Comment = "Login fix"
	if w.TempoText() != "Login fix" {
		t.Fatal(w.TempoText())
	}
}
