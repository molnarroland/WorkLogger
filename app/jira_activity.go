package app

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// ── Jira activity ───────────────────────────────────────────────────────────

// JiraChangeDTO is a field you changed on a ticket ("15:04", the field as Jira names it, old and new value).
type JiraChangeDTO struct {
	Time  string `json:"time"`
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// JiraNoteDTO is a comment you wrote, or a file you attached (Text is its name).
type JiraNoteDTO struct {
	Time string `json:"time"`
	Text string `json:"text"`
}

// JiraTicketDTO is what you did on one ticket in one day.
type JiraTicketDTO struct {
	Account      string `json:"account"`      // the Jira profile's id (OpenJiraIssue)
	AccountTitle string `json:"accountTitle"` // its name, when there are several profiles; else ""
	IssueKey     string `json:"issueKey"`
	Summary      string `json:"summary"`
	// First and Last are the day's first and last activity on it ("15:04").
	First       string          `json:"first"`
	Last        string          `json:"last"`
	Created     bool            `json:"created"`
	Statuses    []JiraChangeDTO `json:"statuses"`
	Changes     []JiraChangeDTO `json:"changes"` // other fields than status and attachments
	Comments    []JiraNoteDTO   `json:"comments"`
	Attachments []JiraNoteDTO   `json:"attachments"`
	// Suggested is the day's entries without a ticket that ran around this ticket's activity: likely its work.
	Suggested []PlanEntryDTO `json:"suggested"`
}

type JiraDayDTO struct {
	Day     string          `json:"day"`
	Tickets []JiraTicketDTO `json:"tickets"`
}

// JiraActivityDTO is the feed by day (newest first), how many entries and tickets it has, and how many Jira
// profiles it was read from (0: none set up).
type JiraActivityDTO struct {
	Days     []JiraDayDTO `json:"days"`
	Entries  int          `json:"entries"`
	Tickets  int          `json:"tickets"`
	Accounts int          `json:"accounts"`
}

// JiraActivity reads what you did in Jira over the days (inclusive) with every Jira profile (issues you created,
// fields you changed, comments and attachments) by day and ticket, each with the entries without a ticket that
// ran around it. It goes to Jira every time; nothing is stored.
func (s *Service) JiraActivity(from, to string) (JiraActivityDTO, error) {
	out := JiraActivityDTO{Days: []JiraDayDTO{}}
	if _, _, err := s.span(from, to); err != nil {
		return out, err
	}
	if s.Secrets.Locked() {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	acts, accounts, failed := s.readJiraActivity(ctx, from, to)
	out.Accounts = accounts
	if len(failed) > 0 && len(failed) == out.Accounts {
		return out, errors.New(strings.Join(failed, "; "))
	}
	logs, err := s.Store.Worklogs(from, to)
	if err != nil {
		return out, err
	}
	return buildJiraFeed(out, acts, logs, s.Days), nil
}

// readJiraActivity reads your activity over the days (inclusive) with every Jira profile that has its tokens:
// what it found, how many profiles it read from, and what failed ("profile: reason").
func (s *Service) readJiraActivity(ctx context.Context, from, to string) (acts []accountActivity, accounts int, failed []string) {
	all := s.current().JiraAccounts
	for _, a := range all {
		c, err := tempo.NewClient(a, s.Secrets, s.HTTP, s.Days)
		if err != nil {
			continue // no tokens for this profile
		}
		accounts++
		me, err := s.accountID(ctx, a, c)
		var got []tempo.Activity
		if err == nil {
			got, err = c.Activity(ctx, me, from, to)
		}
		if err != nil {
			s.logFailure("jira", "Reading "+a.Title()+"'s Jira activity failed", err)
			failed = append(failed, a.Title()+": "+err.Error())
			continue
		}
		title := ""
		if len(all) > 1 {
			title = a.Title()
		}
		for _, it := range got {
			acts = append(acts, accountActivity{it, a.ID, title})
		}
	}
	return acts, accounts, failed
}

// collectJira stores your Jira activity for the days (the Jira module's part of Collect): how many ticket-days it
// found, and what it skipped.
func (s *Service) collectJira(ctx context.Context, from, to string) (int, []string) {
	if s.Secrets.Locked() {
		return 0, nil
	}
	s.progress("jira", core.Say("progress.jira"))
	acts, accounts, failed := s.readJiraActivity(ctx, from, to)
	if accounts == 0 {
		return 0, []string{core.Say("jira.needProfile").Text}
	}
	if len(failed) == accounts {
		return 0, failed
	}
	start, end, _ := s.span(from, to)
	list := make([]store.JiraActivity, len(acts))
	tickets := map[string]bool{}
	for i, a := range acts {
		list[i] = store.JiraActivity{IssueKey: a.IssueKey, Summary: a.Summary, At: a.At}
		tickets[a.IssueKey+"|"+s.Days.Key(a.At)] = true
	}
	if err := s.Store.WithContext(ctx).SyncJiraActivity(start, end, list, len(failed) == 0); err != nil {
		return 0, append(failed, err.Error())
	}
	return len(tickets), failed
}

type accountActivity struct {
	tempo.Activity
	account, title string
}

// suggestWithin is how near a ticket's activity an entry without a ticket must run to be suggested for it.
// Known limit: one fixed window; a busy day with activity on several tickets an hour apart may pick the wrong one
// (the nearest wins), which is why it is a suggestion the user applies.
const suggestWithin = time.Hour

func buildJiraFeed(out JiraActivityDTO, acts []accountActivity, logs []core.Worklog, days core.Days) JiraActivityDTO {
	sort.SliceStable(acts, func(i, j int) bool { return acts[i].At.Before(acts[j].At) })
	type ticket struct {
		dto   JiraTicketDTO
		times []time.Time
	}
	byDay := map[string][]*ticket{}
	index := map[string]*ticket{}
	for _, a := range acts {
		at := a.At.In(days.Loc)
		day, hm := days.Key(at), at.Format("15:04")
		id := a.account + "|" + day + "|" + a.IssueKey
		t := index[id]
		if t == nil {
			t = &ticket{dto: JiraTicketDTO{
				Account: a.account, AccountTitle: a.title, IssueKey: a.IssueKey, Summary: a.Summary, First: hm,
				Statuses: []JiraChangeDTO{}, Changes: []JiraChangeDTO{}, Comments: []JiraNoteDTO{},
				Attachments: []JiraNoteDTO{}, Suggested: []PlanEntryDTO{},
			}}
			index[id] = t
			byDay[day] = append(byDay[day], t)
			out.Tickets++
		}
		t.dto.Last = hm
		t.times = append(t.times, at)
		out.Entries++
		switch {
		case a.Kind == "created":
			t.dto.Created = true
		case a.Kind == "comment":
			t.dto.Comments = append(t.dto.Comments, JiraNoteDTO{hm, a.Text})
		case strings.EqualFold(a.Field, "status"):
			t.dto.Statuses = append(t.dto.Statuses, JiraChangeDTO{hm, a.Field, a.From, a.To})
		case strings.EqualFold(a.Field, "attachment"):
			if a.To != "" { // a removed attachment has only a From
				t.dto.Attachments = append(t.dto.Attachments, JiraNoteDTO{hm, a.To})
			}
		default:
			t.dto.Changes = append(t.dto.Changes, JiraChangeDTO{hm, a.Field, a.From, a.To})
		}
	}
	// Each entry without a ticket goes to the day's ticket whose activity is nearest its run, if near enough.
	for _, w := range logs {
		list := byDay[w.Day]
		if i := nearest(w, len(list), func(i int) []time.Time { return list[i].times }); i >= 0 {
			list[i].dto.Suggested = append(list[i].dto.Suggested, planEntries([]core.Worklog{w})...)
		}
	}
	keys := make([]string, 0, len(byDay))
	for d := range byDay {
		keys = append(keys, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, d := range keys {
		dd := JiraDayDTO{Day: d, Tickets: []JiraTicketDTO{}}
		list := byDay[d]
		sort.SliceStable(list, func(i, j int) bool { return list[i].dto.Last > list[j].dto.Last })
		for _, t := range list {
			dd.Tickets = append(dd.Tickets, t.dto)
		}
		out.Days = append(out.Days, dd)
	}
	return out
}

// nearest is which of n items (each with the moments of its activity) an entry without a ticket ran nearest to,
// within suggestWithin; -1 for none, or when the entry has a ticket or is in Tempo.
func nearest(w core.Worklog, n int, times func(int) []time.Time) int {
	if !w.NeedsIssue() || w.Status == core.StatusExported {
		return -1
	}
	best, bestGap := -1, suggestWithin+1
	end := w.Start.Add(time.Duration(w.Duration) * time.Second)
	for i := range n {
		for _, at := range times(i) {
			if g := gap(at, w.Start, end); g < bestGap {
				best, bestGap = i, g
			}
		}
	}
	return best
}

// gap is how far t is from [start, end]: 0 inside it.
func gap(t, start, end time.Time) time.Duration {
	switch {
	case t.Before(start):
		return start.Sub(t)
	case t.After(end):
		return t.Sub(end)
	}
	return 0
}

var issueKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-\d+$`)

// OpenJiraIssue opens an issue of a Jira profile in the browser. It takes the profile and the key, not a URL, so
// the page can only open its own Jira site.
func (s *Service) OpenJiraIssue(accountID, key string) error {
	a, err := s.jiraAccount(accountID)
	if err != nil {
		return err
	}
	if !issueKey.MatchString(key) {
		return core.Err("issue.invalid", "key", key)
	}
	site := a.Site()
	if u, err := url.Parse(site); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return core.Err("missing.jiraSite")
	}
	return s.Shell.OpenURL(site + "/browse/" + key)
}
