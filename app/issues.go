package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// ── Choosing a ticket for an entry ──────────────────────────────────────────

// IssueOptionDTO is a ticket offered for an entry without one.
type IssueOptionDTO struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
	// Origin is why it is offered: "plan" (Tempo planned it for the day), "day" (another entry of the day has it),
	// "period" (a project period covers the day), "recent" (an entry of this period or an earlier one has it), or
	// "jira" (a Jira search found it).
	Origin string `json:"origin"`
	// Day is when it was last used up to the month's end ("recent", "day"), or created ("jira"); "" otherwise.
	Day string `json:"day"`
	// Uses is how many entries have it ("recent", "day").
	Uses   int    `json:"uses"`
	Status string `json:"status"`
	// Description is the start of the issue's description, on one line ("jira"); "" otherwise.
	Description string `json:"description"`
	// Account is the Jira profile a search found it with (for openJiraIssue); "" otherwise.
	Account string `json:"account"`
	// AccountName names that profile when there are several, else "".
	AccountName string `json:"accountName"`
}

// recentDays is how far before the day's month IssueOptions looks for tickets in use.
const recentDays = 92

// IssueOptions is the tickets to offer for an entry of the day: planned for the day first, then those used that
// day, the project periods around it, then those used in the day's month or the months before it (latest first).
// It reads only what is stored (and Tempo's last read-back); nothing goes to the network.
func (s *Service) IssueOptions(day string) ([]IssueOptionDTO, error) {
	out := []IssueOptionDTO{}
	if _, ok := s.Days.Parse(day); !ok {
		return out, fmt.Errorf("not a day: %q", day)
	}
	_, monthEnd, _ := s.Days.Month(day[:7])
	from := s.Days.Shift(day[:7]+"-01", -recentDays)
	logs, err := s.Store.Worklogs(from, monthEnd)
	if err != nil {
		return out, err
	}
	summaries, err := s.Store.IssueSummaries()
	if err != nil {
		return out, err
	}
	byKey := map[string]*IssueOptionDTO{}
	rank := map[string]int{}
	add := func(key, origin, summary string, r int) *IssueOptionDTO {
		o, ok := byKey[key]
		if !ok {
			o = &IssueOptionDTO{Key: key, Origin: origin}
			byKey[key] = o
			rank[key] = r
		}
		if r < rank[key] {
			rank[key], o.Origin = r, origin
		}
		if o.Summary == "" {
			o.Summary = strings.TrimSpace(summary)
		}
		return o
	}
	for _, p := range s.planned(day, day) {
		if p.Key != "" {
			add(p.Key, "plan", p.Description, 0)
		}
	}
	for _, w := range logs {
		if w.IssueKey == nil {
			continue
		}
		origin, r := "recent", 3
		if w.Day == day {
			origin, r = "day", 1
		}
		o := add(*w.IssueKey, origin, "", r)
		o.Uses++
		o.Day = max(o.Day, w.Day)
	}
	for _, p := range s.current().Periods {
		if key := core.NormalizeKeyString(p.IssueKey); key != nil && p.From <= monthEnd && p.To >= from {
			r := 3
			if p.Contains(day) {
				r = 2
			}
			add(*key, "period", p.Title, r)
		}
	}
	for key, o := range byKey {
		if s, ok := summaries[key]; ok && s != "" {
			o.Summary = s
		}
		out = append(out, *o)
	}
	slices.SortFunc(out, func(a, b IssueOptionDTO) int {
		if ra, rb := rank[a.Key], rank[b.Key]; ra != rb {
			return ra - rb
		}
		if a.Day != b.Day {
			return strings.Compare(b.Day, a.Day) // latest first
		}
		return strings.Compare(a.Key, b.Key)
	})
	return out, nil
}

// SearchJiraIssues finds the Jira issues created within weeks (1–4) of the day whose summary has every word of
// text, with every Jira profile, nearest to the day first. A text that is an issue key finds that issue too.
func (s *Service) SearchJiraIssues(day, text string, weeks int) ([]IssueOptionDTO, error) {
	out := []IssueOptionDTO{}
	if _, ok := s.Days.Parse(day); !ok {
		return out, fmt.Errorf("not a day: %q", day)
	}
	if s.Secrets.Locked() {
		return out, errLocked
	}
	weeks = min(max(weeks, 1), 4)
	from, to := s.Days.Shift(day, -7*weeks), s.Days.Shift(day, 7*weeks)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	all := s.current().JiraAccounts
	accounts := 0
	var failed []string
	seen := map[string]bool{}
	for _, a := range all {
		jiraOnly := a
		jiraOnly.Provider = core.ProviderJira // a search needs only the Jira token
		c, err := tempo.NewClient(jiraOnly, s.Secrets, s.HTTP, s.Days)
		if err != nil {
			continue // no Jira token for this profile
		}
		accounts++
		found, err := c.SearchIssues(ctx, from, to, text)
		if err != nil {
			s.logFailure("jira", "Searching "+a.Title()+"'s Jira failed", err)
			failed = append(failed, a.Title()+": "+err.Error())
			continue
		}
		name := ""
		if len(all) > 1 {
			name = a.Title()
		}
		for _, is := range found {
			if seen[is.Key] {
				continue
			}
			seen[is.Key] = true
			created := ""
			if !is.Created.IsZero() {
				created = s.Days.Key(is.Created)
			}
			out = append(out, IssueOptionDTO{
				Key: is.Key, Summary: is.Summary, Description: is.Description, Origin: "jira", Day: created, Status: is.Status,
				Account: a.ID, AccountName: name,
			})
		}
	}
	switch {
	case accounts == 0:
		return out, core.Err("jira.searchNeedProfile")
	case len(failed) == accounts:
		return out, errors.New(strings.Join(failed, "; "))
	}
	target, _ := s.Days.Parse(day)
	typed := strings.TrimSpace(text)
	distance := func(o IssueOptionDTO) time.Duration {
		if strings.EqualFold(o.Key, typed) {
			return -time.Hour // the key typed in full, whenever it was created: first
		}
		t, ok := s.Days.Parse(o.Day)
		if !ok {
			return 1 << 62
		}
		return max(t.Sub(target), target.Sub(t))
	}
	slices.SortStableFunc(out, func(a, b IssueOptionDTO) int { return int((distance(a) - distance(b)) / time.Hour) })
	return out, nil
}
