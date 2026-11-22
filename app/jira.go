package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// ── Tokens ──────────────────────────────────────────────────────────────────
//
// Tokens never go to the webview: the frontend only learns whether one is saved, and the keyring names are
// made here (RELEASE_REVIEW A10).

var errLocked = core.Err("locked.tokens")

// tokenKey is a Jira profile's keyring item for its Jira or Tempo token, a Confluence profile's for its own token,
// or an Email profile's for its IMAP password.
func (s *Service) tokenKey(accountID, kind string) (string, error) {
	if kind == "confluence" {
		p, err := s.confluenceProfile(accountID)
		return p.ConfluenceTokenKey(), err
	}
	if kind == "email" {
		p, err := s.emailProfile(accountID)
		return p.EmailPasswordKey(), err
	}
	a, err := s.jiraAccount(accountID)
	if err != nil {
		return "", err
	}
	switch kind {
	case "jira":
		return a.JiraTokenKey(), nil
	case "tempo":
		return a.TempoTokenKey(), nil
	}
	return "", fmt.Errorf("unknown token kind %q", kind)
}

// HasToken: a Jira profile's Jira or Tempo token (kind "jira" or "tempo"), or a Confluence profile's own token
// (kind "confluence", by the profile's id), is saved. False while locked.
func (s *Service) HasToken(accountID, kind string) bool {
	key, err := s.tokenKey(accountID, kind)
	if err != nil {
		return false
	}
	v, ok := s.Secrets.Get(key)
	return ok && v != ""
}

// SetToken saves a Jira profile's Jira or Tempo token, or a Confluence profile's own; an empty value deletes it. What was learnt with the old
// token goes with it, so the new one takes effect without a restart: the read-back from Tempo, and for a Jira
// token the site's gateway choice and your account id (a token for another user is another account).
func (s *Service) SetToken(accountID, kind, value string) error {
	key, err := s.tokenKey(accountID, kind)
	if err != nil {
		return err
	}
	if s.Secrets.Locked() {
		return errLocked
	}
	if err := s.Secrets.Set(key, strings.TrimSpace(value)); err != nil {
		return err
	}
	if kind == "email" {
		return nil
	}
	if kind == "confluence" {
		p, _ := s.confluenceProfile(accountID)
		tempo.ForgetGateway(s.confluence(p).Site)
		return nil
	}
	s.forgetTempo()
	if kind == "jira" {
		a, _ := s.jiraAccount(accountID)
		tempo.ForgetGateway(a.Site())
		return s.setAccount(accountID, func(x *core.JiraAccount) { x.AccountID = "" })
	}
	return nil
}

// ClearToken deletes a Jira profile's Jira or Tempo token.
func (s *Service) ClearToken(accountID, kind string) error { return s.SetToken(accountID, kind, "") }

// HasGoogleClientSecret: the Google OAuth client secret (Calendar over the API) is saved.
func (s *Service) HasGoogleClientSecret() bool {
	v, ok := s.Secrets.Get(core.SecretGoogleSecret)
	return ok && v != ""
}

// SetGoogleClientSecret saves the Google OAuth client secret; an empty value deletes it.
func (s *Service) SetGoogleClientSecret(value string) error {
	if s.Secrets.Locked() {
		return errLocked
	}
	return s.Secrets.Set(core.SecretGoogleSecret, strings.TrimSpace(value))
}

// ── Jira accounts ───────────────────────────────────────────────────────────

func (s *Service) jiraAccount(id string) (core.JiraAccount, error) {
	for _, a := range s.current().JiraAccounts {
		if a.ID == id {
			return a, nil
		}
	}
	return core.JiraAccount{}, fmt.Errorf("no Jira account %q", id)
}

func (s *Service) setAccount(id string, change func(*core.JiraAccount)) error {
	return s.update(func(st *core.Settings) {
		for i := range st.JiraAccounts {
			if st.JiraAccounts[i].ID == id {
				change(&st.JiraAccounts[i])
			}
		}
	})
}

// accountID is your Jira account id on the account's site, read once and kept.
func (s *Service) accountID(ctx context.Context, a core.JiraAccount, c tempo.Service) (string, error) {
	if a.AccountID != "" {
		return a.AccountID, nil
	}
	id, err := c.AccountID(ctx)
	if err != nil {
		return "", err
	}
	return id, s.setAccount(a.ID, func(x *core.JiraAccount) { x.AccountID = id })
}

func (s *Service) today() string { return s.Days.Key(s.Clock.Now()) }

func (s *Service) CheckJira(accountID string) ([]tempo.Line, error) {
	a, err := s.jiraAccount(accountID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lines, me := tempo.NewCheck(a, s.Secrets, s.HTTP, s.today()).Jira(ctx)
	if me != "" && me != a.AccountID {
		s.logFailure("jira", "Saving the Jira account id failed", s.setAccount(a.ID, func(x *core.JiraAccount) { x.AccountID = me }))
	}
	return lines, nil
}

func (s *Service) CheckTempo(accountID string) ([]tempo.Line, error) {
	a, err := s.jiraAccount(accountID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	check := tempo.NewCheck(a, s.Secrets, s.HTTP, s.today())
	me := a.AccountID
	if me == "" {
		_, me = check.Jira(ctx) // Tempo needs the Jira account id
	}
	return check.Tempo(ctx, me), nil
}

// ReadProjects reads the site's project keys into the account, so its issues are sent there.
func (s *Service) ReadProjects(accountID string) ([]string, error) {
	a, err := s.jiraAccount(accountID)
	if err != nil {
		return nil, err
	}
	c, err := tempo.NewClient(a, s.Secrets, s.HTTP, s.Days)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	keys, err := c.ProjectKeys(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, s.setAccount(a.ID, func(x *core.JiraAccount) { x.ProjectKeys = keys })
}

// ── Export ──────────────────────────────────────────────────────────────────

func dayRange(logs []core.Worklog) (string, string) {
	if len(logs) == 0 {
		return "", ""
	}
	from, to := logs[0].Day, logs[0].Day
	for _, w := range logs {
		from, to = min(from, w.Day), max(to, w.Day)
	}
	return from, to
}

func (s *Service) site0() string {
	if a := s.current().DefaultJiraAccount(); a != nil {
		return a.Site()
	}
	return ""
}

// ExportText is the Markdown checklist for Claude in Chrome.
func (s *Service) ExportText(ids []int64) (string, error) {
	logs, err := s.Store.WorklogsByID(ids)
	if err != nil {
		return "", err
	}
	from, to := dayRange(logs)
	return tempo.Markdown(logs, from, to, s.site0(), s.Days), nil
}

// ExportFile asks where to save, then writes the file. Nil when cancelled.
func (s *Service) ExportFile(ids []int64, format string) (*string, error) {
	logs, err := s.Store.WorklogsByID(ids)
	if err != nil {
		return nil, err
	}
	from, to := dayRange(logs)
	var data []byte
	ext := format
	switch format {
	case "csv":
		data = []byte(tempo.CSV(logs))
	case "json":
		accountID := ""
		if a := s.current().DefaultJiraAccount(); a != nil {
			accountID = a.AccountID
		}
		if data, err = tempo.JSON(logs, accountID, s.Days); err != nil {
			return nil, err
		}
	case "markdown":
		data, ext = []byte(tempo.Markdown(logs, from, to, s.site0(), s.Days)), "md"
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
	path, err := s.Shell.SaveFile(fmt.Sprintf("tempo-%s_%s.%s", from, to, ext))
	if err != nil || path == "" {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}
	return &path, nil
}

// PushDayToTempo sends one day's reviewed entries, by hand and one day at a time: one request per entry, each in
// the account that owns its project (Tempo or Jira), and marks the sent ones exported. The day must be over or
// today and have no drafts left. Before sending, each account's worklogs for the day are read back and the
// ticket-days Tempo already has are left out, so nothing is logged twice; an account whose read-back fails sends
// nothing.
func (s *Service) PushDayToTempo(day string) error {
	if _, ok := s.Days.Parse(day); !ok {
		return fmt.Errorf("not a day: %q", day)
	}
	if day > s.today() {
		return core.Err("push.future", "day", day)
	}
	ready, plan, err := s.pushPlan(day, day)
	if err != nil {
		return err
	}
	if len(plan.DraftDays) > 0 {
		return core.Err("push.dayHasDrafts", "n", plan.drafts, "day", day)
	}
	if len(s.current().JiraAccounts) == 0 && len(ready) > 0 {
		return core.Err("missing.jiraAccount")
	}
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()
	sent, already, failures := s.push(ctx, ready, day, day)
	if len(failures) > 0 {
		return s.finish(ctx, core.Message{}, core.Err("push.failed", "n", sent, "reason", strings.Join(failures, " | ")))
	}
	return s.finish(ctx, core.Say("push.dayDone", "n", sent, "day", day, "already", already), nil)
}

// PushPreviewDTO is what sending a range to Tempo would do (PushRangeToTempo), for the question asked first.
type PushPreviewDTO struct {
	// Entries and Seconds are the reviewed entries that would be sent; Days how many days they are on.
	Entries int `json:"entries"`
	Seconds int `json:"seconds"`
	Days    int `json:"days"`
	// DraftDays are the days left out because they still have drafts.
	DraftDays []string `json:"draftDays"`
	// Later is the reviewed entries on days that haven't come yet, left out.
	Later int `json:"later"`

	drafts int // the drafts on DraftDays
}

// pushPlan is the reviewed entries of the range that may go to Tempo: on days up to today that have no drafts
// left (a day goes whole or not at all), not sent yet and with a ticket.
func (s *Service) pushPlan(from, to string) ([]core.Worklog, PushPreviewDTO, error) {
	plan := PushPreviewDTO{DraftDays: []string{}}
	logs, err := s.Store.Worklogs(from, to)
	if err != nil {
		return nil, plan, err
	}
	byDay := map[string][]core.Worklog{}
	for _, w := range logs {
		byDay[w.Day] = append(byDay[w.Day], w)
	}
	today := s.today()
	var ready []core.Worklog
	for _, day := range s.Days.Range(from, to) {
		var dayReady []core.Worklog
		drafts := 0
		for _, w := range byDay[day] {
			switch {
			case w.Status == core.StatusDraft && w.Duration > 0:
				drafts++
			case w.Status == core.StatusReviewed && w.TempoID == nil && w.Exportable():
				dayReady = append(dayReady, w)
			}
		}
		switch {
		case day > today:
			plan.Later += len(dayReady)
		case drafts > 0:
			plan.DraftDays = append(plan.DraftDays, day)
			plan.drafts += drafts
		case len(dayReady) > 0:
			ready = append(ready, dayReady...)
			plan.Days++
			plan.Entries += len(dayReady)
			plan.Seconds += core.TotalDuration(dayReady)
		}
	}
	return ready, plan, nil
}

// PushPreview is what PushRangeToTempo would send for the days (inclusive). No network.
func (s *Service) PushPreview(from, to string) (PushPreviewDTO, error) {
	if _, _, err := s.span(from, to); err != nil {
		return PushPreviewDTO{DraftDays: []string{}}, err
	}
	_, plan, err := s.pushPlan(min(from, to), max(from, to))
	return plan, err
}

// PushRangeToTempo sends the reviewed entries of a week, a month or any days (inclusive) to Tempo, like
// PushDayToTempo for each day at once: days that still have drafts and days that haven't come yet are left out
// (and said so), and what Tempo already has for the range is read back first and left out.
func (s *Service) PushRangeToTempo(from, to string) error {
	if _, _, err := s.span(from, to); err != nil {
		return err
	}
	from, to = min(from, to), max(from, to)
	ready, plan, err := s.pushPlan(from, to)
	if err != nil {
		return err
	}
	if len(s.current().JiraAccounts) == 0 && len(ready) > 0 {
		return core.Err("missing.jiraAccount")
	}
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()
	sent, already, failures := 0, 0, []string(nil)
	if len(ready) > 0 {
		sent, already, failures = s.push(ctx, ready, from, to)
	}
	if len(failures) > 0 {
		return s.finish(ctx, core.Message{}, core.Err("push.failed", "n", sent, "reason", strings.Join(failures, " | ")))
	}
	if ctx.Err() != nil {
		return s.finish(ctx, core.Say("job.cancelled"), nil)
	}
	return s.finish(ctx, core.Say("push.rangeDone", "n", sent, "from", from, "to", to, "already", already,
		"drafts", len(plan.DraftDays), "later", plan.Later), nil)
}

// push sends the entries (all within [from, to]) to Tempo, each in the account that owns its project, and marks the
// sent ones exported: how many were sent, how many Tempo already had (left out), and what failed. Each account's
// worklogs for the days are read back first; an account whose read-back fails sends nothing.
func (s *Service) push(ctx context.Context, ready []core.Worklog, from, to string) (sent, already int, failures []string) {
	st := s.current()
	groups := map[string][]core.Worklog{}
	for _, w := range ready {
		if a := st.JiraAccountFor(*w.IssueKey); a != nil {
			groups[a.ID] = append(groups[a.ID], w)
		} else {
			failures = append(failures, *w.IssueKey+": no Jira profile has this project")
		}
	}
	for _, a := range st.JiraAccounts {
		group := groups[a.ID]
		if len(group) == 0 || ctx.Err() != nil {
			continue
		}
		if a.Provider == core.ProviderOther {
			failures = append(failures, a.Title()+": logged in another tool, not sent")
			continue
		}
		c, err := tempo.NewClient(a, s.Secrets, s.HTTP, s.Days)
		if err != nil {
			failures = append(failures, a.Title()+": "+err.Error())
			continue
		}
		me, err := s.accountID(ctx, a, c)
		if err != nil {
			failures = append(failures, a.Title()+": "+err.Error())
			continue
		}
		there, err := tempo.Read(ctx, s.Store, c, me, from, to)
		if err != nil {
			failures = append(failures, a.Title()+": "+err.Error())
			continue
		}
		covered := tempo.Coverage(group, there)
		var pending []core.Worklog
		for _, w := range group {
			if covered[*w.IssueKey+"|"+w.Day] {
				already++
				continue
			}
			pending = append(pending, w)
		}
		n, f := tempo.Push(ctx, s.Store, c, pending, me, s.Days, func(w core.Worklog, id int64) error {
			w.TempoID, w.Status, w.Locked = &id, core.StatusExported, true
			return s.Store.UpdateWorklog(w)
		}, func(i, n int) {
			s.progress("tempo", core.Say("progress.push", "i", i, "n", n))
		})
		sent += n
		failures = append(failures, f...)
	}
	s.forgetTempo() // what Tempo has changed: the next read-back fills it in again
	return sent, already, failures
}
