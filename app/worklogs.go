package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/tempo"
	"github.com/molnarroland/WorkLogger/internal/view"
)

// Month is the timesheet: every day with what it should add up to and where it stands, the worklogs, the
// to-dos, tickets and totals (internal/view). Tempo's read-back counts when the keyring is unlocked and an
// account is set up. An empty month is the current one.
func (s *Service) Month(month string) (MonthDTO, error) {
	if month == "" {
		month = s.today()[:7]
	}
	from, to, ok := s.Days.Month(month)
	if !ok {
		return MonthDTO{}, fmt.Errorf("not a month: %q", month)
	}
	r, err := s.loadRange(from, to)
	if err != nil {
		return MonthDTO{}, err
	}
	m := view.BuildMonth(view.MonthInput{
		Month: month, Today: s.today(), Days: s.Days, Info: r.info, Worklogs: r.logs, InTempo: r.covered,
		Periods: r.periods, Planned: r.planned,
	})
	out := monthDTO(m, r.inTempo())
	settled, err := s.settledDays(from, to)
	if err != nil {
		return MonthDTO{}, err
	}
	for _, d := range s.Days.Range(from, to) {
		if settled[d] {
			out.Settled = append(out.Settled, d)
		}
	}
	return out, nil
}

// Week is Monday to Sunday around day (the Week view), across a month end if it falls on one. An empty day is today.
func (s *Service) Week(day string) (WeekDTO, error) {
	if day == "" {
		day = s.today()
	}
	if _, ok := s.Days.Parse(day); !ok {
		return WeekDTO{}, fmt.Errorf("not a day: %q", day)
	}
	days := view.WeekOf(s.Days, day)
	r, err := s.loadRange(days[0], days[6])
	if err != nil {
		return WeekDTO{}, err
	}
	w := view.BuildWeek(view.WeekInput{
		Day: day, Today: s.today(), Days: s.Days, Info: r.info, Worklogs: r.logs, InTempo: r.covered,
		Periods: r.periods, Planned: r.planned,
	})
	return weekDTO(w), nil
}

// loaded is what the views are built from for a range of days.
type loaded struct {
	info    []view.DayInfo
	logs    []core.Worklog
	covered map[string]bool
	periods []core.ProjectPeriod
	planned map[string]int // Tempo Planner's time by "KEY|day"
}

func (l loaded) inTempo() []string {
	out := []string{}
	for ref := range l.covered {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

// loadRange reads the days' worklogs, what each day should add up to (with HR's records and sessions when the
// work-times module is on), and Tempo's read-back.
func (s *Service) loadRange(from, to string) (loaded, error) {
	var out loaded
	logs, err := s.Store.Worklogs(from, to)
	if err != nil {
		return out, err
	}
	b, err := s.builder()
	if err != nil {
		return out, err
	}
	sessions := map[string][]core.WorkSession{}
	records := map[string]*core.DayRecord{}
	if b.Settings.Has(core.ModuleWorkTimes) {
		list, err := s.Store.SessionsRange(from, to)
		if err != nil {
			return out, err
		}
		for _, w := range list {
			sessions[w.Day] = append(sessions[w.Day], w)
		}
		if records, err = s.Store.DayRecordMap(from, to); err != nil {
			return out, err
		}
	}
	for _, day := range s.Days.Range(from, to) {
		d := view.DayInfo{Day: day, Expected: b.Expected(day, sessions[day], records[day])}
		if r := records[day]; r != nil {
			d.Label, d.Absence = r.Label, r.Status.IsAbsence()
		}
		out.info = append(out.info, d)
	}
	out.logs = logs
	out.covered = tempo.Coverage(logs, s.tempoLogs(from, to))
	out.planned = view.PlannedByRef(s.planned(from, to))
	out.periods = b.Settings.Periods
	return out, nil
}

// UseDescriptionsAsComments gives each of the day's drafts without a Tempo comment its own description as the
// comment (they lock, like any edit). Drafts without a description stay as they are.
func (s *Service) UseDescriptionsAsComments(day string) error {
	if _, ok := s.Days.Parse(day); !ok {
		return fmt.Errorf("not a day: %q", day)
	}
	return s.Store.Tx(func(db *store.Store) error {
		logs, err := db.WorklogsOn(day)
		if err != nil {
			return err
		}
		for _, w := range logs {
			if !view.NeedsComment(w) || strings.TrimSpace(w.Description) == "" {
				continue
			}
			w.Comment, w.Locked = strings.TrimSpace(w.Description), true
			if err := db.UpdateWorklog(w); err != nil {
				return err
			}
		}
		return nil
	})
}

// tempoLogs is what the last RefreshTempo read for the days: your worklogs already in Tempo/Jira. It never
// goes to the network, so the month and week views stay cheap reads (RELEASE_REVIEW C3).
func (s *Service) tempoLogs(from, to string) []tempo.Worklog {
	s.tempoMu.Lock()
	defer s.tempoMu.Unlock()
	var out []tempo.Worklog
	for _, day := range s.Days.Range(from, to) {
		out = append(out, s.tempoCache[day]...)
	}
	return out
}

// RefreshTempo reads back what Tempo (or Jira) already has for the month's days, for every Jira profile with its
// tokens, and keeps it for the month and week views. It reports whether that changed what the month shows, so the
// frontend reloads only then. A failed read changes nothing and is reported; profiles without tokens are skipped,
// and so is everything while the keyring is locked. An empty month is the current one.
func (s *Service) RefreshTempo(month string) (bool, error) {
	if month == "" {
		month = s.today()[:7]
	}
	from, to, ok := s.Days.Month(month)
	if !ok {
		return false, fmt.Errorf("not a month: %q", month)
	}
	if s.Secrets.Locked() {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var logs []tempo.Worklog
	var plans []tempo.Plan
	var failed []string
	for _, a := range s.current().JiraAccounts {
		if a.Provider == core.ProviderOther {
			continue
		}
		c, err := tempo.NewClient(a, s.Secrets, s.HTTP, s.Days)
		if err != nil {
			continue // no tokens for this profile
		}
		me, err := s.accountID(ctx, a, c)
		if err == nil && len(a.ProjectKeys) == 0 {
			_, err := s.ReadProjects(a.ID)
			s.logFailure("jira", "Reading "+a.Title()+"'s projects failed", err)
		}
		var got []tempo.Worklog
		if err == nil {
			got, err = tempo.Read(ctx, s.Store, c, me, from, to)
		}
		if err != nil {
			failed = append(failed, a.Title()+": "+err.Error())
			continue
		}
		logs = append(logs, got...)
		// Plans are extra: a token without the Planner scope still reads back the worklogs.
		p, err := tempo.ReadPlans(ctx, s.Store, c, me, from, to)
		s.logFailure("tempo", "Reading "+a.Title()+"'s Tempo plans failed", err)
		plans = append(plans, p...)
	}
	if len(failed) > 0 {
		err := core.Err("tempo.readBackFailed", "reason", strings.Join(failed, "; "))
		s.logActivity("tempo", "error", err.Error())
		return false, err
	}
	local, err := s.Store.Worklogs(from, to)
	if err != nil {
		return false, err
	}
	before := tempo.Coverage(local, s.tempoLogs(from, to))
	plannedBefore := view.PlannedByRef(s.planned(from, to))
	byDay := map[string][]tempo.Worklog{}
	for _, w := range logs {
		byDay[w.Day] = append(byDay[w.Day], w)
	}
	plansByDay := map[string][]tempo.Plan{}
	for _, p := range plans {
		plansByDay[p.Day] = append(plansByDay[p.Day], p)
	}
	s.tempoMu.Lock()
	for _, day := range s.Days.Range(from, to) {
		s.tempoCache[day] = byDay[day]
		s.planCache[day] = plansByDay[day]
	}
	s.tempoMu.Unlock()
	changed := !maps.Equal(before, tempo.Coverage(local, logs)) ||
		!maps.Equal(plannedBefore, view.PlannedByRef(s.planned(from, to)))
	return changed, nil
}

func (s *Service) forgetTempo() {
	s.tempoMu.Lock()
	s.tempoCache = map[string][]tempo.Worklog{}
	s.planCache = map[string][]tempo.Plan{}
	s.tempoMu.Unlock()
}

func (s *Service) one(id int64) (core.Worklog, error) {
	list, err := s.Store.WorklogsByID([]int64{id})
	if err != nil {
		return core.Worklog{}, err
	}
	if len(list) == 0 {
		return core.Worklog{}, fmt.Errorf("no worklog %d", id)
	}
	return list[0], nil
}

// UpdateWorklog saves an edit. A change to start, duration, issue, description or status locks the entry.
func (s *Service) UpdateWorklog(in WorklogDTO) (WorklogDTO, error) {
	w, err := s.one(in.ID)
	if err != nil {
		return WorklogDTO{}, err
	}
	start, err := time.Parse(time.RFC3339, in.Start)
	if err != nil {
		return WorklogDTO{}, fmt.Errorf("bad start %q", in.Start)
	}
	if in.Duration < 0 {
		return WorklogDTO{}, errors.New("the duration can't be negative")
	}
	if _, ok := s.Days.Parse(in.Day); !ok {
		return WorklogDTO{}, fmt.Errorf("not a day: %q", in.Day)
	}
	if s.Days.Key(start) != in.Day {
		return WorklogDTO{}, fmt.Errorf("the start %s is not on %s", in.Start, in.Day)
	}
	issue := core.NormalizeKey(in.IssueKey)
	status := core.Status(in.Status)
	switch status {
	case core.StatusDraft, core.StatusReviewed, core.StatusExported:
	default:
		return WorklogDTO{}, fmt.Errorf("unknown status %q", in.Status)
	}
	comment := strings.TrimSpace(in.Comment)
	if status == core.StatusReviewed && w.Status != core.StatusReviewed && comment == "" {
		return WorklogDTO{}, errNeedsComment
	}
	changed := !w.Start.Equal(start) || w.Duration != in.Duration || str(w.IssueKey) != str(issue) ||
		w.Description != in.Description || w.Comment != comment || w.Status != status
	// Sent by the API: Tempo has it as it is, so only WorkLogger's own note and the lock may change.
	if w.TempoID != nil && (w.Day != in.Day || !w.Start.Equal(start) || w.Duration != in.Duration ||
		str(w.IssueKey) != str(issue) || w.Comment != comment || w.Status != status) {
		return WorklogDTO{}, errSent
	}
	w.Day, w.Start, w.Duration, w.IssueKey, w.Description, w.Status = in.Day, start, in.Duration, issue, in.Description, status
	w.Comment = comment
	w.Locked = in.Locked || changed
	if err := s.Store.UpdateWorklog(w); err != nil {
		return WorklogDTO{}, err
	}
	return s.worklogDTO(w), nil
}

// SetDuration sets an entry's duration from what was typed: "1h 30m", "1.5h", "1,5h", "90m", "1:30" or minutes.
// It locks the entry, like any edit.
func (s *Service) SetDuration(id int64, text string) (WorklogDTO, error) {
	seconds, ok := core.ParseDuration(text)
	if !ok {
		return WorklogDTO{}, core.Err("worklog.badDuration", "text", text)
	}
	w, err := s.one(id)
	if err != nil {
		return WorklogDTO{}, err
	}
	in := s.worklogDTO(w)
	in.Duration = seconds
	return s.UpdateWorklog(in)
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// endOfDay is where the next entry of a day goes: after its last entry, or at the start of the workday.
func (s *Service) endOfDay(db *store.Store, day string) (time.Time, error) {
	logs, err := db.WorklogsOn(day)
	if err != nil {
		return time.Time{}, err
	}
	var last time.Time
	for _, w := range logs {
		if w.End().After(last) {
			last = w.End()
		}
	}
	if !last.IsZero() {
		return last, nil
	}
	if t, ok := s.Days.AtHM(day, s.current().DefaultDayStart); ok {
		return t, nil
	}
	return s.Days.Date(day), nil
}

// MoveWorklogs is drag & drop on the timesheet: entries go to another day, after its last entry, and are locked
// so a rebuild keeps them. Exported entries stay put.
func (s *Service) MoveWorklogs(ids []int64, toDay string) error {
	if _, ok := s.Days.Parse(toDay); !ok {
		return fmt.Errorf("not a day: %q", toDay)
	}
	return s.Store.Tx(func(db *store.Store) error {
		cursor, err := s.endOfDay(db, toDay)
		if err != nil {
			return err
		}
		logs, err := db.WorklogsByID(ids)
		if err != nil {
			return err
		}
		for _, w := range logs {
			if w.Day == toDay || w.Status == core.StatusExported {
				continue
			}
			w.Day, w.Start, w.Locked = toDay, cursor, true
			cursor = w.End()
			if err := db.UpdateWorklog(w); err != nil {
				return err
			}
		}
		return nil
	})
}

// AddWorklog adds a 30-minute manual entry after the day's last one.
func (s *Service) AddWorklog(day string) (WorklogDTO, error) {
	if _, ok := s.Days.Parse(day); !ok {
		return WorklogDTO{}, fmt.Errorf("not a day: %q", day)
	}
	start, err := s.endOfDay(s.Store, day)
	if err != nil {
		return WorklogDTO{}, err
	}
	w := core.Worklog{Day: day, Start: start, Duration: 30 * 60, Source: core.SourceManual, Status: core.StatusDraft, Locked: true}
	if w.ID, err = s.Store.InsertWorklog(w); err != nil {
		return WorklogDTO{}, err
	}
	return s.worklogDTO(w), nil
}

// errSent: the entry went to Tempo through the API; change or delete it in Tempo.
var errSent = core.Err("worklog.sent")

// DeleteWorklog removes one entry. Entries sent to Tempo through the API can't be deleted here.
func (s *Service) DeleteWorklog(id int64) error { return s.DeleteWorklogs([]int64{id}) }

// DeleteWorklogs removes entries in one go; none is removed when one of them was sent to Tempo or doesn't exist.
func (s *Service) DeleteWorklogs(ids []int64) error {
	return s.Store.Tx(func(db *store.Store) error {
		logs, err := db.WorklogsByID(ids)
		if err != nil {
			return err
		}
		found := map[int64]bool{}
		for _, w := range logs {
			if w.TempoID != nil {
				return errSent
			}
			found[w.ID] = true
		}
		for _, id := range ids {
			if !found[id] {
				return fmt.Errorf("no worklog %d", id)
			}
			if err := db.DeleteWorklog(id); err != nil {
				return err
			}
		}
		return nil
	})
}

// each applies change to the entries (in one transaction); those already in Tempo are skipped unless keep says.
func (s *Service) each(ids []int64, skip func(core.Worklog) bool, change func(*core.Worklog)) error {
	return s.Store.Tx(func(db *store.Store) error {
		logs, err := db.WorklogsByID(ids)
		if err != nil {
			return err
		}
		for _, w := range logs {
			if skip(w) {
				continue
			}
			change(&w)
			w.Locked = true
			if err := db.UpdateWorklog(w); err != nil {
				return err
			}
		}
		return nil
	})
}

func exported(w core.Worklog) bool { return w.Status == core.StatusExported }

var errNeedsComment = core.Err("worklog.needsComment")

// SetStatus is Review's bulk bar. Entries already in Tempo stay. Reviewed needs every entry to have its comment
// for Tempo; none is changed when one doesn't.
func (s *Service) SetStatus(ids []int64, status string) error {
	st := core.Status(status)
	if st != core.StatusDraft && st != core.StatusReviewed && st != core.StatusExported {
		return fmt.Errorf("unknown status %q", status)
	}
	if st == core.StatusReviewed {
		logs, err := s.Store.WorklogsByID(ids)
		if err != nil {
			return err
		}
		missing := 0
		for _, w := range logs {
			if !exported(w) && w.Status != core.StatusReviewed && strings.TrimSpace(w.Comment) == "" {
				missing++
			}
		}
		if missing > 0 {
			return core.Err("worklog.needComments", "n", missing)
		}
	}
	return s.each(ids, exported, func(w *core.Worklog) { w.Status = st })
}

// SetComment gives several entries the same comment for Tempo (they lock, like any edit). Entries already in
// Tempo stay.
func (s *Service) SetComment(ids []int64, comment string) error {
	c := strings.TrimSpace(comment)
	return s.each(ids, exported, func(w *core.Worklog) { w.Comment = c })
}

// SetIssue gives several entries one ticket (they lock, like any edit). Entries already in Tempo stay.
func (s *Service) SetIssue(ids []int64, issueKey string) error {
	key := core.NormalizeKeyString(issueKey)
	if key == nil {
		return errNoIssue
	}
	return s.each(ids, exported, func(w *core.Worklog) { w.IssueKey = key })
}

// SetEntered ticks a ticket-day off after typing it into Tempo by hand: its worklogs count as in Tempo (exported
// without a Tempo id). Unticking puts them back to reviewed; entries sent by the API stay exported.
func (s *Service) SetEntered(ids []int64, entered bool) error {
	return s.each(ids, func(w core.Worklog) bool { return w.TempoID != nil && !entered }, func(w *core.Worklog) {
		if entered {
			w.Status = core.StatusExported
		} else {
			w.Status = core.StatusReviewed
		}
	})
}
