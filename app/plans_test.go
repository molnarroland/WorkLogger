package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
	"github.com/molnarroland/WorkLogger/internal/store"
	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Tempo's plans (and worklogs WorkLogger didn't send) are read back with the worklogs, shown next to the entries
// on the same ticket and day, and suggest a ticket for the day's entries without one.
func TestTempoPlans(t *testing.T) {
	db, err := store.Open(":memory:", ts.Days)
	must(t, err)
	t.Cleanup(func() { db.Close() })
	a := core.NewJiraAccount("j1")
	a.BaseURL, a.Email, a.AccountID, a.ProjectKeys = "https://acme.atlassian.net", "me@example.com", "acc", []string{"ORB"}
	a.TempoBaseURL = "https://api.tempo.io/4"
	plansFail := false
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case strings.Contains(r.URL.Path, "/rest/api/3/issue/10001"):
			return 200, `{"id":"10001","key":"ORB-1"}`
		case strings.Contains(r.URL.Path, "/rest/api/3/issue/10002"):
			return 200, `{"id":"10002","key":"ORB-2"}`
		case strings.Contains(r.URL.Path, "/plans/user/acc"):
			if plansFail {
				return 403, `{"errors":[{"message":"no scope"}]}`
			}
			return 200, `{"metadata":{},"results":[` +
				`{"id":7,"planItem":{"id":"10001","type":"ISSUE"},"description":"Moved to In progress",` +
				`"dates":{"values":[{"date":"2026-03-03","timePlannedSeconds":7200},{"date":"2026-03-04","timePlannedSeconds":3600}]}},` +
				`{"id":8,"planItem":{"id":5,"type":"PROJECT"},"dates":{"values":[{"date":"2026-03-03","timePlannedSeconds":3600}]}}]}`
		}
		return 200, `{"metadata":{},"results":[{"tempoWorklogId":99,"issue":{"id":10002},"timeSpentSeconds":900,` +
			`"startDate":"2026-03-03","startTime":"10:00:00","description":"Status change"}]}`
	}}
	secrets := ts.NewSecrets(map[string]string{a.JiraTokenKey(): "jira-token", a.TempoTokenKey(): "tempo-token"})
	s := New(Deps{
		Store: db, Days: ts.Days, Clock: ts.FixedClock{T: ts.At("2026-03-10", "12:00")},
		Secrets: &keyring{secrets, false}, HTTP: h, Git: git.Git{Run: gitRunner{}},
	})
	must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a} }))
	commit := ts.Worklog("2026-03-03", "09:00", 3600, core.SourceGit, "", false)
	commit.IssueKey = core.Ptr("ORB-1")
	_, err = db.InsertWorklog(commit)
	must(t, err)
	chat := ts.Worklog("2026-03-03", "11:00", 1800, core.SourceChat, "", false)
	chat.IssueKey = nil
	_, err = db.InsertWorklog(chat)
	must(t, err)
	lone := ts.Worklog("2026-03-06", "10:00", 1800, core.SourceGit, "", false) // a day with no plans
	lone.IssueKey = nil
	_, err = db.InsertWorklog(lone)
	must(t, err)

	p, err := s.Plans("2026-03")
	must(t, err)
	if p.Read || len(p.Days) != 0 {
		t.Fatalf("plans before a read-back: %+v", p)
	}
	changed, err := s.RefreshTempo("2026-03")
	must(t, err)
	if !changed {
		t.Error("new plans aren't a change")
	}

	p, err = s.Plans("2026-03")
	must(t, err)
	if !p.Read || p.Planned != 3*3600 || len(p.Days) != 3 {
		t.Fatalf("read %v, planned %d, %d days", p.Read, p.Planned, len(p.Days))
	}
	d := p.Days[0]
	if d.Day != "2026-03-03" || len(d.Items) != 2 {
		t.Fatalf("first day: %+v", d)
	}
	plan, outside := d.Items[0], d.Items[1]
	if plan.Kind != "plan" || plan.Key != "ORB-1" || plan.Seconds != 7200 || plan.Logged != 3600 || len(plan.Entries) != 1 ||
		plan.Entries[0].Source != "git" || plan.Description != "Moved to In progress" {
		t.Errorf("plan: %+v", plan)
	}
	if outside.Kind != "tempo" || outside.Key != "ORB-2" || outside.Seconds != 900 {
		t.Errorf("worklog Tempo has: %+v", outside)
	}
	if len(d.Untagged) != 1 || d.Untagged[0].Source != "chat" || d.Suggest == nil || *d.Suggest != "ORB-1" {
		t.Errorf("untagged %+v, suggest %v", d.Untagged, d.Suggest)
	}

	if lone := p.Days[2]; lone.Day != "2026-03-06" || len(lone.Items) != 0 || len(lone.Untagged) != 1 || lone.Suggest != nil {
		t.Errorf("a day with only entries without a ticket: %+v", lone)
	}

	m, err := s.Month("2026-03")
	must(t, err)
	for _, w := range m.Worklogs {
		if w.Source == "chat" && (w.PlanIssue == nil || *w.PlanIssue != "ORB-1") {
			t.Errorf("chat without a ticket isn't offered the planned one: %v", w.PlanIssue)
		}
		if w.Source == "git" && w.PlanIssue != nil {
			t.Error("an entry with a ticket gets a suggestion")
		}
	}
	for _, g := range m.Tickets {
		if g.Key != nil && *g.Key == "ORB-1" && g.Days[0].Planned != 7200 {
			t.Errorf("ticket evidence: planned %d", g.Days[0].Planned)
		}
	}

	t.Run("a token without the Planner scope still reads back the worklogs", func(t *testing.T) {
		plansFail = true
		if _, err := s.RefreshTempo("2026-03"); err != nil {
			t.Fatal(err)
		}
		p, _ := s.Plans("2026-03")
		if p.Planned != 0 || len(p.Days) != 2 || p.Days[0].Items[0].Kind != "tempo" {
			t.Errorf("after a failed plan read: %+v", p)
		}
	})
}
