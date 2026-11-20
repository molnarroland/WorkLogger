package app

import (
	"strings"
	"testing"
)

// RELEASE_REVIEW C4: every job takes the one job slot, ends with collect:done and lands in the activity console.
func TestJobsShareOneSlotAndReport(t *testing.T) {
	s, rec := newService(t)

	t.Run("a rebuild waits for the running job", func(t *testing.T) {
		ctx, done := s.startJob()
		if ctx == nil {
			t.Fatal("no slot")
		}
		if err := s.Rebuild("2026-03-01", "2026-03-31"); err != errBusy {
			t.Errorf("rebuild during a job: %v", err)
		}
		done()
	})
	t.Run("a rebuild ends with collect:done and an activity line", func(t *testing.T) {
		must(t, s.Rebuild("2026-03-01", "2026-03-31"))
		if n := len(rec.events); n == 0 || rec.events[n-1] != EventDone {
			t.Errorf("events: %v", rec.events)
		}
		if !lastActivity(s, "Drafts rebuilt") {
			t.Errorf("activity: %+v", s.Activity())
		}
	})
	t.Run("a push with nothing to send ends the same way", func(t *testing.T) {
		before := len(rec.events)
		must(t, s.PushDayToTempo("2026-01-05"))
		if len(rec.events) == before || rec.events[len(rec.events)-1] != EventDone {
			t.Errorf("events: %v", rec.events)
		}
		if !lastActivity(s, "Sent 0 worklogs") {
			t.Errorf("activity: %+v", s.Activity())
		}
	})
}

func lastActivity(s *Service, text string) bool {
	lines := s.Activity()
	return len(lines) > 0 && strings.Contains(lines[len(lines)-1].Message, text)
}
