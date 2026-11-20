package app

import (
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// The backend enforces what the menus only hide (RELEASE_REVIEW B5).
func TestRulesTheBackendEnforces(t *testing.T) {
	s, _ := newService(t)
	add := func(day string, tempoID *int64, status core.Status) WorklogDTO {
		w := core.Worklog{Day: day, Start: ts.At(day, "09:00"), Duration: 3600, IssueKey: core.Ptr("PROJ-1"), Source: core.SourceManual, Status: status, TempoID: tempoID, Comment: "x"}
		id, err := s.Store.InsertWorklog(w)
		must(t, err)
		w.ID = id
		return s.worklogDTO(w)
	}
	sent := add("2026-03-03", core.Ptr[int64](42), core.StatusExported)
	byHand := add("2026-03-04", nil, core.StatusExported)
	draft := add("2026-03-05", nil, core.StatusDraft)

	t.Run("an entry sent to Tempo can't be deleted, alone or in a batch", func(t *testing.T) {
		if err := s.DeleteWorklog(sent.ID); err != errSent {
			t.Errorf("delete: %v", err)
		}
		if err := s.DeleteWorklogs([]int64{draft.ID, sent.ID}); err != errSent {
			t.Errorf("batch: %v", err)
		}
		if _, err := s.one(draft.ID); err != nil {
			t.Error("a refused batch removed nothing")
		}
	})
	t.Run("an entry sent to Tempo keeps its time, ticket and status", func(t *testing.T) {
		w := sent
		w.Duration = 1800
		if _, err := s.UpdateWorklog(w); err != errSent {
			t.Errorf("duration: %v", err)
		}
		w = sent
		w.Status = string(core.StatusReviewed)
		if _, err := s.UpdateWorklog(w); err != errSent {
			t.Errorf("status: %v", err)
		}
		w = sent
		w.Locked = !w.Locked
		w.Description = "note"
		if _, err := s.UpdateWorklog(w); err != nil {
			t.Errorf("lock and note may change: %v", err)
		}
	})
	t.Run("an entry ticked off by hand stays editable", func(t *testing.T) {
		w := byHand
		w.Status = string(core.StatusReviewed)
		if _, err := s.UpdateWorklog(w); err != nil {
			t.Error(err)
		}
	})
	t.Run("the start must be on the entry's day", func(t *testing.T) {
		w := draft
		w.Start = rfc(ts.At("2026-03-06", "09:00"))
		if _, err := s.UpdateWorklog(w); err == nil || !strings.Contains(err.Error(), "not on") {
			t.Errorf("got %v", err)
		}
	})
	t.Run("deleting a missing entry is an error", func(t *testing.T) {
		if err := s.DeleteWorklog(99999); err == nil {
			t.Error("no error")
		}
	})
	t.Run("a period can't end before it starts", func(t *testing.T) {
		st := s.Settings()
		st.Periods = []core.ProjectPeriod{{ID: "p", Title: "Backwards", IssueKey: "PROJ-1", From: "2026-03-10", To: "2026-03-01"}}
		if _, err := s.SaveSettings(st); err == nil {
			t.Error("saved")
		}
	})
}

func TestUseDescriptionsAsComments(t *testing.T) {
	s, _ := newService(t)
	day := "2026-03-03"
	ins := func(desc, comment string, st core.Status) int64 {
		id, err := s.Store.InsertWorklog(core.Worklog{Day: day, Start: ts.At(day, "09:00"), Duration: 600, Source: core.SourceManual, Status: st, Description: desc, Comment: comment})
		must(t, err)
		return id
	}
	plain := ins("Fix rounding", "", core.StatusDraft)
	kept := ins("Generated", "My own words", core.StatusDraft)
	blank := ins("  ", "", core.StatusDraft)
	must(t, s.UseDescriptionsAsComments(day))
	for id, want := range map[int64]string{plain: "Fix rounding", kept: "My own words", blank: ""} {
		w, err := s.one(id)
		must(t, err)
		if w.Comment != want {
			t.Errorf("%d: comment %q, want %q", id, w.Comment, want)
		}
	}
	if w, _ := s.one(plain); !w.Locked {
		t.Error("an entry given a comment locks")
	}
}

func TestWeekCrossesTheMonthEnd(t *testing.T) {
	s, _ := newService(t)
	w, err := s.Week("2026-03-31") // Tuesday; the week runs to Sunday 5 April
	must(t, err)
	if len(w.Days) != 7 || w.Days[0].Day != "2026-03-30" || w.Days[6].Day != "2026-04-05" || w.Month != "2026-04" {
		t.Errorf("week: %s–%s, month %s", w.Days[0].Day, w.Days[6].Day, w.Month)
	}
}

func TestSetDurationParsesWhatWasTyped(t *testing.T) {
	s, _ := newService(t)
	w, err := s.AddWorklog("2026-03-03")
	must(t, err)
	got, err := s.SetDuration(w.ID, " 1,5h ")
	must(t, err)
	if got.Duration != 5400 || !got.Locked {
		t.Errorf("%+v", got)
	}
	if _, err := s.SetDuration(w.ID, "soon"); err == nil {
		t.Error("accepted")
	}
}
