package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewedNeedsTheTempoComment(t *testing.T) {
	s, _ := newService(t)
	w, err := s.AddWorklog("2026-03-03")
	must(t, err)
	if err := s.SetStatus([]int64{w.ID}, "reviewed"); err == nil {
		t.Fatal("an entry without a comment was marked reviewed")
	}
	w.Status = "reviewed"
	if _, err := s.UpdateWorklog(w); err == nil {
		t.Fatal("an edit marked an entry reviewed without a comment")
	}
	k := "PROJ-1"
	w.Status, w.Comment, w.IssueKey = "draft", "  Billing fix  ", &k
	w, err = s.UpdateWorklog(w)
	must(t, err)
	if w.Comment != "Billing fix" || !w.Locked {
		t.Fatalf("the comment is trimmed and locks the entry: %+v", w)
	}
	must(t, s.SetStatus([]int64{w.ID}, "reviewed"))
	text, err := s.ExportText([]int64{w.ID})
	must(t, err)
	if !strings.Contains(text, "Billing fix") {
		t.Errorf("Tempo gets the comment:\n%s", text)
	}
	must(t, s.SetComment([]int64{w.ID}, "Billing"))
	m, _ := s.Month("2026-03")
	if m.Worklogs[0].Comment != "Billing" {
		t.Errorf("bulk comment: %+v", m.Worklogs[0])
	}
}

func TestScanEveryWorkspaceFolder(t *testing.T) {
	s, _ := newService(t)
	a, b := t.TempDir(), t.TempDir()
	must(t, os.MkdirAll(filepath.Join(a, "billing", ".git"), 0o755))
	must(t, os.MkdirAll(filepath.Join(b, "portal", ".git"), 0o755))
	must(t, os.MkdirAll(filepath.Join(b, "archive", "old", ".git"), 0o755))
	st := s.Settings()
	st.WorkspaceRoots, st.ExcludedFolders = []string{a, b}, []string{"archive"}
	_, err := s.SaveSettings(st)
	must(t, err)
	must(t, s.ScanWorkspace())
	repos, _ := s.Repos("2026-03")
	if len(repos) != 2 {
		t.Fatalf("both folders, the skipped folder in neither: %+v", repos)
	}
	if s.Settings().WorkspaceRoot != a {
		t.Errorf("workspaceRoot mirrors the first folder: %q", s.Settings().WorkspaceRoot)
	}
}
