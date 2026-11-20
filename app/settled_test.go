package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// Today is 2026-03-10: the week of 2–8 March is past. Tempo has 8 hours on each of its workdays.
func TestSettledWeeks(t *testing.T) {
	s, _ := newService(t)
	root := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "billing", ".git"), 0o755))
	st := s.Settings()
	st.WorkspaceRoots = []string{root}
	_, err := s.SaveSettings(st)
	must(t, err)
	must(t, s.ScanWorkspace())

	s.tempoMu.Lock()
	for _, d := range s.Days.Range("2026-03-02", "2026-03-08") {
		s.tempoCache[d] = nil
		if d <= "2026-03-06" {
			s.tempoCache[d] = []tempo.Worklog{{Day: d, Seconds: 8 * 3600}}
		}
	}
	s.tempoMu.Unlock()

	days, err := s.settledDays("2026-03-01", "2026-03-31")
	must(t, err)
	if len(days) != 7 || !days["2026-03-02"] || !days["2026-03-08"] || days["2026-03-01"] {
		t.Fatalf("settled: %v (the week of 1 March was never read from Tempo)", days)
	}
	if m, _ := s.Month("2026-03"); len(m.Settled) != 7 {
		t.Errorf("month settled: %v", m.Settled)
	}
	if f, to, ok := unsettled(s.Days.Range("2026-03-02", "2026-03-12"), days); !ok || f != "2026-03-09" || to != "2026-03-12" {
		t.Errorf("trimmed: %s – %s %v", f, to, ok)
	}

	// Collect leaves the settled week out; forced, it reads it.
	must(t, s.Collect("2026-03-02", "2026-03-08", false))
	if logs, _ := s.Store.Worklogs("2026-03-02", "2026-03-08"); len(logs) != 0 {
		t.Fatalf("a settled week was collected: %d drafts", len(logs))
	}
	must(t, s.Collect("2026-03-02", "2026-03-08", true))
	if logs, _ := s.Store.Worklogs("2026-03-02", "2026-03-08"); len(logs) != 1 {
		t.Fatalf("forced: %d drafts", len(logs))
	}

	// Cleaning up drops the week's commits and keeps its worklogs.
	got, err := s.CleanUpSettled("2026-03-01", "2026-03-31")
	must(t, err)
	if got.Days != 7 || got.Rows != 1 {
		t.Errorf("cleanup: %+v", got)
	}
	if repos, _ := s.Repos("2026-03"); repos[0].Commits != 0 {
		t.Errorf("commits left: %d", repos[0].Commits)
	}
	if logs, _ := s.Store.Worklogs("2026-03-02", "2026-03-08"); len(logs) != 1 {
		t.Errorf("worklogs after cleanup: %d", len(logs))
	}
}
