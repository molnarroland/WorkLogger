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

// RELEASE_REVIEW C3: the month is a cheap read; Tempo is read back only when asked, and a failure isn't kept.
func TestTempoReadBack(t *testing.T) {
	db, err := store.Open(":memory:", ts.Days)
	must(t, err)
	t.Cleanup(func() { db.Close() })
	a := core.NewJiraAccount("j1")
	a.BaseURL, a.Email, a.AccountID, a.ProjectKeys = "https://acme.atlassian.net", "me@example.com", "acc", []string{"ORB"}
	failing := false
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		if failing {
			return 500, `{}`
		}
		if strings.Contains(r.URL.Path, "/rest/api/3/issue/10001") {
			return 200, `{"id":"10001","key":"ORB-1"}`
		}
		return 200, `{"metadata":{},"results":[{"tempoWorklogId":1,"issue":{"id":10001},"timeSpentSeconds":3600,` +
			`"startDate":"2026-03-03","startTime":"09:00:00"}]}`
	}}
	secrets := ts.NewSecrets(map[string]string{a.JiraTokenKey(): "jira-token", a.TempoTokenKey(): "tempo-token"})
	s := New(Deps{
		Store: db, Days: ts.Days, Clock: ts.FixedClock{T: ts.At("2026-03-10", "12:00")},
		Secrets: &keyring{secrets, false}, HTTP: h, Git: git.Git{Run: gitRunner{}},
	})
	must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a} }))
	w := ts.Worklog("2026-03-03", "09:00", 3600, core.SourceGit, "", false)
	w.IssueKey = core.Ptr("ORB-1")
	_, err = db.InsertWorklog(w)
	must(t, err)

	inTempo := func() bool {
		m, err := s.Month("2026-03")
		must(t, err)
		return len(m.InTempo) == 1 && m.Worklogs[0].InTempo
	}
	requests := func() int { return len(h.Requests) }

	t.Run("the month never goes to the network", func(t *testing.T) {
		if inTempo() || requests() != 0 {
			t.Fatalf("in Tempo before a read-back, or %d requests", requests())
		}
	})
	t.Run("a read-back fills it in and says it changed", func(t *testing.T) {
		changed, err := s.RefreshTempo("2026-03")
		must(t, err)
		if !changed || !inTempo() {
			t.Fatalf("changed %v, in Tempo %v", changed, inTempo())
		}
		n := requests()
		_, _ = s.Month("2026-03")
		if requests() != n {
			t.Error("the month went to the network after the read-back")
		}
		changed, err = s.RefreshTempo("2026-03")
		must(t, err)
		if changed {
			t.Error("a second read-back with the same answer reports a change")
		}
	})
	t.Run("a failed read-back is reported and keeps the last good answer", func(t *testing.T) {
		failing = true
		if _, err := s.RefreshTempo("2026-03"); err == nil {
			t.Error("no error")
		}
		if !inTempo() {
			t.Error("a failure replaced the last good answer")
		}
	})
}
