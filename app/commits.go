package app

import (
	"context"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// ── Commits behind an entry ─────────────────────────────────────────────────

// CommitDTO is a stored commit, with its repository's name.
type CommitDTO struct {
	RepoID     int64  `json:"repoId"`
	Repo       string `json:"repo"`
	Hash       string `json:"hash"`
	At         string `json:"at"`
	Author     string `json:"author"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	Ref        string `json:"ref"`
	Files      int    `json:"files"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

// CommitFileDTO is a file a commit changed; a rename's path is git's "old => new".
type CommitFileDTO struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
}

// CommitDiffDTO is a commit's patch (of one file, or all of it); Truncated when it was too long to show whole.
type CommitDiffDTO struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

// gitTimeout bounds reading one commit's files or patch.
const gitTimeout = 20 * time.Second

// WorklogCommits is the commits an entry was made from (none for an entry that isn't from git), oldest first.
func (s *Service) WorklogCommits(id int64) ([]CommitDTO, error) {
	out := []CommitDTO{}
	list, err := s.Store.WorklogsByID([]int64{id})
	if err != nil || len(list) == 0 {
		return out, err
	}
	w := list[0]
	if w.Source != core.SourceGit {
		return out, nil
	}
	commits, err := s.Store.CommitsByHash(w.Refs())
	if err != nil {
		return out, err
	}
	repos, err := s.Store.Repos()
	if err != nil {
		return out, err
	}
	names := map[int64]string{}
	for _, r := range repos {
		names[r.ID] = r.Name
	}
	for _, c := range commits {
		if w.RepoID != nil && c.RepoID != *w.RepoID {
			continue // the same hash in a fork or a copy of the repository
		}
		out = append(out, CommitDTO{
			RepoID: c.RepoID, Repo: names[c.RepoID], Hash: c.Hash, At: rfc(c.Date), Author: c.Author, Subject: c.Subject,
			Body: c.Body, Ref: c.Ref, Files: c.FilesChanged, Insertions: c.Insertions, Deletions: c.Deletions,
		})
	}
	return out, nil
}

// CommitFiles is what a commit changed, read from its repository.
func (s *Service) CommitFiles(repoID int64, hash string) ([]CommitFileDTO, error) {
	r, err := s.repo(repoID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	files, err := s.Git.Files(ctx, r.Path, hash)
	if err != nil {
		s.logFailure("git", "Reading the files of "+short(hash)+" failed", err)
		return nil, commitError(r, err)
	}
	out := make([]CommitFileDTO, len(files))
	for i, f := range files {
		out[i] = CommitFileDTO{Path: f.Path, Added: f.Added, Deleted: f.Deleted}
	}
	return out, nil
}

// CommitDiff is a commit's patch, of one file (as CommitFiles names it) or, with an empty path, all of it.
func (s *Service) CommitDiff(repoID int64, hash, path string) (CommitDiffDTO, error) {
	r, err := s.repo(repoID)
	if err != nil {
		return CommitDiffDTO{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	text, cut, err := s.Git.Diff(ctx, r.Path, hash, path)
	if err != nil {
		s.logFailure("git", "Reading the changes of "+short(hash)+" failed", err)
		return CommitDiffDTO{}, commitError(r, err)
	}
	return CommitDiffDTO{Text: text, Truncated: cut}, nil
}

func (s *Service) repo(id int64) (core.Repo, error) {
	repos, err := s.Store.Repos()
	if err != nil {
		return core.Repo{}, err
	}
	for _, r := range repos {
		if r.ID == id {
			return r, nil
		}
	}
	return core.Repo{}, core.Err("commit.noRepo")
}

// commitError keeps a user error (a bad hash) and says the commit couldn't be read for anything else.
func commitError(r core.Repo, err error) error {
	if _, ok := err.(*core.UserError); ok {
		return err
	}
	return core.Err("commit.notFound", "repo", r.Name)
}

func short(hash string) string { return hash[:min(len(hash), 8)] }
