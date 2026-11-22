package app

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
	"github.com/molnarroland/WorkLogger/internal/store"
	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Today is 2026-03-10 (newService).
func TestIssueOptions(t *testing.T) {
	s, _ := newService(t)
	insert := func(day, key string) {
		w := ts.Worklog(day, "09:00", 3600, core.SourceManual, "", false)
		w.IssueKey = core.NormalizeKeyString(key)
		_, err := s.Store.InsertWorklog(w)
		must(t, err)
	}
	insert("2025-10-01", "OLD-1") // more than the look-back before March
	insert("2026-01-15", "JAN-1")
	insert("2026-03-02", "MAR-1")
	insert("2026-03-02", "MAR-1")
	insert("2026-03-05", "DAY-1")
	insert("2026-03-28", "LATE-1") // later in the month: offered too
	insert("2026-04-02", "APR-1")  // the next month: not
	insert("2026-03-05", "")
	must(t, s.update(func(st *core.Settings) {
		st.Periods = []core.ProjectPeriod{{ID: "p", Title: "Migration", IssueKey: "PER-1", From: "2026-03-01", To: "2026-03-31"}}
	}))
	must(t, s.Store.SyncJiraActivity(ts.At("2026-03-01", "00:00"), ts.At("2026-03-31", "00:00"),
		[]store.JiraActivity{{IssueKey: "MAR-1", Summary: "Fix the login", At: ts.At("2026-03-02", "10:00")}}, true))

	got, err := s.IssueOptions("2026-03-05")
	must(t, err)
	var keys []string
	for _, o := range got {
		keys = append(keys, o.Origin+":"+o.Key)
	}
	want := []string{"day:DAY-1", "period:PER-1", "recent:LATE-1", "recent:MAR-1", "recent:JAN-1"}
	if !slices.Equal(keys, want) {
		t.Fatalf("got %v, want %v", keys, want)
	}
	if mar := got[3]; mar.Summary != "Fix the login" || mar.Uses != 2 || mar.Day != "2026-03-02" {
		t.Errorf("MAR-1: %+v", mar)
	}
	if got[1].Summary != "Migration" {
		t.Errorf("a period's title is its summary: %+v", got[1])
	}
	if _, err := s.IssueOptions("March"); err == nil {
		t.Error("not a day")
	}
}

func TestSearchJiraIssues(t *testing.T) {
	s, _ := newService(t)
	a := core.NewJiraAccount("j1")
	a.BaseURL, a.Email, a.AccountID = "https://acme.atlassian.net", "me@example.com", "acc"
	var jql string
	s.HTTP = &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/issue/ORB-7"):
			return 200, `{"key":"ORB-7","fields":{"summary":"Typed in full","created":"2025-12-01T09:00:00.000+0100"}}`
		case strings.HasSuffix(r.URL.Path, "/search/jql"):
			jql = r.URL.Query().Get("jql")
			return 200, `{"issues":[` +
				`{"key":"ORB-9","fields":{"summary":"Far","created":"2026-03-20T09:00:00.000+0100","status":{"name":"To Do"}}},` +
				`{"key":"ORB-8","fields":{"summary":"Near","created":"2026-03-04T09:00:00.000+0100","status":{"name":"Done"},` +
				`"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Fix the\n  thing."}]},` +
				`{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},` +
				`{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"two"}]}]}]}]}}},` +
				`{"key":"ORB-7","fields":{"summary":"Typed in full","created":"2025-12-01T09:00:00.000+0100"}}]}`
		}
		return 404, `{}`
	}}
	if _, err := s.SearchJiraIssues("2026-03-05", "", 2); err == nil {
		t.Error("searching while the keyring is locked")
	}
	s.Secrets.Unlock()
	if _, err := s.SearchJiraIssues("2026-03-05", "", 2); err == nil || !strings.Contains(err.Error(), "Jira profile") {
		t.Errorf("searching without a Jira profile: %v", err)
	}
	must(t, s.Secrets.Set(a.JiraTokenKey(), "token"))
	must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a} }))

	got, err := s.SearchJiraIssues("2026-03-05", "orb-7", 2)
	must(t, err)
	if want := `created >= "2026-02-19" AND created < "2026-03-20" AND summary ~ "orb*" AND summary ~ "7*" ORDER BY created DESC`; jql != want {
		t.Errorf("jql:\n got %s\nwant %s", jql, want)
	}
	var keys []string
	for _, o := range got {
		keys = append(keys, o.Key)
	}
	if !slices.Equal(keys, []string{"ORB-7", "ORB-8", "ORB-9"}) {
		t.Fatalf("the typed key, then nearest to the day: %v", keys)
	}
	if o := got[1]; o.Origin != "jira" || o.Day != "2026-03-04" || o.Status != "Done" || o.Account != a.ID || o.AccountName != "" ||
		o.Description != "Fix the thing. one two" {
		t.Errorf("ORB-8: %+v", o)
	}
	if got[0].Description != "" || got[2].Description != "" {
		t.Errorf("no description: %q %q", got[0].Description, got[2].Description)
	}
}

func TestWorklogCommits(t *testing.T) {
	s, _ := newService(t)
	must(t, s.Store.AddRepo("/code/billing", "billing"))
	repos, err := s.Store.Repos()
	must(t, err)
	id := repos[0].ID
	must(t, s.Store.SaveCommits([]core.StoredCommit{
		{RepoID: id, Hash: "aaaa1111", Date: ts.At("2026-03-03", "10:00"), Subject: "PROJ-1 First", FilesChanged: 2, Insertions: 5},
		{RepoID: id, Hash: "bbbb2222", Date: ts.At("2026-03-03", "11:00"), Subject: "PROJ-1 Second", Body: "Details"},
		{RepoID: id, Hash: "cccc3333", Date: ts.At("2026-03-03", "12:00"), Subject: "Other"},
	}))
	w := ts.Worklog("2026-03-03", "09:00", 3600, core.SourceGit, "bbbb2222,aaaa1111", false)
	w.RepoID = &id
	wid, err := s.Store.InsertWorklog(w)
	must(t, err)
	got, err := s.WorklogCommits(wid)
	must(t, err)
	if len(got) != 2 || got[0].Hash != "aaaa1111" || got[0].Repo != "billing" || got[0].Files != 2 || got[1].Body != "Details" {
		t.Fatalf("commits: %+v", got)
	}
	meeting, err := s.Store.InsertWorklog(ts.Worklog("2026-03-03", "13:00", 1800, core.SourceMeeting, "ev1", false))
	must(t, err)
	if got, err := s.WorklogCommits(meeting); err != nil || len(got) != 0 {
		t.Errorf("a meeting has no commits: %v %v", got, err)
	}

	s.Git = git.Git{Run: fakeShow{}}
	files, err := s.CommitFiles(id, "aaaa1111")
	must(t, err)
	if len(files) != 1 || files[0].Path != "src/a.go" || files[0].Added != 3 {
		t.Errorf("files: %+v", files)
	}
	diff, err := s.CommitDiff(id, "aaaa1111", "src/a.go")
	must(t, err)
	if !strings.Contains(diff.Text, "+new") {
		t.Errorf("diff: %+v", diff)
	}
	if _, err := s.CommitFiles(id+1, "aaaa1111"); err == nil || !strings.Contains(err.Error(), "repository") {
		t.Errorf("a repository that is gone: %v", err)
	}
	if _, err := s.CommitFiles(id, "deadbeef"); err == nil || !strings.Contains(err.Error(), "billing") {
		t.Errorf("a commit git can't find: %v", err)
	}
}

type fakeShow struct{}

func (fakeShow) Run(_ context.Context, _ string, args ...string) (string, error) {
	if slices.Contains(args, "deadbeef") {
		return "", context.DeadlineExceeded
	}
	if slices.Contains(args, "--numstat") {
		return "3\t1\tsrc/a.go\n", nil
	}
	return "diff --git a/src/a.go b/src/a.go\n-old\n+new\n", nil
}
