package app

import (
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// A new token takes effect without a restart: the read-back done with the old one is dropped, and a new Jira
// token also drops the account id it was read for.
func TestSetTokenForgetsWhatTheOldOneRead(t *testing.T) {
	s, _ := newService(t)
	s.Secrets.Unlock()
	a := core.NewJiraAccount("j1")
	a.BaseURL, a.AccountID = "https://acme.atlassian.net", "old-user"
	must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a} }))
	fill := func() { s.tempoCache["2026-03-03"] = []tempo.Worklog{{ID: 1}} }

	fill()
	must(t, s.SetToken("j1", "tempo", "new-tempo"))
	if len(s.tempoCache) != 0 {
		t.Error("tempo token: read-back kept")
	}
	if got, _ := s.jiraAccount("j1"); got.AccountID != "old-user" {
		t.Errorf("tempo token: account id %q", got.AccountID)
	}

	fill()
	must(t, s.SetToken("j1", "jira", "new-jira"))
	if got, _ := s.jiraAccount("j1"); got.AccountID != "" || len(s.tempoCache) != 0 {
		t.Errorf("jira token: account id %q, read-back %v", got.AccountID, s.tempoCache)
	}
}

// A Confluence profile uses its linked Jira profile's site, email and token, until it has its own; removing it
// takes its own token along, and removing the Jira profile unlinks it.
func TestConfluenceCredentials(t *testing.T) {
	s, _ := newService(t)
	s.Secrets.Unlock()
	a := core.NewJiraAccount("j1")
	a.BaseURL, a.Email = "https://acme.atlassian.net/", "me@acme.com"
	must(t, s.update(func(st *core.Settings) { st.JiraAccounts = []core.JiraAccount{a} }))
	must(t, s.SetToken("j1", "jira", "jira-token"))
	e, err := s.AddProfile("confluence")
	must(t, err)
	p, _ := s.current().Profile(e.ID)
	if c := s.confluence(p); p.JiraAccountID != "j1" || c.Site != "https://acme.atlassian.net" || c.Email != "me@acme.com" || c.Token != "jira-token" {
		t.Errorf("linked: %+v %+v", p, c)
	}
	must(t, s.SetToken(e.ID, "confluence", "own-token"))
	if c := s.confluence(p); c.Token != "own-token" || !s.HasToken(e.ID, "confluence") {
		t.Errorf("own token: %+v", c)
	}
	_, err = s.RemoveProfile("j1")
	must(t, err)
	if p, _ = s.current().Profile(e.ID); p.JiraAccountID != "" || s.confluence(p).Site != "" {
		t.Errorf("unlinked: %+v", p)
	}
	_, err = s.RemoveProfile(e.ID)
	must(t, err)
	if v, _ := s.Secrets.Get(p.ConfluenceTokenKey()); v != "" {
		t.Error("its token stays behind")
	}
}
