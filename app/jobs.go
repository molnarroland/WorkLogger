package app

import (
	"context"
	"errors"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/web"
)

var errBusy = core.Err("busy")

// startJob starts the one long job allowed at a time; nil ctx when one is running. done must be called.
func (s *Service) startJob() (context.Context, func()) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if s.cancel != nil {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	return ctx, func() {
		cancel()
		s.jobMu.Lock()
		s.cancel = nil
		s.jobMu.Unlock()
	}
}

// Cancel stops the running collect, fetch or push.
func (s *Service) Cancel() {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Service) progress(source string, m core.Message) {
	s.logActivity(source, "progress", m.Text)
	s.emit(EventProgress, CollectProgress{Source: source, Message: m})
}

// finish emits collect:done with the job's result (or its error's message) and passes err on.
func (s *Service) finish(ctx context.Context, m core.Message, err error) error {
	return s.finishSettled(ctx, m, err, 0)
}

// finishSettled is finish for a Collect that left out settled days (CollectDone.Settled).
func (s *Service) finishSettled(ctx context.Context, m core.Message, err error, settled int) error {
	if err != nil && m.Text == "" {
		m = core.MessageOf(err)
	}
	level := "info"
	if err != nil {
		level = "error"
	}
	if m.Text != "" {
		s.logActivity("job", level, m.Text)
	}
	s.emit(EventDone, CollectDone{Cancelled: ctx.Err() != nil, Failed: err != nil, Message: m, Settled: settled})
	return err
}

// Collect reads git history, the calendars and HRmaster's hours that aren't final yet (hrmasterGaps) for the days, then rebuilds the drafts. It emits collect:progress
// while it runs and collect:done when it ends (finished or cancelled). Unless forced, it leaves out the weeks
// Tempo already has in full (settledDays): the range shrinks to the days around them, and collect:done counts
// what was left out.
func (s *Service) Collect(from, to string, force bool) error {
	if _, _, err := s.span(from, to); err != nil {
		return err
	}
	from, to = min(from, to), max(from, to)
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()

	settled := 0
	if !force {
		days, err := s.settledDays(from, to)
		if err != nil {
			return s.finish(ctx, core.Message{}, err)
		}
		all := s.Days.Range(from, to)
		f, t, ok := unsettled(all, days)
		if !ok {
			return s.finishSettled(ctx, core.Say("collect.allSettled", "from", from, "to", to), nil, len(all))
		}
		settled = len(all) - len(s.Days.Range(f, t))
		from, to = f, t
	}
	start, end, _ := s.span(from, to)

	st := s.current()
	useGit := st.Has(core.ModuleGit)
	commits, meetings := 0, 0
	var skipped []string
	if useGit {
		authors := st.AuthorEmails
		if len(authors) == 0 {
			if email := s.Git.GlobalUserEmail(ctx); email != "" {
				authors = []string{email}
				s.logFailure("git", "Saving your git email failed", s.update(func(x *core.Settings) { x.AuthorEmails = authors }))
			}
		}
		if len(authors) == 0 {
			return s.finish(ctx, core.Message{}, core.Err("collect.needEmail"))
		}
		all, err := s.Store.Repos()
		if err != nil {
			return s.finish(ctx, core.Message{}, err)
		}
		var enabled []core.Repo
		for _, r := range all {
			if r.Enabled {
				enabled = append(enabled, r)
			}
		}
		s.progress("git", core.Say("progress.git", "n", len(enabled)))
		results, errs := s.Git.History(ctx, enabled, start, end, authors)
		skipped = append(skipped, errs...)
		err = s.Store.WithContext(ctx).Tx(func(db *store.Store) error {
			for _, r := range results {
				list := make([]core.StoredCommit, len(r.Commits))
				for i, c := range r.Commits {
					list[i] = core.NewStoredCommit(c, r.RepoID, st.IgnoredPathPatterns)
				}
				commits += len(list)
				if err := db.SaveCommits(list); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return s.finish(ctx, core.Message{}, err)
		}
	}
	readMeetings := s.canReadMeetings()
	if readMeetings && ctx.Err() == nil {
		s.progress("calendar", core.Say("progress.calendar"))
		n, err := s.fetchMeetings(ctx, start, end)
		meetings = n
		if err != nil {
			skipped = append(skipped, err.Error())
		}
	}
	// HRmaster's hours for the days whose hours weren't final when last read (today's always).
	if profiles := st.ProfilesOf(core.ProfileHRmaster); st.Has(core.ModuleWorkTimes) && len(profiles) > 0 && ctx.Err() == nil {
		months, err := s.hrmasterGaps(from, to)
		if err != nil {
			return s.finish(ctx, core.Message{}, err)
		}
		if len(months) > 0 {
			_, problems, err := s.importHrmaster(ctx, profiles, months)
			skipped = append(skipped, problems...)
			if err != nil && len(problems) == 0 {
				skipped = append(skipped, err.Error())
			}
		}
	}
	jiraTickets := 0
	if st.Has(core.ModuleJira) && ctx.Err() == nil {
		n, problems := s.collectJira(ctx, from, to)
		jiraTickets = n
		skipped = append(skipped, problems...)
	}
	pages := 0
	if st.Has(core.ModuleConfluence) && ctx.Err() == nil {
		n, problems := s.collectConfluence(ctx, from, to)
		pages = n
		skipped = append(skipped, problems...)
	}
	threads := 0
	if st.Has(core.ModuleEmail) && ctx.Err() == nil {
		n, problems := s.collectEmail(ctx, from, to)
		threads = n
		skipped = append(skipped, problems...)
	}
	if ctx.Err() != nil {
		return s.finish(ctx, core.Say("job.cancelled"), nil)
	}
	s.progress("drafts", core.Say("progress.drafts"))
	if err := s.rebuild(ctx, from, to); err != nil {
		return s.finish(ctx, core.Message{}, err)
	}
	var msg core.Message
	switch {
	case useGit && readMeetings:
		msg = core.Say("collect.done", "commits", commits, "meetings", meetings, "from", from, "to", to)
	case useGit:
		msg = core.Say("collect.git", "commits", commits, "from", from, "to", to)
	case readMeetings && jiraTickets > 0:
		msg = core.Say("collect.meetingsJira", "meetings", meetings, "tickets", jiraTickets, "from", from, "to", to)
	case readMeetings:
		msg = core.Say("collect.meetings", "meetings", meetings, "from", from, "to", to)
	case jiraTickets > 0:
		msg = core.Say("collect.jira", "tickets", jiraTickets, "from", from, "to", to)
	case pages > 0:
		msg = core.Say("collect.confluence", "pages", pages, "from", from, "to", to)
	case threads > 0:
		msg = core.Say("collect.email", "threads", threads, "from", from, "to", to)
	default:
		msg = core.Say("collect.nothing", "from", from, "to", to)
	}
	return s.finishSettled(ctx, msg.WithSkipped(skipped), nil, settled)
}

// Rebuild rebuilds unlocked drafts from stored commits, meetings and sessions (no network or git access). It is a
// job like the others: not while one runs, and it ends with collect:done.
func (s *Service) Rebuild(from, to string) error {
	if _, _, err := s.span(from, to); err != nil {
		return err
	}
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()
	if err := s.rebuild(ctx, from, to); err != nil {
		return s.finish(ctx, core.Message{}, err)
	}
	return s.finish(ctx, core.Say("rebuild.done", "from", min(from, to), "to", max(from, to)), nil)
}

func (s *Service) rebuild(ctx context.Context, from, to string) error {
	start, end, err := s.span(from, to)
	if err != nil {
		return err
	}
	b, err := s.builder()
	if err != nil {
		return err
	}
	if err := s.Store.WithContext(ctx).RebuildDrafts(b, s.Days.Key(start), s.Days.Shift(s.Days.Key(end), -1)); err != nil {
		return err
	}
	return s.applyChatRules(start, end)
}

// signInNeeded opens a profile's window for signing in after a fetch found it signed out. ctx is the job's, so
// cancelling the job stops waiting for the window.
func (s *Service) signInNeeded(ctx context.Context, p core.Profile) error {
	if site, err := s.siteFor(p); err == nil {
		if err := site.SignIn(ctx); err != nil {
			s.logActivity(p.ID, "error", "Could not open the sign-in window: "+err.Error())
		}
	}
	return core.Err("site.signedOut", "name", p.Title())
}

// FetchHrmaster reads every HRmaster profile's work-time calendar for each month of the range and imports them
// merged: each day from the profile that has something on it.
func (s *Service) FetchHrmaster(from, to string) error {
	if _, _, err := s.span(from, to); err != nil {
		return err
	}
	profiles := s.current().ProfilesOf(core.ProfileHRmaster)
	if len(profiles) == 0 {
		return core.Err("hrmaster.needProfile")
	}
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()
	days, problems, err := s.importHrmaster(ctx, profiles, web.Months(s.Days, from, to))
	switch {
	case ctx.Err() != nil:
		return s.finish(ctx, core.Say("job.cancelled"), nil)
	case err != nil:
		return s.finish(ctx, core.Message{}, err)
	}
	return s.finish(ctx, core.Say("hrmaster.imported", "n", days).WithSkipped(problems), nil)
}

// hrmasterGaps is the months Collect reads from HRmaster: those with a day up to today whose hours aren't final
// yet. Rest days, leave and holidays are final as soon as they are read; any other day is final once it was read
// after it ended, so a month read in the middle of a day (or before the day happened) is read again, and today is
// always read. Records from before the read day was kept are read once more; sessions without a record count as
// read.
func (s *Service) hrmasterGaps(from, to string) ([]web.Month, error) {
	to = min(to, s.today())
	if from > to {
		return nil, nil
	}
	sessions, err := s.Store.SessionsRange(from, to)
	if err != nil {
		return nil, err
	}
	records, err := s.Store.DayRecordMap(from, to)
	if err != nil {
		return nil, err
	}
	hasSessions := map[string]bool{}
	for _, w := range sessions {
		hasSessions[w.Day] = true
	}
	var out []web.Month
	for _, day := range s.Days.Range(from, to) {
		if hrmasterFinal(day, records[day], hasSessions[day]) {
			continue
		}
		d := s.Days.Date(day)
		if m := (web.Month{Year: d.Year(), Month: int(d.Month())}); len(out) == 0 || out[len(out)-1] != m {
			out = append(out, m)
		}
	}
	return out, nil
}

// hrmasterFinal: HRmaster's record of the day won't change any more (hrmasterGaps).
func hrmasterFinal(day string, r *core.DayRecord, hasSessions bool) bool {
	switch {
	case r == nil:
		return hasSessions
	case r.Status == core.DayRestDay || r.Status.IsAbsence():
		return true
	default:
		return r.ReadOn > day
	}
}

// importHrmaster reads the months from every HRmaster profile and imports them merged: each day from the profile
// that has something on it. It returns the days imported and what was skipped; err when nothing could be read
// (problems say why) or the import failed.
func (s *Service) importHrmaster(ctx context.Context, profiles []core.Profile, months []web.Month) (int, []string, error) {
	var sheets []core.ParsedTimesheet
	var problems []string
	for _, p := range profiles {
		if ctx.Err() != nil {
			break
		}
		base, ok := web.NormalizeHRmaster(p.URL)
		if !ok {
			problems = append(problems, p.Title()+": the HRmaster address is not set")
			continue
		}
		site, err := s.siteFor(p)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		prefix := ""
		if len(profiles) > 1 {
			prefix = p.Title() + " · "
		}
		pages, err := web.CollectHRmaster(ctx, site, base, months, func(m string) { s.progress("hrmaster", core.Message{Text: prefix + m}) })
		switch {
		case web.IsNotSignedIn(err):
			problems = append(problems, s.signInNeeded(ctx, p).Error())
			continue
		case err != nil:
			problems = append(problems, p.Title()+": "+err.Error())
			continue
		}
		for _, pg := range pages {
			sheets = append(sheets, core.HRmasterParser{FallbackYear: pg.Month.Year, FallbackMonth: pg.Month.Month, Days: s.Days}.Parse(pg.HTML))
		}
	}
	if ctx.Err() != nil {
		return 0, problems, nil
	}
	if len(sheets) == 0 {
		return 0, problems, errors.New(strings.Join(problems, " "))
	}
	merged := core.MergeTimesheets(sheets)
	today := s.today()
	for i := range merged.Days {
		merged.Days[i].ReadOn = today
	}
	if err := s.Store.WithContext(ctx).ImportTimesheet(merged); err != nil {
		return 0, problems, err
	}
	return len(merged.DayKeys()), problems, nil
}

// FetchGoogleChat reads your Google Chat messages for the range, up to today, with every Google profile, and
// imports them.
func (s *Service) FetchGoogleChat(from, to string) error {
	if _, _, err := s.span(from, to); err != nil {
		return err
	}
	if from > to {
		from, to = to, from
	}
	// Days that haven't come yet have no messages: stop at today.
	if today := s.today(); to > today {
		if from > today {
			return core.Err("chat.future")
		}
		to = today
	}
	st := s.current()
	profiles := st.ProfilesOf(core.ProfileGoogleChat)
	if len(profiles) == 0 {
		return core.Err("google.needProfile")
	}
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()
	added, convs, read := 0, map[string]bool{}, 0
	var problems []string
	for _, p := range profiles {
		if ctx.Err() != nil {
			break
		}
		email := p.Email
		if email == "" && len(profiles) == 1 && len(st.AuthorEmails) > 0 {
			email = st.AuthorEmails[0]
		}
		if email == "" {
			problems = append(problems, p.Title()+": enter its Google address in Settings → Profiles")
			continue
		}
		site, err := s.siteFor(p)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		prefix := ""
		if len(profiles) > 1 {
			prefix = p.Title() + " · "
		}
		err = web.CollectGoogleChat(ctx, site, s.Days, email, p.Email, from, to, func(m string) { s.progress("googleChat", core.Message{Text: prefix + m}) },
			func(day string, e core.ChatExport) error {
				n, err := s.importChats(e)
				added += n
				for _, c := range e.Conversations {
					convs[c.ID] = true
				}
				s.emit(EventChatDay, day) // the Chats page shows the day while the next one is read
				return err
			})
		switch {
		case web.IsNotSignedIn(err):
			problems = append(problems, s.signInNeeded(ctx, p).Error())
		case ctx.Err() != nil:
		case err != nil:
			problems = append(problems, p.Title()+": "+err.Error())
		default:
			read++
		}
	}
	switch {
	case ctx.Err() != nil:
		return s.finish(ctx, core.Say("chat.fetchCancelled", "messages", added, "conversations", len(convs)), nil)
	case read == 0:
		return s.finish(ctx, core.Message{}, errors.New(strings.Join(problems, " ")))
	}
	return s.finish(ctx, core.Say("chat.fetched", "messages", added, "conversations", len(convs)).WithSkipped(problems), nil)
}

// FetchGoogleCalendar reads your Google Calendars for the range from calendar.google.com, signed in through the
// Google profiles (no API keys), plus the ICS feeds. Rebuild drafts to apply them.
func (s *Service) FetchGoogleCalendar(from, to string) error {
	start, end, err := s.span(from, to)
	if err != nil {
		return err
	}
	ctx, done := s.startJob()
	if ctx == nil {
		return errBusy
	}
	defer done()
	if !s.googleAccount().Connected() && !s.googleWebReady() {
		// Not checked yet, or signed out: ask the pages, and open a window for sign-in when none is.
		profiles := s.current().ProfilesOf(core.ProfileGoogleChat)
		if len(profiles) == 0 {
			return s.finish(ctx, core.Message{}, core.Err("calendar.needGoogle"))
		}
		signed := false
		for _, p := range profiles {
			if site, err := s.siteFor(p); err == nil && site.IsSignedIn(ctx) {
				signed = true
			}
		}
		if !signed {
			return s.finish(ctx, core.Message{}, s.signInNeeded(ctx, profiles[0]))
		}
	}
	s.progress("calendar", core.Say("progress.googleCalendar"))
	n, err := s.fetchMeetings(ctx, start, end)
	if err != nil {
		return s.finish(ctx, core.Message{}, err)
	}
	return s.finish(ctx, core.Say("calendar.read", "n", n), nil)
}
