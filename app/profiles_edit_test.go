package app

import (
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// RELEASE_REVIEW A10: no bound method returns a token; keyring names are made in Go.
func TestTokensStayInGo(t *testing.T) {
	s, _ := newService(t)
	j, err := s.AddProfile("jira")
	must(t, err)
	if err := s.SetToken(j.ID, "jira", "t"); err == nil {
		t.Error("a token was set while locked")
	}
	s.Unlock()
	must(t, s.SetToken(j.ID, "jira", " t "))
	if !s.HasToken(j.ID, "jira") || s.HasToken(j.ID, "tempo") {
		t.Error("HasToken")
	}
	if v, _ := s.Secrets.Get(j.Settings.JiraAccounts[0].JiraTokenKey()); v != "t" {
		t.Errorf("stored %q under the account's key", v)
	}
	if err := s.SetToken(j.ID, "google.refreshToken", "x"); err == nil {
		t.Error("an arbitrary kind was accepted")
	}
	if err := s.SetToken("nobody", "jira", "x"); err == nil {
		t.Error("an unknown account was accepted")
	}
	must(t, s.ClearToken(j.ID, "jira"))
	if s.HasToken(j.ID, "jira") {
		t.Error("ClearToken")
	}
	must(t, s.SetGoogleClientSecret("sec"))
	if !s.HasGoogleClientSecret() {
		t.Error("Google client secret")
	}
}

func TestProfilesByIntent(t *testing.T) {
	s, _ := newService(t)
	s.Unlock()

	t.Run("a Tempo profile needs a Jira profile without one", func(t *testing.T) {
		e, err := s.AddProfile("tempo")
		must(t, err)
		if !e.NeedsJira || e.ID != "" {
			t.Errorf("%+v", e)
		}
	})
	j1, err := s.AddProfile("jira")
	must(t, err)
	j2, err := s.AddProfile("jira")
	must(t, err)
	t.Run("the first Jira profile becomes the default", func(t *testing.T) {
		if d := s.Settings().DefaultJiraAccountID; d == nil || *d != j1.ID {
			t.Errorf("default %v", d)
		}
	})
	tp, err := s.AddProfile("tempo")
	must(t, err)
	t.Run("a new Tempo profile links the first Jira profile without one", func(t *testing.T) {
		p := tp.Settings.Profiles[len(tp.Settings.Profiles)-1]
		if p.Kind != core.ProfileTempo || p.JiraAccountID != j1.ID || p.URL != core.DefaultTempoURL {
			t.Errorf("%+v", p)
		}
	})
	t.Run("moving a Tempo profile to another Jira profile takes its token along", func(t *testing.T) {
		must(t, s.SetToken(j1.ID, "tempo", "tt"))
		p := tp.Settings.Profiles[len(tp.Settings.Profiles)-1]
		p.JiraAccountID = j2.ID
		_, err := s.UpdateProfile(p)
		must(t, err)
		if s.HasToken(j1.ID, "tempo") || !s.HasToken(j2.ID, "tempo") {
			t.Error("token not moved")
		}
	})
	t.Run("removing a Jira profile deletes its tokens and its Tempo profile, and moves the default", func(t *testing.T) {
		must(t, s.SetToken(j1.ID, "jira", "jt"))
		_, err := s.SetDefaultJiraAccount(j2.ID)
		must(t, err)
		st, err := s.RemoveProfile(j2.ID)
		must(t, err)
		if s.HasToken(j2.ID, "tempo") || len(st.JiraAccounts) != 1 || *st.DefaultJiraAccountID != j1.ID {
			t.Errorf("%+v", st)
		}
		for _, p := range st.Profiles {
			if p.Kind == core.ProfileTempo {
				t.Error("the Tempo profile linking it stayed")
			}
		}
	})
	t.Run("unknown kinds are refused", func(t *testing.T) {
		if _, err := s.AddProfile("myspace"); err == nil {
			t.Error("accepted")
		}
	})
}

func TestRemovingAJiraProfileNeedsTheKeyring(t *testing.T) {
	s, _ := newService(t)
	j, err := s.AddProfile("jira")
	must(t, err)
	if _, err := s.RemoveProfile(j.ID); err == nil {
		t.Error("removed while locked: its tokens would stay behind")
	}
	if len(s.Settings().JiraAccounts) != 1 {
		t.Error("a refused removal changed the settings")
	}
}
