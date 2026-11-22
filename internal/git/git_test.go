package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// record is one entry in the `--pretty` format Log asks git for, followed by `--numstat` lines.
func record(hash, date, ref string, numstat ...string) string {
	if date == "" {
		date = "2026-03-03T10:00:00+01:00"
	}
	if ref == "" {
		ref = "refs/heads/main"
	}
	return "\x1e" + strings.Join([]string{hash, date, "Jane", "me@example.com", "PROJ-1 Fix", "Body text\n", ref}, "\x1f") +
		"\x1d\n" + strings.Join(numstat, "\n") + "\n"
}

func TestParse(t *testing.T) {
	c := Parse(record("abc123", "", "", "5\t2\tsrc/a.swift", "-\t-\timg.png"))[0]
	want, _ := time.Parse(time.RFC3339, "2026-03-03T09:00:00Z")
	if c.Hash != "abc123" || !c.Date.Equal(want) || c.Author != "Jane" || c.Email != "me@example.com" ||
		c.Subject != "PROJ-1 Fix" || c.Body != "Body text" || c.Ref != "refs/heads/main" ||
		!reflect.DeepEqual(c.Files, []core.FileStat{{Added: 5, Deleted: 2, Path: "src/a.swift"}, {Path: "img.png"}}) {
		t.Errorf("parsesHeaderAndFileStats: %+v", c)
	}
	if c := Parse(record("a", "", "", "1\t1\tdir/with\ttab.txt"))[0]; c.Files[0].Path != "dir/with\ttab.txt" {
		t.Error("pathsWithTabsStayWhole")
	}
	hashes := func(cs []core.GitCommit) (out []string) {
		for _, c := range cs {
			out = append(out, c.Hash)
		}
		return
	}
	if got := hashes(Parse(record("a", "", "refs/heads/main") + record("a", "", "refs/heads/feature") + record("b", "", ""))); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("duplicatesFromSeveralBranchesAreDropped: %v", got)
	}
	if got := hashes(Parse(record("bad-date", "yesterday", "") + "\x1eno header end" + record("ok", "", ""))); !reflect.DeepEqual(got, []string{"ok"}) {
		t.Errorf("malformedRecordsAreSkipped: %v", got)
	}
	if len(Parse("")) != 0 {
		t.Error("emptyOutput")
	}
}

func TestIgnoredPaths(t *testing.T) {
	c := core.GitCommit{Hash: "a", Files: []core.FileStat{
		{Added: 5, Deleted: 2, Path: "src/a.swift"},
		{Added: 1000, Deleted: 500, Path: "package-lock.json"},
		{Added: 3, Path: "web/dist/app.js"},
	}}
	s := core.NewStoredCommit(c, 7, []string{"package-lock.json", "/dist/", ""})
	if s.RepoID != 7 || s.FilesChanged != 1 || s.Insertions != 5 || s.Deletions != 2 {
		t.Errorf("%+v", s)
	}
}

func TestExcludedFolders(t *testing.T) {
	ex := []string{"archive", "/Users/me/Work/old"}
	for path, want := range map[string]bool{
		"/Users/me/Work/archive/repo": true, "/Users/me/Work/old": true, "/Users/me/Work/old/repo": true,
		"/Users/me/Work/older/repo": false, "/Users/me/Work/archived": false,
	} {
		if IsExcluded(path, ex) != want {
			t.Errorf("%s: want %v", path, want)
		}
	}
}

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	if strings.HasSuffix(args[1], "broken") {
		return "", errors.New("not a git repository")
	}
	return record(args[1], "", ""), nil
}

func TestHistory(t *testing.T) {
	repos := []core.Repo{{ID: 1, Path: "/w/billing", Name: "billing"}, {ID: 2, Path: "/w/broken", Name: "broken"}}
	ok, failed := Git{fakeRunner{}}.History(context.Background(), repos, time.Unix(0, 0), time.Now(), []string{"me@x.hu"})
	if len(ok) != 1 || ok[0].RepoID != 1 || ok[0].Commits[0].Hash != "/w/billing" ||
		!reflect.DeepEqual(failed, []string{"broken: not a git repository"}) {
		t.Errorf("readsEachRepositoryAndReportsTheBrokenOnes: %+v %v", ok, failed)
	}
}

// Linked folders (symlinks; junctions on Windows) are followed when looking for repositories.
func TestFindReposFollowsLinkedFolders(t *testing.T) {
	root, elsewhere := t.TempDir(), t.TempDir()
	repo := filepath.Join(elsewhere, "linked", "app")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plain", "lib", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(elsewhere, "linked"), filepath.Join(root, "linked")); err != nil {
		t.Skip("symlinks not available:", err)
	}
	got := FindRepos(root, nil, 4)
	want := []string{filepath.Join(root, "linked", "app"), filepath.Join(root, "plain", "lib")}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A repository inside another repository is found too, but not one in node_modules.
func TestFindReposFindsNestedRepos(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"outer/.git", "outer/packages/inner/.git", "outer/node_modules/dep/.git"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := FindRepos(root, nil, 4)
	want := []string{filepath.Join(root, "outer", "packages", "inner"), filepath.Join(root, "outer")}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A commit rebased later (committer date after the range) still belongs to the day it was authored.
func TestLogUsesAuthorDateNotCommitterDate(t *testing.T) {
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skip("git not available:", err, string(out))
		}
	}
	run(nil, "init", "-q")
	run([]string{"GIT_AUTHOR_DATE=2026-11-03T10:00:00Z", "GIT_COMMITTER_DATE=2026-11-06T10:00:00Z"},
		"-c", "user.name=A", "-c", "user.email=a@b.c", "commit", "-q", "--allow-empty", "-m", "rebased")
	since := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	got, err := Git{Run: execRunner{}}.Log(context.Background(), dir, since, since.AddDate(0, 0, 1), []string{"a@b.c"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v; want the rebased commit", got, err)
	}
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}

func TestFilesAndDiff(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=A", "-c", "user.email=a@b.c"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Skip("git not available:", err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "src", "a.go"), []byte("one\ntwo\n"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644))
	run("add", ".")
	run("commit", "-q", "-m", "first")
	must(os.WriteFile(filepath.Join(dir, "src", "a.go"), []byte("one\nthree\n"), 0o644))
	run("mv", "b.txt", "c.txt")
	run("commit", "-q", "-am", "second")
	hash := run("rev-parse", "HEAD")

	g := Git{Run: execRunner{}}
	files, err := g.Files(context.Background(), dir, hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[1].Path != "src/a.go" || files[1].Added != 1 || files[1].Deleted != 1 {
		t.Errorf("files: %+v", files)
	}
	diff, cut, err := g.Diff(context.Background(), dir, hash, "src/a.go")
	if err != nil || cut || !strings.Contains(diff, "-two\n+three") || strings.Contains(diff, "c.txt") {
		t.Errorf("one file's diff: %q %v %v", diff, cut, err)
	}
	if diff, _, err := g.Diff(context.Background(), dir, hash, files[0].Path); err != nil || !strings.Contains(diff, "rename to c.txt") {
		t.Errorf("a renamed file's diff (%s): %q %v", files[0].Path, diff, err)
	}
	if _, err := g.Files(context.Background(), dir, "--output=/tmp/x"); err == nil {
		t.Error("an option is refused as a hash")
	}
}

func TestRenamePaths(t *testing.T) {
	for in, want := range map[string][]string{
		"a.go":                  {"a.go"},
		"b.txt => c.txt":        {"b.txt", "c.txt"},
		"src/{old => new}/f.go": {"src/old/f.go", "src/new/f.go"},
		"src/{ => sub}/f.go":    {"src/f.go", "src/sub/f.go"},
	} {
		if got := renamePaths(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
