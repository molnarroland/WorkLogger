package app

import (
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/tempo"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// The page names a profile and a key; only that profile's own site is opened, and only for a real key.
func TestOpenJiraIssue(t *testing.T) {
	s, _ := newService(t)
	var opened []string
	s.Shell.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	must(t, s.update(func(st *core.Settings) {
		st.JiraAccounts = []core.JiraAccount{
			{ID: "a", BaseURL: "https://acme.atlassian.net/"},
			{ID: "b", BaseURL: "javascript:alert(1)"},
		}
	}))
	must(t, s.OpenJiraIssue("a", "PROJ-12"))
	for _, c := range [][2]string{{"a", "PROJ-12/../../evil"}, {"a", "proj-12"}, {"b", "PROJ-12"}, {"x", "PROJ-12"}} {
		if err := s.OpenJiraIssue(c[0], c[1]); err == nil {
			t.Errorf("%v: opened", c)
		}
	}
	if len(opened) != 1 || opened[0] != "https://acme.atlassian.net/browse/PROJ-12" {
		t.Errorf("opened %v", opened)
	}
}

// One day: PROJ-1 moved and commented in the morning (with a file), PROJ-2 commented after lunch. The untagged
// commit around 10:00 goes to PROJ-1, the one at 14:40 to PROJ-2, the evening one to neither.
func TestBuildJiraFeed(t *testing.T) {
	act := func(hm, key, kind, field, to, text string) accountActivity {
		return accountActivity{tempo.Activity{At: ts.At("2026-03-03", hm), IssueKey: key, Summary: key + " summary",
			Kind: kind, Field: field, From: "To Do", To: to, Text: text}, "acme", ""}
	}
	acts := []accountActivity{
		act("15:00", "PROJ-2", "comment", "", "", "Looks good"),
		act("10:00", "PROJ-1", "change", "status", "In Progress", ""),
		act("11:00", "PROJ-1", "comment", "", "", "Fixed"),
		act("11:05", "PROJ-1", "change", "Attachment", "trace.log", ""),
		act("11:10", "PROJ-1", "change", "assignee", "Me", ""),
	}
	log := func(id int64, hm string, min int, key *string) core.Worklog {
		return core.Worklog{ID: id, Day: "2026-03-03", Start: ts.At("2026-03-03", hm), Duration: min * 60, IssueKey: key,
			Source: core.SourceGit, Status: core.StatusDraft}
	}
	key := "PROJ-9"
	logs := []core.Worklog{log(1, "09:30", 45, nil), log(2, "14:40", 10, nil), log(3, "20:00", 30, nil), log(4, "10:00", 60, &key)}
	got := buildJiraFeed(JiraActivityDTO{Days: []JiraDayDTO{}}, acts, logs, ts.Days)
	if got.Entries != 5 || got.Tickets != 2 || len(got.Days) != 1 || len(got.Days[0].Tickets) != 2 {
		t.Fatalf("feed: %+v", got)
	}
	t2, t1 := got.Days[0].Tickets[0], got.Days[0].Tickets[1] // the latest activity first
	if t1.IssueKey != "PROJ-1" || t1.First != "10:00" || t1.Last != "11:10" || len(t1.Statuses) != 1 ||
		len(t1.Comments) != 1 || len(t1.Attachments) != 1 || t1.Attachments[0].Text != "trace.log" || len(t1.Changes) != 1 {
		t.Errorf("PROJ-1: %+v", t1)
	}
	if len(t1.Suggested) != 1 || t1.Suggested[0].ID != 1 || len(t2.Suggested) != 1 || t2.Suggested[0].ID != 2 {
		t.Errorf("suggested: PROJ-1 %+v, PROJ-2 %+v", t1.Suggested, t2.Suggested)
	}
}
