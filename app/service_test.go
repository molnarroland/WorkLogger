package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
	"github.com/molnarroland/WorkLogger/internal/store"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

type keyring struct {
	*ts.InMemorySecrets
	locked bool
}

func (k *keyring) Unlock() bool { k.locked = false; return true }
func (k *keyring) Locked() bool { return k.locked }

// gitRunner answers `git log` with one commit per repository on 2026-03-03 and the global email.
type gitRunner struct{}

func (gitRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	if args[0] == "config" {
		return "me@example.com\n", nil
	}
	return "\x1e" + strings.Join([]string{
		"h-" + filepath.Base(args[1]), "2026-03-03T10:00:00+01:00", "Me", "me@example.com",
		"PROJ-1 Fix rounding", "", "refs/heads/main",
	}, "\x1f") + "\x1d\n10\t2\tsrc/a.go\n", nil
}

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) emit(name string, _ any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, name)
}

func newService(t *testing.T) (*Service, *recorder) {
	t.Helper()
	db, err := store.Open(":memory:", ts.Days)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	rec := &recorder{}
	s := New(Deps{
		Store: db, Days: ts.Days, Clock: ts.FixedClock{T: ts.At("2026-03-10", "12:00")},
		Secrets: &keyring{ts.NewSecrets(nil), true}, HTTP: &ts.FakeHTTP{}, Git: git.Git{Run: gitRunner{}},
		Shell: Shell{Emit: rec.emit},
	})
	return s, rec
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCollectAndEdit(t *testing.T) {
	s, rec := newService(t)
	root := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "billing", ".git"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "archive", "old", ".git"), 0o755))
	st := s.Settings()
	st.WorkspaceRoots, st.ExcludedFolders = []string{root}, []string{"archive"}
	_, err := s.SaveSettings(st)
	must(t, err)
	must(t, s.ScanWorkspace())
	repos, _ := s.Repos("2026-03")
	if len(repos) != 1 || repos[0].Name != "billing" {
		t.Fatalf("scan: %+v", repos)
	}

	must(t, s.Collect("2026-03-01", "2026-03-31", false))
	if len(rec.events) < 2 || rec.events[len(rec.events)-1] != EventDone {
		t.Errorf("events: %v", rec.events)
	}
	if s.Settings().AuthorEmails[0] != "me@example.com" {
		t.Error("the global git email is taken when none is set")
	}
	m, err := s.Month("2026-03")
	must(t, err)
	if len(m.Days) != 31 || m.Days[2].Expected != 8*3600 || m.Days[6].Expected != 0 || len(m.Worklogs) != 1 {
		t.Fatalf("month: %d days, %+v, %d logs", len(m.Days), m.Days[2], len(m.Worklogs))
	}
	w := m.Worklogs[0]
	if w.Day != "2026-03-03" || *w.IssueKey != "PROJ-1" || w.Duration != 8*3600 || w.Locked {
		t.Errorf("draft: %+v", w)
	}
	if repos, _ := s.Repos("2026-03"); repos[0].Commits != 1 || len(repos[0].Days) != 1 || repos[0].Days["2026-03-03"] != 1 {
		t.Errorf("commit counts and days: %+v", repos[0])
	}
	if c, err := s.SourceCounts("2026-03-03", "2026-03-03"); err != nil || c.Repositories != 1 {
		t.Errorf("counts on the commit's day: %+v %v", c, err)
	}
	if c, _ := s.SourceCounts("2026-03-04", "2026-03-31"); c.Repositories != 0 {
		t.Errorf("counts without commits: %+v", c)
	}

	w.Duration = 4 * 3600
	got, err := s.UpdateWorklog(w)
	must(t, err)
	if !got.Locked || got.Duration != 4*3600 {
		t.Errorf("an edit locks: %+v", got)
	}
	added, err := s.AddWorklog("2026-03-03")
	must(t, err)
	if added.Start != rfc(ts.At("2026-03-03", "13:00")) || added.Source != "manual" {
		t.Errorf("manual entry goes after the last one: %+v", added)
	}
	must(t, s.MoveWorklogs([]int64{added.ID}, "2026-03-04"))
	must(t, s.SetIssue([]int64{added.ID}, " int-12 "))
	must(t, s.SetEntered([]int64{w.ID}, true))
	must(t, s.Rebuild("2026-03-01", "2026-03-31")) // both are locked now: nothing is rebuilt over them
	m, _ = s.Month("2026-03")
	if len(m.Worklogs) != 2 || m.Worklogs[0].Status != "exported" || m.Worklogs[1].Day != "2026-03-04" ||
		*m.Worklogs[1].IssueKey != "INT-12" || m.Worklogs[1].Start != rfc(ts.At("2026-03-04", "09:00")) {
		t.Errorf("after edits and rebuild: %+v", m.Worklogs)
	}
	text, _ := s.ExportText([]int64{w.ID, added.ID})
	if !strings.Contains(text, "| 1 | PROJ-1 | 09:00 | 4h |") {
		t.Errorf("export:\n%s", text)
	}
}

func TestSettingsKeepMacOnlyKeys(t *testing.T) {
	s, _ := newService(t)
	must(t, s.update(func(st *core.Settings) { st.MacCalendarIDs = []string{"mac-cal"} }))
	in := s.Settings()
	in.MacCalendarIDs, in.KnownModules, in.FixedDailyHours = nil, nil, 6
	out, err := s.SaveSettings(in)
	must(t, err)
	if out.FixedDailyHours != 6 || len(out.MacCalendarIDs) != 1 || len(out.KnownModules) != len(core.AllModules) {
		t.Errorf("%+v", out)
	}
}

func TestChats(t *testing.T) {
	s, _ := newService(t)

	n, err := s.ImportChats(`{"source":"google-chat","conversations":[{"id":"dm/B","type":"dm","name":"Bea","messages":[
      {"time":"2026-03-03T09:00:00Z","sender":"me@x","me":true,"text":"Ön: hi"},
      {"time":"2026-03-03T09:20:00Z","sender":"Bea","me":false,"text":"yo"}]}]}`)
	must(t, err)
	grouped, _ := s.Chats("2026-03-01", "2026-03-31")
	if len(grouped.Days) != 1 || grouped.Days[0].Day != "2026-03-03" || grouped.Days[0].Seconds != 120 || grouped.Active != 120 {
		t.Errorf("by day: %+v", grouped)
	}
	chats, _ := s.chatList("2026-03-01", "2026-03-31")
	if n != 2 || len(chats) != 1 || chats[0].Type != "dm" || chats[0].ActiveSeconds != 120 || chats[0].IssueKey != nil {
		t.Fatalf("%d %+v", n, chats)
	}
	if c, _ := s.SourceCounts("2026-03-01", "2026-03-31"); c.Chats != 1 {
		t.Errorf("chat count: %+v", c)
	}
	msgs, _ := s.ChatMessages(chats[0].ID)
	if len(msgs) != 2 || msgs[0].Text != "hi" {
		t.Errorf("%+v", msgs)
	}
	must(t, s.LogChat(chats[0].ID, "proj-9"))
	chats, _ = s.chatList("2026-03-01", "2026-03-31")
	m, _ := s.Month("2026-03")
	if *chats[0].IssueKey != "PROJ-9" || len(m.Worklogs) != 1 || m.Worklogs[0].Duration != 120 || m.Worklogs[0].Source != "chat" {
		t.Errorf("logChat: %+v %+v", chats[0], m.Worklogs)
	}
}
