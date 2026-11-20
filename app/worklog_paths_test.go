package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// RELEASE_REVIEW C12: the edits' failure paths and the cases they leave alone.
func TestWorklogEditPaths(t *testing.T) {
	s, _ := newService(t)
	add := func(day string, st core.Status, tempoID *int64) WorklogDTO {
		w := core.Worklog{
			Day: day, Start: ts.At(day, "09:00"), Duration: 1800, IssueKey: core.Ptr("ORB-1"),
			Source: core.SourceManual, Status: st, TempoID: tempoID, Comment: "x",
		}
		id, err := s.Store.InsertWorklog(w)
		must(t, err)
		w.ID = id
		return s.worklogDTO(w)
	}
	draft := add("2026-03-03", core.StatusDraft, nil)
	byHand := add("2026-03-04", core.StatusExported, nil)
	sent := add("2026-03-05", core.StatusExported, core.Ptr[int64](7))

	t.Run("UpdateWorklog refuses bad input", func(t *testing.T) {
		for name, change := range map[string]func(*WorklogDTO){
			"a start that isn't a time": func(w *WorklogDTO) { w.Start = "nine" },
			"a negative duration":       func(w *WorklogDTO) { w.Duration = -1 },
			"a day that isn't a day":    func(w *WorklogDTO) { w.Day = "2026-13-40" },
			"an unknown status":         func(w *WorklogDTO) { w.Status = "done" },
			"a missing entry":           func(w *WorklogDTO) { w.ID = 9999 },
		} {
			w := draft
			change(&w)
			if _, err := s.UpdateWorklog(w); err == nil {
				t.Errorf("%s: accepted", name)
			}
		}
	})
	t.Run("SetStatus refuses an unknown status and leaves entries in Tempo", func(t *testing.T) {
		if err := s.SetStatus([]int64{draft.ID}, "done"); err == nil {
			t.Error("unknown status accepted")
		}
		must(t, s.SetStatus([]int64{byHand.ID, sent.ID}, "draft"))
		for _, id := range []int64{byHand.ID, sent.ID} {
			if w, _ := s.one(id); w.Status != core.StatusExported {
				t.Errorf("%d moved back to %s", id, w.Status)
			}
		}
	})
	t.Run("SetEntered unticks what was entered by hand, not what the API sent", func(t *testing.T) {
		must(t, s.SetEntered([]int64{byHand.ID, sent.ID}, false))
		if w, _ := s.one(byHand.ID); w.Status != core.StatusReviewed {
			t.Errorf("by hand: %s", w.Status)
		}
		if w, _ := s.one(sent.ID); w.Status != core.StatusExported {
			t.Errorf("sent: %s", w.Status)
		}
	})
	t.Run("MoveWorklogs refuses a bad day and leaves exported entries where they are", func(t *testing.T) {
		if err := s.MoveWorklogs([]int64{draft.ID}, "someday"); err == nil {
			t.Error("bad day accepted")
		}
		must(t, s.MoveWorklogs([]int64{draft.ID, sent.ID}, "2026-03-06"))
		if w, _ := s.one(draft.ID); w.Day != "2026-03-06" || !w.Locked || s.Days.Key(w.Start) != "2026-03-06" {
			t.Errorf("draft: %+v", w)
		}
		if w, _ := s.one(sent.ID); w.Day != "2026-03-05" {
			t.Errorf("sent entry moved to %s", w.Day)
		}
	})
	t.Run("PushDayToTempo refuses a bad day, a future day and a day with drafts", func(t *testing.T) {
		if err := s.PushDayToTempo("someday"); err == nil {
			t.Error("bad day accepted")
		}
		var ue *core.UserError
		if err := s.PushDayToTempo("2026-03-11"); !errors.As(err, &ue) || ue.Code != "push.future" {
			t.Errorf("future day: %v", err)
		}
		if err := s.PushDayToTempo("2026-03-06"); err == nil { // the draft was moved there above
			t.Error("a day with drafts was sent")
		} else if !errors.As(err, &ue) || ue.Code != "push.dayHasDrafts" || ue.Params["n"] != 1 {
			t.Errorf("drafts: %v", err)
		}
	})
	t.Run("PushDayToTempo without a Jira profile is a user error", func(t *testing.T) {
		add("2026-03-02", core.StatusReviewed, nil)
		err := s.PushDayToTempo("2026-03-02")
		var ue *core.UserError
		if !errors.As(err, &ue) || ue.Code != "missing.jiraAccount" {
			t.Errorf("got %v", err)
		}
	})
	t.Run("PushDayToTempo reports the profiles it couldn't send for", func(t *testing.T) {
		s.Unlock()
		a := core.NewJiraAccount("j1")
		a.BaseURL, a.Email, a.ProjectKeys = "https://acme.atlassian.net", "me@example.com", []string{"ORB"}
		other := core.NewJiraAccount("j2")
		other.Provider, other.ProjectKeys = core.ProviderOther, []string{"NOVA"}
		must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a, other} }))
		add("2026-03-09", core.StatusReviewed, nil)
		nova := add("2026-03-09", core.StatusReviewed, nil)
		must(t, s.SetIssue([]int64{nova.ID}, "NOVA-2"))
		err := s.PushDayToTempo("2026-03-09")
		var ue *core.UserError
		if !errors.As(err, &ue) || ue.Code != "push.failed" || ue.Params["n"] != 0 {
			t.Fatalf("got %v", err)
		}
		reason, _ := ue.Params["reason"].(string)
		if !strings.Contains(reason, "logged in another tool") || !strings.Contains(reason, "Jira") {
			t.Errorf("reason: %q", reason)
		}
	})
}
