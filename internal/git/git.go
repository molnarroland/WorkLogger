// Package git reads repositories and history through the git command line.
package git

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

type Git struct{ Run core.CommandRunner }

// Log reads the user's non-merge commits on all branches in [since, until). `%S` gives the ref the commit was
// reached from, which is used to find Jira keys in branch names.
// git's --since/--until go by committer date, but commits are filed by author date: a commit rebased or amended
// after `until` would be dropped from the day it was written. So only --since is passed (the committer date is
// never before the author date) and the author date is filtered below.
func (g Git) Log(ctx context.Context, repoPath string, since, until time.Time, authors []string) ([]core.GitCommit, error) {
	args := []string{
		"-C", repoPath, "log", "--all", "--no-merges", "--source", "--numstat",
		"--since=" + since.UTC().Format(time.RFC3339),
		"--pretty=format:%x1e%H%x1f%aI%x1f%an%x1f%ae%x1f%s%x1f%b%x1f%S%x1d",
	}
	for _, a := range authors {
		args = append(args, "--author="+a)
	}
	out, err := g.Run.Run(ctx, "git", args...)
	if err != nil {
		return nil, err
	}
	var in []core.GitCommit
	for _, c := range Parse(out) {
		if !c.Date.Before(since) && c.Date.Before(until) {
			in = append(in, c)
		}
	}
	return in, nil
}

// Parse reads `git log` output in the format Log asks for; malformed records and duplicates are dropped.
func Parse(output string) []core.GitCommit {
	var commits []core.GitCommit
	seen := map[string]bool{}
	for _, record := range strings.Split(output, "\x1e") {
		sep := strings.IndexByte(record, '\x1d')
		if sep < 0 {
			continue
		}
		fields := strings.Split(record[:sep], "\x1f")
		if len(fields) < 7 || seen[fields[0]] {
			continue
		}
		date, err := time.Parse(time.RFC3339, fields[1])
		if err != nil {
			continue
		}
		seen[fields[0]] = true
		var files []core.FileStat
		for _, line := range strings.FieldsFunc(record[sep+1:], func(r rune) bool { return r == '\n' || r == '\r' }) {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) != 3 {
				continue
			}
			// Binary files show "-" instead of line counts.
			added, _ := strconv.Atoi(parts[0])
			deleted, _ := strconv.Atoi(parts[1])
			files = append(files, core.FileStat{Added: added, Deleted: deleted, Path: parts[2]})
		}
		commits = append(commits, core.GitCommit{
			Hash: fields[0], Date: date, Author: fields[2], Email: fields[3], Subject: fields[4],
			Body: strings.TrimSpace(fields[5]), Ref: fields[6], Files: files,
		})
	}
	return commits
}

func (g Git) GlobalUserEmail(ctx context.Context) string {
	out, err := g.Run.Run(ctx, "git", "config", "--global", "user.email")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

type RepoCommits struct {
	RepoID  int64
	Commits []core.GitCommit
}

// historyWorkers is how many `git log` run at once; each is mostly disk and process start-up.
const historyWorkers = 8

// History reads your commits per repository, several repositories at a time. A repository that can't be read is
// reported by name in errors instead of stopping the others. Results keep the order of repos.
func (g Git) History(ctx context.Context, repos []core.Repo, since, until time.Time, authors []string) ([]RepoCommits, []string) {
	type result struct {
		commits []core.GitCommit
		err     error
	}
	results := make([]result, len(repos))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(historyWorkers, len(repos)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				commits, err := g.Log(ctx, repos[i].Path, since, until, authors)
				results[i] = result{commits, err}
			}
		}()
	}
	for i := range repos {
		if ctx.Err() != nil {
			break
		}
		next <- i
	}
	close(next)
	wg.Wait()
	var ok []RepoCommits
	var failed []string
	for i, r := range results {
		switch {
		case r.err != nil:
			failed = append(failed, repos[i].Name+": "+r.err.Error())
		case r.commits != nil || ctx.Err() == nil:
			ok = append(ok, RepoCommits{repos[i].ID, r.commits})
		}
	}
	return ok, failed
}

var skippedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "build": true, "dist": true, ".build": true,
	"Pods": true, "DerivedData": true, "storage": true, "target": true,
}

// FindRepos finds git repositories (and worktrees) below a workspace folder, up to maxDepth levels down.
func FindRepos(root string, excluded []string, maxDepth int) []string {
	var result []string
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if IsExcluded(dir, excluded) {
			return
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			result = append(result, dir) // keep descending: nested repos and submodules are repos too (node_modules/vendor are skipped below)
		}
		if depth >= maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") || skippedDirs[name] || strings.HasSuffix(name, ".app") {
				continue
			}
			path := filepath.Join(dir, name)
			// Symlinks, and on Windows junctions and other reparse points, aren't IsDir: follow them.
			// maxDepth stops link loops.
			if !e.IsDir() {
				if e.Type()&(os.ModeSymlink|os.ModeIrregular) == 0 {
					continue
				}
				if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
					continue
				}
			}
			walk(path, depth+1)
		}
	}
	walk(root, 0)
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(filepath.Base(result[i])) < strings.ToLower(filepath.Base(result[j]))
	})
	return result
}

// ExpandHome turns a leading "~" into the home folder.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

// IsExcluded: an entry with a "/" is a path (the folder and everything below it); otherwise any folder with that name.
func IsExcluded(path string, excluded []string) bool {
	components := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for _, entry := range excluded {
		if strings.ContainsAny(entry, `/\`) {
			dir := filepath.Clean(ExpandHome(entry))
			if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) || strings.HasPrefix(path, dir+"/") {
				return true
			}
			continue
		}
		for _, c := range components {
			if c == entry {
				return true
			}
		}
	}
	return false
}

// validHash: a full or abbreviated commit hash (never an option or a revision expression).
func validHash(h string) bool {
	if len(h) < 4 || len(h) > 64 {
		return false
	}
	for _, r := range h {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// Files is what a commit changed: each file with its added and deleted lines ("-" for binary files reads as 0).
// A rename shows as git writes it ("old => new").
func (g Git) Files(ctx context.Context, repoPath, hash string) ([]core.FileStat, error) {
	if !validHash(hash) {
		return nil, core.Err("commit.badHash")
	}
	out, err := g.Run.Run(ctx, "git", "-C", repoPath, "show", "--no-color", "--numstat", "--format=", hash)
	if err != nil {
		return nil, err
	}
	files := []core.FileStat{}
	for _, line := range strings.FieldsFunc(out, func(r rune) bool { return r == '\n' || r == '\r' }) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		files = append(files, core.FileStat{Added: added, Deleted: deleted, Path: parts[2]})
	}
	return files, nil
}

// MaxDiff is the most of a diff Diff returns, in bytes.
const MaxDiff = 256 << 10

// Diff is a commit's patch, of one file when path isn't empty: at most MaxDiff bytes, and whether it was cut.
func (g Git) Diff(ctx context.Context, repoPath, hash, path string) (string, bool, error) {
	if !validHash(hash) {
		return "", false, core.Err("commit.badHash")
	}
	args := []string{"-C", repoPath, "show", "--no-color", "--no-ext-diff", "--format=", "--patch", "--find-renames", hash}
	if path != "" {
		// A rename's numstat path ("dir/{a => b}.go" or "a => b"): both names, so git shows the rename.
		args = append(append(args, "--"), renamePaths(path)...)
	}
	out, err := g.Run.Run(ctx, "git", args...)
	if err != nil {
		return "", false, err
	}
	if len(out) > MaxDiff {
		cut := strings.LastIndexByte(out[:MaxDiff], '\n')
		if cut < 0 {
			cut = MaxDiff
		}
		return out[:cut], true, nil
	}
	return out, false, nil
}

// renamePaths is the old and new path of a numstat rename ("a => b", "dir/{a => b}/f"), else the path itself.
func renamePaths(p string) []string {
	clean := func(s string) string { return strings.ReplaceAll(s, "//", "/") }
	if i, j := strings.Index(p, "{"), strings.Index(p, "}"); i >= 0 && j > i {
		if arrow := strings.Index(p[i:j], " => "); arrow >= 0 {
			return []string{clean(p[:i] + p[i+1:i+arrow] + p[j+1:]), clean(p[:i] + p[i+arrow+4:j] + p[j+1:])}
		}
	}
	if arrow := strings.Index(p, " => "); arrow >= 0 {
		return []string{p[:arrow], p[arrow+4:]}
	}
	return []string{p}
}
