// Package testsupport holds the fakes and fixture builders shared by tests.
package testsupport

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Budapest is the pinned zone every test computes day keys in.
var Budapest = func() *time.Location {
	l, err := time.LoadLocation("Europe/Budapest")
	if err != nil {
		panic(err)
	}
	return l
}()

var Days = core.Days{Loc: Budapest}

func RandomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return hex.EncodeToString(b)
}

// At is a time on a day, e.g. At("2026-03-03", "09:30").
func At(day, hm string) time.Time {
	t, ok := Days.AtHM(day, hm)
	if !ok {
		panic("bad fixture time " + day + " " + hm)
	}
	return t
}

func Interval(day, from, to string) core.Interval {
	return core.Interval{Start: At(day, from), End: At(day, to)}
}

type CommitOpt func(*core.StoredCommit)

func Subject(s string) CommitOpt { return func(c *core.StoredCommit) { c.Subject = s } }
func Ref(s string) CommitOpt     { return func(c *core.StoredCommit) { c.Ref = s } }
func Hash(s string) CommitOpt    { return func(c *core.StoredCommit) { c.Hash = s } }
func RepoID(id int64) CommitOpt  { return func(c *core.StoredCommit) { c.RepoID = id } }
func Size(files, insertions int) CommitOpt {
	return func(c *core.StoredCommit) { c.FilesChanged, c.Insertions = files, insertions }
}

func Commit(day, hm string, opts ...CommitOpt) core.StoredCommit {
	c := core.StoredCommit{
		RepoID: 1, Hash: RandomID(), Date: At(day, hm), Author: "Me", Email: "me@example.com",
		Subject: "Work", FilesChanged: 1, Insertions: 10,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

func Meeting(day, from, to, title string) core.Meeting {
	return core.Meeting{ID: RandomID(), CalendarID: "cal", Title: title, Start: At(day, from), End: At(day, to)}
}

func Session(day, from, to string, kind core.SessionKind) core.WorkSession {
	return core.WorkSession{Day: day, Start: At(day, from), End: At(day, to), Kind: kind}
}

func Worklog(day, hm string, duration int, source core.Source, ref string, locked bool) core.Worklog {
	return core.Worklog{
		Day: day, Start: At(day, hm), Duration: duration, IssueKey: core.Ptr("PROJ-1"),
		Description: "Entry", Source: source, SourceRef: ref, Estimated: duration, Status: core.StatusDraft, Locked: locked,
	}
}

func Builder(s core.Settings, repos []core.Repo, projects []core.ProjectMapping) core.Builder {
	b := core.Builder{Settings: s, Repos: map[int64]core.Repo{}, Projects: map[string]core.ProjectMapping{}, Days: Days}
	for _, r := range repos {
		b.Repos[r.ID] = r
	}
	for _, p := range projects {
		b.Projects[p.Key] = p
	}
	return b
}

func Repo(name string, issueKey, projectKey *string) core.Repo {
	return core.Repo{ID: 1, Path: "/tmp/" + name, Name: name, IssueKey: issueKey, ProjectKey: projectKey, Enabled: true}
}
