package app

import (
	"fmt"
	"path/filepath"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
)

// ── Repositories & projects ─────────────────────────────────────────────────

// Repos lists the repositories with your commits in the month, and how many fall on each day.
func (s *Service) Repos(month string) ([]RepoDTO, error) {
	from, to, ok := s.Days.Month(month)
	if !ok {
		return nil, fmt.Errorf("not a month: %q", month)
	}
	start, end, _ := s.span(from, to)
	repos, err := s.Store.Repos()
	if err != nil {
		return nil, err
	}
	byDay, err := s.Store.CommitDays(start, end)
	if err != nil {
		return nil, err
	}
	out := []RepoDTO{}
	for _, r := range repos {
		days := byDay[r.ID]
		if days == nil {
			days = map[string]int{}
		}
		n := 0
		for _, c := range days {
			n += c
		}
		out = append(out, RepoDTO{r.ID, r.Path, r.Name, r.IssueKey, r.ProjectKey, r.Enabled, n, days})
	}
	return out, nil
}

func (s *Service) UpdateRepo(r RepoDTO) error {
	return s.Store.UpdateRepo(core.Repo{
		ID: r.ID, Path: r.Path, Name: r.Name,
		IssueKey: core.NormalizeKey(r.IssueKey), ProjectKey: core.NormalizeKey(r.ProjectKey), Enabled: r.Enabled,
	})
}

// RemoveRepo takes it off the list only; the repository on disk is not touched.
func (s *Service) RemoveRepo(id int64) error { return s.Store.DeleteRepo(id) }

// ScanWorkspace finds git repositories under the workspace folders.
func (s *Service) ScanWorkspace() error {
	st := s.current()
	roots := st.Roots()
	if len(roots) == 0 {
		return core.Err("repos.needFolder")
	}
	// The skipped folders apply to every workspace folder.
	for _, r := range roots {
		root := filepath.Clean(git.ExpandHome(r))
		for _, p := range git.FindRepos(root, st.ExcludedFolders, 4) {
			if err := s.Store.AddRepo(p, filepath.Base(p)); err != nil {
				return err
			}
		}
	}
	return s.removeHiddenRepos()
}

// removeHiddenRepos drops listed repositories that sit inside a hidden (excluded) folder.
func (s *Service) removeHiddenRepos() error {
	repos, err := s.Store.Repos()
	if err != nil {
		return err
	}
	for _, r := range repos {
		if git.IsExcluded(r.Path, s.current().ExcludedFolders) {
			if err := s.Store.DeleteRepo(r.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Projects() ([]ProjectDTO, error) {
	list, err := s.Store.Projects()
	out := []ProjectDTO{}
	for _, p := range list {
		out = append(out, ProjectDTO{p.Key, p.DefaultIssue})
	}
	return out, err
}

func (s *Service) SaveProject(p ProjectDTO) error {
	key := core.NormalizeKeyString(p.Key)
	if key == nil {
		return core.Err("issue.missing")
	}
	return s.Store.UpsertProject(core.ProjectMapping{Key: *key, DefaultIssue: core.NormalizeKey(p.DefaultIssue)})
}

func (s *Service) RemoveProject(key string) error { return s.Store.DeleteProject(key) }
