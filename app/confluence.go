package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// ── Confluence ──────────────────────────────────────────────────────────────

// confluenceProfile finds a Confluence profile by id.
func (s *Service) confluenceProfile(id string) (core.Profile, error) {
	if p, ok := s.current().Profile(id); ok && p.Kind == core.ProfileConfluence {
		return p, nil
	}
	return core.Profile{}, fmt.Errorf("no Confluence profile %q", id)
}

// confluence is a Confluence profile's client. A linked Jira profile gives the site, email and token; the profile's
// own site, email and token, when filled in, win (a Jira token with scopes doesn't work for Confluence).
func (s *Service) confluence(p core.Profile) tempo.Confluence {
	c := tempo.Confluence{HTTP: s.HTTP, Days: s.Days}
	if a, err := s.jiraAccount(p.JiraAccountID); err == nil {
		c.Site, c.Email = a.Site(), a.Email
		c.Token, _ = s.Secrets.Get(a.JiraTokenKey())
	}
	if site := (core.JiraAccount{BaseURL: p.URL}).Site(); site != "" {
		c.Site = site
	}
	if p.Email != "" {
		c.Email = p.Email
	}
	if t, _ := s.Secrets.Get(p.ConfluenceTokenKey()); t != "" {
		c.Token = t
	}
	return c
}

// CheckConfluence tests a Confluence profile's site, sign-in and permissions.
func (s *Service) CheckConfluence(profileID string) ([]tempo.Line, error) {
	p, err := s.confluenceProfile(profileID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.confluence(p).Check(ctx), nil
}

// profileEdit is a ConfluenceEdit with the profile it was read with.
type profileEdit struct {
	tempo.ConfluenceEdit
	profile, title string // title: the profile's, when there are several; else ""
}

// readConfluence reads your Confluence work over the days (inclusive) with every Confluence profile: what it
// found, how many profiles there are, and what failed ("profile: reason").
func (s *Service) readConfluence(ctx context.Context, from, to string) (edits []profileEdit, profiles int, failed []string) {
	all := s.current().ProfilesOf(core.ProfileConfluence)
	for _, p := range all {
		c := s.confluence(p)
		me, err := c.AccountID(ctx)
		var got []tempo.ConfluenceEdit
		if err == nil {
			got, err = c.Activity(ctx, me, from, to)
		}
		if err != nil {
			s.logFailure("confluence", "Reading "+p.Title()+"'s Confluence work failed", err)
			failed = append(failed, p.Title()+": "+err.Error())
			continue
		}
		title := ""
		if len(all) > 1 {
			title = p.Title()
		}
		for _, e := range got {
			edits = append(edits, profileEdit{e, p.ID, title})
		}
	}
	return edits, len(all), failed
}

// collectConfluence stores your Confluence work for the days (the Confluence module's part of Collect): how many
// page-days it found, and what it skipped.
func (s *Service) collectConfluence(ctx context.Context, from, to string) (int, []string) {
	if s.Secrets.Locked() {
		return 0, nil
	}
	s.progress("confluence", core.Say("progress.confluence"))
	edits, profiles, failed := s.readConfluence(ctx, from, to)
	if profiles == 0 {
		return 0, []string{core.Say("confluence.needProfile").Text}
	}
	if len(failed) == profiles {
		return 0, failed
	}
	start, end, _ := s.span(from, to)
	list := make([]store.ConfluenceActivity, len(edits))
	pages := map[string]bool{}
	for i, e := range edits {
		list[i] = store.ConfluenceActivity{PageID: e.PageID, Title: e.Title, Space: e.Space, At: e.At}
		pages[e.PageID+"|"+s.Days.Key(e.At)] = true
	}
	if err := s.Store.WithContext(ctx).SyncConfluenceActivity(start, end, list, len(failed) == 0); err != nil {
		return 0, append(failed, err.Error())
	}
	return len(pages), failed
}

// ── Confluence activity page ────────────────────────────────────────────────

// ConfluencePageDTO is what you did on one page in one day.
type ConfluencePageDTO struct {
	Profile      string `json:"profile"`      // the Confluence profile's id (OpenConfluencePage)
	ProfileTitle string `json:"profileTitle"` // its name, when there are several; else ""
	PageID       string `json:"pageId"`
	Kind         string `json:"kind"` // page or blogpost (ConfluenceChanges)
	Title        string `json:"title"`
	Space        string `json:"space"`
	// First and Last are the day's first and last activity on it ("15:04").
	First string `json:"first"`
	Last  string `json:"last"`
	// Edits is when you saved a version of it ("15:04"); Comments what you commented on it, and when.
	Edits    []string      `json:"edits"`
	Comments []JiraNoteDTO `json:"comments"`
	// IssueKey is a ticket key in the title (its work goes to that ticket), else "".
	IssueKey string `json:"issueKey"`
	// Entry is the draft or entry the page-day became (its suggested time), when collected; else nil.
	Entry *PlanEntryDTO `json:"entry"`
	// Suggested is the day's entries without a ticket that ran around this page's activity, when it has a key.
	Suggested []PlanEntryDTO `json:"suggested"`
}

type ConfluenceDayDTO struct {
	Day   string              `json:"day"`
	Pages []ConfluencePageDTO `json:"pages"`
}

// ConfluenceActivityDTO is the feed by day (newest first), how many entries (versions and comments) and page-days
// it has, and how many Confluence profiles it was read from (0: none set up).
type ConfluenceActivityDTO struct {
	Days     []ConfluenceDayDTO `json:"days"`
	Entries  int                `json:"entries"`
	Pages    int                `json:"pages"`
	Profiles int                `json:"profiles"`
}

// ConfluenceActivity reads what you did in Confluence over the days (inclusive) with every Confluence profile, by
// day and page: the versions you saved, your comments, the entry the page-day became and, for a page with a ticket
// key in its title, the entries without a ticket that ran around it. It goes to Confluence every time; Collect
// stores what the drafts are built from.
func (s *Service) ConfluenceActivity(from, to string) (ConfluenceActivityDTO, error) {
	out := ConfluenceActivityDTO{Days: []ConfluenceDayDTO{}}
	if _, _, err := s.span(from, to); err != nil {
		return out, err
	}
	if s.Secrets.Locked() {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	edits, profiles, failed := s.readConfluence(ctx, from, to)
	out.Profiles = profiles
	if len(failed) > 0 && len(failed) == profiles {
		return out, errors.New(strings.Join(failed, "; "))
	}
	logs, err := s.Store.Worklogs(from, to)
	if err != nil {
		return out, err
	}
	b, err := s.builder()
	if err != nil {
		return out, err
	}
	return buildConfluenceFeed(out, edits, logs, b.KnownPrefixes(), s.Days), nil
}

func buildConfluenceFeed(out ConfluenceActivityDTO, edits []profileEdit, logs []core.Worklog, known map[string]bool, days core.Days) ConfluenceActivityDTO {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].At.Before(edits[j].At) })
	type page struct {
		dto   ConfluencePageDTO
		times []time.Time
	}
	byDay := map[string][]*page{}
	index := map[string]*page{}
	for _, e := range edits {
		at := e.At.In(days.Loc)
		day, hm := days.Key(at), at.Format("15:04")
		id := e.profile + "|" + day + "|" + e.PageID
		p := index[id]
		if p == nil {
			p = &page{dto: ConfluencePageDTO{
				Profile: e.profile, ProfileTitle: e.title, PageID: e.PageID, Kind: e.Kind, Title: e.Title, Space: e.Space, First: hm,
				Edits: []string{}, Comments: []JiraNoteDTO{}, Suggested: []PlanEntryDTO{},
			}}
			if k, ok := core.FindKey(e.Title, known); ok {
				p.dto.IssueKey = k
			}
			index[id] = p
			byDay[day] = append(byDay[day], p)
			out.Pages++
		}
		p.dto.Last = hm
		p.times = append(p.times, at)
		out.Entries++
		if e.Comment {
			p.dto.Comments = append(p.dto.Comments, JiraNoteDTO{hm, e.Text})
		} else {
			p.dto.Edits = append(p.dto.Edits, hm)
		}
	}
	for _, w := range logs {
		list := byDay[w.Day]
		for _, p := range list {
			if slices.Contains(w.Refs(), (core.ConfluencePage{Day: w.Day, PageID: p.dto.PageID}).Ref()) {
				p.dto.Entry = &planEntries([]core.Worklog{w})[0]
			}
		}
		// Only a page with a ticket has one to offer.
		if i := nearest(w, len(list), func(i int) []time.Time {
			if list[i].dto.IssueKey == "" {
				return nil
			}
			return list[i].times
		}); i >= 0 {
			list[i].dto.Suggested = append(list[i].dto.Suggested, planEntries([]core.Worklog{w})...)
		}
	}
	keys := make([]string, 0, len(byDay))
	for d := range byDay {
		keys = append(keys, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, d := range keys {
		dd := ConfluenceDayDTO{Day: d, Pages: []ConfluencePageDTO{}}
		list := byDay[d]
		sort.SliceStable(list, func(i, j int) bool { return list[i].dto.Last > list[j].dto.Last })
		for _, p := range list {
			dd.Pages = append(dd.Pages, p.dto)
		}
		out.Days = append(out.Days, dd)
	}
	return out
}

var pageID = regexp.MustCompile(`^\d+$`)

// OpenConfluencePage opens a page of a Confluence profile's site in the browser. It takes the profile and the
// page id, not a URL, so the page can only open its own Confluence site.
func (s *Service) OpenConfluencePage(profileID, id string) error {
	p, err := s.confluenceProfile(profileID)
	if err != nil {
		return err
	}
	if !pageID.MatchString(id) {
		return fmt.Errorf("not a page id: %q", id)
	}
	site := s.confluence(p).Site
	if u, err := url.Parse(site); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return core.Err("missing.confluenceSite")
	}
	return s.Shell.OpenURL(site + "/wiki/pages/viewpage.action?pageId=" + id)
}

// ConfluenceChangeDTO is one version you saved of a page: when ("15:04"), its number and message, how many lines
// it added and removed, and the first few of them.
type ConfluenceChangeDTO struct {
	Time        string   `json:"time"`
	Number      int      `json:"number"`
	Message     string   `json:"message"`
	Added       int      `json:"added"`
	Removed     int      `json:"removed"`
	AddedText   []string `json:"addedText"`
	RemovedText []string `json:"removedText"`
}

// ConfluenceChanges is what you changed on one page (or blog post) on a day: each version you saved, against the
// one before it. Read from Confluence when asked (two reads per version).
func (s *Service) ConfluenceChanges(profileID, kind, id, day string) ([]ConfluenceChangeDTO, error) {
	p, err := s.confluenceProfile(profileID)
	if err != nil {
		return nil, err
	}
	if !pageID.MatchString(id) {
		return nil, fmt.Errorf("not a page id: %q", id)
	}
	if _, ok := s.Days.Parse(day); !ok {
		return nil, fmt.Errorf("not a day: %q", day)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := s.confluence(p)
	me, err := c.AccountID(ctx)
	if err != nil {
		return nil, err
	}
	list, err := c.Changes(ctx, me, kind, id, day, day)
	if err != nil {
		return nil, err
	}
	out := []ConfluenceChangeDTO{}
	for _, ch := range list {
		out = append(out, ConfluenceChangeDTO{
			Time: ch.At.In(s.Days.Loc).Format("15:04"), Number: ch.Number, Message: ch.Message,
			Added: ch.AddedLines, Removed: ch.Removed, AddedText: ch.AddedText, RemovedText: ch.RemovedText,
		})
	}
	return out, nil
}
