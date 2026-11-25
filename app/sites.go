package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/web"
)

// ── Signed-in sites (profiles) ──────────────────────────────────────────────

// siteFor is the profile's site window: made on first use, one per profile (Sites overrides it in tests).
func (s *Service) siteFor(p core.Profile) (Site, error) {
	if site, ok := s.Sites[p.ID]; ok {
		return loggedSite{site, s, p.ID}, nil
	}
	if p.Kind != core.ProfileGoogleChat && p.Kind != core.ProfileHRmaster {
		return nil, core.Err("site.noWindow", "name", p.Title())
	}
	if s.NewSite == nil {
		return nil, core.Err("site.noWindow", "name", p.Title())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.webSites == nil {
		s.webSites = map[string]Site{}
	}
	w := s.webSites[p.ID]
	if w == nil {
		id := p.ID
		home := func() string {
			cur, ok := s.current().Profile(id)
			if !ok {
				return ""
			}
			u, _ := profileHome(cur)
			return u
		}
		spec := SiteSpec{
			ID: id, Kind: p.Kind, Name: p.Title(), Home: home, StateFile: filepath.Join(s.StateDir, "signed-in-"+id),
			Hidden: s.siteHidden,
		}
		if p.Kind == core.ProfileGoogleChat {
			spec.SignedInAt = web.GoogleChatSignedIn
			spec.SignInURL = func() string {
				cur, _ := s.current().Profile(id)
				return web.GoogleSignInFor(cur.Email)
			}
		} else {
			spec.SignedInAt = web.HRmasterSignedIn
		}
		w = s.NewSite(spec)
		s.webSites[id] = w
	}
	return loggedSite{w, s, p.ID}, nil
}

// forgetSite closes a removed profile's window and drops it, so a new profile can't inherit it.
func (s *Service) forgetSite(id string) {
	s.mu.Lock()
	w := s.webSites[id]
	delete(s.webSites, id)
	s.mu.Unlock()
	if w != nil {
		w.Close()
	}
}

// profileHome is the page that proves a profile's sign-in: Google Chat for the account, HRmaster's work-time page.
func profileHome(p core.Profile) (string, error) {
	switch p.Kind {
	case core.ProfileGoogleChat:
		return web.GoogleURL("https://chat.google.com/", p.Email), nil
	case core.ProfileHRmaster:
		base, ok := web.NormalizeHRmaster(p.URL)
		if !ok {
			return "", core.Err("hrmaster.needAddress", "name", p.Title())
		}
		return web.WorkingTimeURL(base), nil
	}
	return "", core.Err("site.noWindow", "name", p.Title())
}

func (s *Service) profileSite(id string) (core.Profile, Site, error) {
	p, ok := s.current().Profile(id)
	if !ok {
		return p, nil, fmt.Errorf("no profile %q", id)
	}
	site, err := s.siteFor(p)
	return p, site, err
}

// SignedIn is the last sign-in check of a profile's window; nil before the first one.
func (s *Service) SignedIn(profileID string) (*bool, error) {
	_, site, err := s.profileSite(profileID)
	if err != nil {
		return nil, err
	}
	return site.LastSignedIn(), nil
}

// OpenSite shows the profile's site in its window, to sign in.
func (s *Service) OpenSite(profileID string) error {
	p, site, err := s.profileSite(profileID)
	if err != nil {
		return err
	}
	if _, err := profileHome(p); err != nil {
		return err // not set up yet: say what's missing
	}
	return site.SignIn(context.Background())
}

func (s *Service) SignOut(profileID string) error {
	_, site, err := s.profileSite(profileID)
	if err != nil {
		return err
	}
	return site.SignOut(context.Background())
}
