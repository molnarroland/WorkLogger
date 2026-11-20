package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
	"github.com/molnarroland/WorkLogger/internal/store"
	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Sending one day by hand: only its reviewed entries go, what Tempo already has for the day is left out, and the
// sent entries are marked exported with their Tempo id.
func TestPushDayToTempo(t *testing.T) {
	db, err := store.Open(":memory:", ts.Days)
	must(t, err)
	t.Cleanup(func() { db.Close() })
	a := core.NewJiraAccount("j1")
	a.BaseURL, a.Email, a.AccountID, a.ProjectKeys = "https://acme.atlassian.net", "me@example.com", "acc", []string{"ORB"}
	var created []string
	readFails := false
	h := &ts.FakeHTTP{Handle: func(r *http.Request, body string) (int, string) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/worklogs/user/acc"):
			if readFails {
				return 500, `{}`
			}
			// ORB-1 is already in Tempo for the day (entered by hand).
			return 200, `{"metadata":{},"results":[{"tempoWorklogId":1,"issue":{"id":10001},"timeSpentSeconds":3600,` +
				`"startDate":"2026-03-03","startTime":"09:00:00"}]}`
		case strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/10001"):
			return 200, `{"id":"10001","key":"ORB-1"}`
		case strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/ORB-2"):
			return 200, `{"id":"10002","key":"ORB-2"}`
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/worklogs"):
			created = append(created, body)
			return 200, `{"tempoWorklogId":77}`
		}
		return 404, `{}`
	}}
	secrets := ts.NewSecrets(map[string]string{a.JiraTokenKey(): "jira-token", a.TempoTokenKey(): "tempo-token"})
	s := New(Deps{
		Store: db, Days: ts.Days, Clock: ts.FixedClock{T: ts.At("2026-03-10", "12:00")},
		Secrets: &keyring{secrets, false}, HTTP: h, Git: git.Git{Run: gitRunner{}},
	})
	must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a} }))
	add := func(day, hm, key string, seconds int, st core.Status) int64 {
		w := ts.Worklog(day, hm, seconds, core.SourceManual, "", false)
		w.IssueKey, w.Status, w.Comment = core.Ptr(key), st, "work"
		id, err := db.InsertWorklog(w)
		must(t, err)
		return id
	}
	there := add("2026-03-03", "09:00", "ORB-1", 3600, core.StatusReviewed)
	fresh := add("2026-03-03", "10:00", "ORB-2", 1800, core.StatusReviewed)
	other := add("2026-03-04", "09:00", "ORB-2", 1800, core.StatusReviewed)

	t.Run("a failed read-back sends nothing", func(t *testing.T) {
		readFails = true
		defer func() { readFails = false }()
		var ue *core.UserError
		if err := s.PushDayToTempo("2026-03-03"); !errors.As(err, &ue) || ue.Code != "push.failed" || ue.Params["n"] != 0 {
			t.Fatalf("got %v", err)
		}
		if len(created) != 0 {
			t.Errorf("sent %d worklogs blind", len(created))
		}
	})
	t.Run("only the day's entries Tempo doesn't have yet are sent", func(t *testing.T) {
		must(t, s.PushDayToTempo("2026-03-03"))
		if len(created) != 1 || !strings.Contains(created[0], `"issueId":10002`) ||
			!strings.Contains(created[0], `"startDate":"2026-03-03"`) {
			t.Fatalf("created: %v", created)
		}
		if w, _ := s.one(fresh); w.Status != core.StatusExported || w.TempoID == nil || *w.TempoID != 77 {
			t.Errorf("sent entry: %+v", w)
		}
		if w, _ := s.one(there); w.Status != core.StatusReviewed || w.TempoID != nil {
			t.Errorf("entry already in Tempo: %+v", w)
		}
		if w, _ := s.one(other); w.Status != core.StatusReviewed {
			t.Errorf("another day's entry: %+v", w)
		}
		if !lastActivity(s, "Sent 1 worklogs for 2026-03-03") || !lastActivity(s, "1 already there") {
			t.Errorf("activity: %+v", s.Activity())
		}
	})
	t.Run("sending the day again sends nothing twice", func(t *testing.T) {
		must(t, s.PushDayToTempo("2026-03-03"))
		if len(created) != 1 {
			t.Errorf("created: %v", created)
		}
	})
}
