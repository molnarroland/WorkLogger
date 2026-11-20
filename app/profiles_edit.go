package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Profiles by intent (RELEASE_REVIEW B3, A10): adding, removing and relinking profiles happens here in one step,
// tokens included, so the frontend never handles a token or a keyring name.

// ProfileEdit is the saved settings after AddProfile, with the new profile's id. NeedsJira: a Tempo profile
// was asked for, but every Jira profile already has one (nothing was added).
type ProfileEdit struct {
	Settings  core.Settings `json:"settings"`
	ID        string        `json:"id"`
	NeedsJira bool          `json:"needsJira"`
}

// AddProfile adds an empty profile of a kind: jira (a Jira site), tempo (for the first Jira profile without
// one), googleChat (with your first git email), hrmaster, or confluence (linked to the default Jira profile).
func (s *Service) AddProfile(kind string) (ProfileEdit, error) {
	id := NewID()
	if kind == "jira" {
		st, err := s.edit(func(st *core.Settings) error {
			st.JiraAccounts = append(st.JiraAccounts, core.JiraAccount{
				ID: id, Provider: core.ProviderJira, TempoBaseURL: core.DefaultTempoURL, ProjectKeys: []string{},
			})
			return nil
		})
		return ProfileEdit{Settings: st, ID: id}, err
	}
	p := core.Profile{ID: id, Kind: core.ProfileKind(kind)}
	switch p.Kind {
	case core.ProfileGoogleChat:
		if emails := s.current().AuthorEmails; len(emails) > 0 {
			p.Email = emails[0]
		}
	case core.ProfileHRmaster:
	case core.ProfileEmail:
		if emails := s.current().AuthorEmails; len(emails) > 0 {
			p.Email = emails[0]
		}
	case core.ProfileConfluence:
		if a := s.current().DefaultJiraAccount(); a != nil {
			p.JiraAccountID = a.ID
		}
	case core.ProfileTempo:
		free := unlinked(s.current())
		if len(free) == 0 {
			return ProfileEdit{Settings: s.current(), NeedsJira: true}, nil
		}
		p.URL, p.JiraAccountID = core.DefaultTempoURL, free[0]
	default:
		return ProfileEdit{Settings: s.current()}, fmt.Errorf("unknown profile kind %q", kind)
	}
	st, err := s.edit(func(st *core.Settings) error {
		st.Profiles = append(st.Profiles, p)
		return nil
	})
	return ProfileEdit{Settings: st, ID: id}, err
}

// unlinked is the Jira profiles no Tempo profile links yet.
func unlinked(st core.Settings) []string {
	var out []string
	for _, a := range st.JiraAccounts {
		if !slices.ContainsFunc(st.Profiles, func(p core.Profile) bool { return p.Kind == core.ProfileTempo && p.JiraAccountID == a.ID }) {
			out = append(out, a.ID)
		}
	}
	return out
}

// RemoveProfile removes a profile or a Jira profile by id. A Jira profile takes its tokens and the Tempo profile
// linking it along (a Confluence profile linking it is unlinked); a Tempo or Confluence profile its own token. Needs the keyring unlocked when tokens go.
func (s *Service) RemoveProfile(id string) (core.Settings, error) {
	st := s.current()
	if slices.ContainsFunc(st.JiraAccounts, func(a core.JiraAccount) bool { return a.ID == id }) {
		if err := s.clearTokens(id, "jira", "tempo"); err != nil {
			return st, err
		}
		return s.edit(func(st *core.Settings) error {
			st.JiraAccounts = slices.DeleteFunc(st.JiraAccounts, func(a core.JiraAccount) bool { return a.ID == id })
			st.Profiles = slices.DeleteFunc(st.Profiles, func(p core.Profile) bool { return p.Kind == core.ProfileTempo && p.JiraAccountID == id })
			st.SyncProfiles() // unlinks the Confluence profiles that used it
			return nil
		})
	}
	i := slices.IndexFunc(st.Profiles, func(p core.Profile) bool { return p.ID == id })
	if i < 0 {
		return st, fmt.Errorf("no profile %q", id)
	}
	if p := st.Profiles[i]; p.Kind == core.ProfileTempo && p.JiraAccountID != "" {
		if err := s.clearTokens(p.JiraAccountID, "tempo"); err != nil {
			return st, err
		}
	}
	if k := st.Profiles[i].Kind; k == core.ProfileConfluence || k == core.ProfileEmail {
		if err := s.clearTokens(id, string(k)); err != nil {
			return st, err
		}
	}
	return s.edit(func(st *core.Settings) error {
		st.Profiles = slices.DeleteFunc(st.Profiles, func(p core.Profile) bool { return p.ID == id })
		return nil
	})
}

// clearTokens deletes a Jira profile's tokens. Locked, it refuses: the profile would go and its tokens stay behind.
func (s *Service) clearTokens(accountID string, kinds ...string) error {
	for _, k := range kinds {
		if err := s.SetToken(accountID, k, ""); err != nil {
			return err
		}
	}
	return nil
}

// UpdateProfile saves a profile's fields (its kind stays). A Tempo profile moved to another Jira profile takes
// its Tempo token along.
func (s *Service) UpdateProfile(p core.Profile) (core.Settings, error) {
	st := s.current()
	i := slices.IndexFunc(st.Profiles, func(x core.Profile) bool { return x.ID == p.ID })
	if i < 0 {
		return st, fmt.Errorf("no profile %q", p.ID)
	}
	old := st.Profiles[i]
	p.Kind = old.Kind
	p.Name, p.Email, p.URL = strings.TrimSpace(p.Name), strings.TrimSpace(p.Email), strings.TrimSpace(p.URL)
	if p.Kind == core.ProfileTempo && p.JiraAccountID != old.JiraAccountID {
		if p.JiraAccountID != "" && !slices.Contains(unlinked(st), p.JiraAccountID) {
			return st, core.Err("profile.tempoTaken")
		}
		if err := s.moveTempoToken(old.JiraAccountID, p.JiraAccountID); err != nil {
			return st, err
		}
	}
	return s.edit(func(st *core.Settings) error {
		st.Profiles[i] = p
		return nil
	})
}

func (s *Service) moveTempoToken(from, to string) error {
	if from == "" {
		return nil
	}
	if s.Secrets.Locked() {
		return errLocked
	}
	key, err := s.tokenKey(from, "tempo")
	if err != nil {
		return nil // the old Jira profile is gone, and its token with it
	}
	token, _ := s.Secrets.Get(key)
	if err := s.Secrets.Set(key, ""); err != nil {
		return err
	}
	if token != "" && to != "" {
		return s.SetToken(to, "tempo", token)
	}
	return nil
}

// UpdateJiraAccount saves a Jira profile's fields.
func (s *Service) UpdateJiraAccount(a core.JiraAccount) (core.Settings, error) {
	a.Name, a.BaseURL, a.Email = strings.TrimSpace(a.Name), strings.TrimRight(strings.TrimSpace(a.BaseURL), "/"), strings.TrimSpace(a.Email)
	return s.edit(func(st *core.Settings) error {
		i := slices.IndexFunc(st.JiraAccounts, func(x core.JiraAccount) bool { return x.ID == a.ID })
		if i < 0 {
			return fmt.Errorf("no Jira profile %q", a.ID)
		}
		if a.ProjectKeys == nil {
			a.ProjectKeys = []string{}
		}
		st.JiraAccounts[i] = a
		return nil
	})
}

// SetDefaultJiraAccount picks the Jira profile issues go to when no project key matches.
func (s *Service) SetDefaultJiraAccount(id string) (core.Settings, error) {
	return s.edit(func(st *core.Settings) error {
		if !slices.ContainsFunc(st.JiraAccounts, func(a core.JiraAccount) bool { return a.ID == id }) {
			return fmt.Errorf("no Jira profile %q", id)
		}
		st.DefaultJiraAccountID = core.Ptr(id)
		return nil
	})
}
