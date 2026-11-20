package app

import (
	"context"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/calendar"
	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/view"
	"github.com/molnarroland/WorkLogger/internal/web"
)

// ── Meetings and calendars ──────────────────────────────────────────────────

// Meetings is the meetings starting on the days, with the issue the rules give each, and the untagged title
// worth a rule.
func (s *Service) Meetings(from, to string) (MeetingsDTO, error) {
	out := MeetingsDTO{Meetings: []MeetingDTO{}}
	start, end, err := s.span(from, to)
	if err != nil || !s.current().Has(core.ModuleCalendar) {
		return out, err
	}
	b, err := s.builder()
	if err != nil {
		return out, err
	}
	list, err := s.Store.Meetings(start, end)
	if err != nil {
		return out, err
	}
	return meetingsDTO(meetingRows(b, list), s.Days), nil
}

func meetingRows(b core.Builder, list []core.Meeting) []view.MeetingRow {
	var rows []view.MeetingRow
	for _, m := range list {
		rows = append(rows, view.MeetingRow{Meeting: m, IssueKey: b.MeetingIssue(m), SkippedByKeyword: b.SkippedByKeyword(m)})
	}
	return rows
}

func (s *Service) SetMeetingSkipped(id string, skipped bool) error {
	return s.Store.SetMeetingSkipped(id, skipped)
}

// googleAccount is the Google Calendar sign-in for the current client id and secret.
func (s *Service) googleAccount() *calendar.Google {
	secret, _ := s.Secrets.Get(core.SecretGoogleSecret)
	id := s.current().GoogleClientID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.google == nil || s.google.ClientID != id || s.google.ClientSecret != secret {
		s.google = &calendar.Google{
			ClientID: id, ClientSecret: secret, Secrets: s.Secrets, HTTP: s.HTTP, Clock: s.Clock,
			OpenURL: s.Shell.OpenURL,
		}
	}
	return s.google
}

func (s *Service) CalendarStatus() CalendarStatus {
	return CalendarStatus{GoogleConnected: s.googleAccount().Connected(), GoogleWeb: s.googleWebReady()}
}

// ConnectGoogle signs in with OAuth (loopback + PKCE) in the system browser.
func (s *Service) ConnectGoogle() error {
	if s.Secrets.Locked() {
		return core.Err("locked.google")
	}
	return s.googleAccount().SignIn(context.Background())
}

func (s *Service) DisconnectGoogle() error { return s.googleAccount().SignOut() }

// googleProfiles are the Google profiles that were signed in at their window's last check. Without the Calendar
// API, their calendars are read from calendar.google.com with that sign-in.
func (s *Service) signedInGoogle() []core.Profile {
	var out []core.Profile
	for _, p := range s.current().ProfilesOf(core.ProfileGoogleChat) {
		if site, err := s.siteFor(p); err == nil {
			if ok := site.LastSignedIn(); ok != nil && *ok {
				out = append(out, p)
			}
		}
	}
	return out
}

func (s *Service) googleWebReady() bool { return len(s.signedInGoogle()) > 0 }

// myEmails are the addresses that are "me" in calendars: the Google profiles' and the commit emails.
func (s *Service) myEmails() []string {
	st := s.current()
	var out []string
	for _, p := range st.ProfilesOf(core.ProfileGoogleChat) {
		if p.Email != "" {
			out = append(out, p.Email)
		}
	}
	return append(out, st.AuthorEmails...)
}

func (s *Service) googleCalendarID() string {
	if id := s.current().GoogleCalendarID; id != "" {
		return id
	}
	return "primary"
}

// googleCalendarKey is the calendar id a Google profile's meetings are stored under. The first profile keeps the
// key used before profiles, so its meetings (and what was unticked) carry over.
func (s *Service) googleCalendarKey(i int, p core.Profile) string {
	if i == 0 {
		return s.googleCalendarID()
	}
	return "google:" + p.ID
}

// googleWebMeetings reads a Google profile's own calendar from calendar.google.com's export: the one named after
// its address (or the configured calendar id), else the first that's yours.
func (s *Service) googleWebMeetings(ctx context.Context, p core.Profile, key string, start, end time.Time) ([]core.Meeting, error) {
	site, err := s.siteFor(p)
	if err != nil {
		return nil, err
	}
	files, err := web.CollectGoogleCalendar(ctx, site, p.Email)
	if err != nil {
		return nil, err
	}
	var ids []string
	if p.Email != "" {
		ids = append(ids, p.Email)
	}
	if id := s.googleCalendarID(); id != "primary" {
		ids = append(ids, id)
	}
	id, text, available := web.PickCalendar(files, append(ids, s.myEmails()...)...)
	if text == "" {
		return nil, core.Err("calendar.notYours", "name", p.Title(), "calendars", strings.Join(available, ", "))
	}
	if p.Email == "" && strings.Contains(id, "@") {
		// The calendar is named after the Google account; Chat searches "my messages" by that address.
		err := s.update(func(st *core.Settings) {
			for i := range st.Profiles {
				if st.Profiles[i].ID == p.ID && st.Profiles[i].Email == "" {
					st.Profiles[i].Email = id
				}
			}
			st.SyncProfiles()
		})
		s.logFailure("calendar", "Saving the Google address of "+p.Title()+" failed", err)
	}
	return calendar.ParseICS(text, key, start, end, s.Days.Loc, s.myEmails()), nil
}

// fetchMeetings reads the range from Google Calendar (the API when connected, else calendar.google.com for every
// signed-in Google profile) and the ICS feeds into the database. Events of calendars no longer read are removed.
// Returns how many events came in.
func (s *Service) fetchMeetings(ctx context.Context, start, end time.Time) (int, error) {
	st := s.current()
	g := s.googleAccount()
	api := g.Connected()
	var viaWeb []core.Profile
	if !api {
		viaWeb = s.signedInGoogle()
	}
	googleProfiles := st.ProfilesOf(core.ProfileGoogleChat)
	keyOf := func(p core.Profile) string {
		for i, q := range googleProfiles {
			if q.ID == p.ID {
				return s.googleCalendarKey(i, p)
			}
		}
		return "google:" + p.ID
	}
	active := map[string]bool{}
	if api {
		active[s.googleCalendarID()] = true
	}
	for _, p := range viaWeb {
		active[keyOf(p)] = true
	}
	google := map[string]bool{}
	for k := range active {
		google[k] = true
	}
	for _, u := range st.IcsURLs {
		if strings.TrimSpace(u) != "" {
			active[u] = true
		}
	}
	if err := s.Store.WithContext(ctx).SyncMeetings(start, end, map[string]bool{}, active, nil); err != nil {
		return 0, err
	}
	n := 0
	var failed []string
	sync := func(calID string, events []core.Meeting, err error) error {
		if err != nil {
			failed = append(failed, err.Error())
			return nil
		}
		n += len(events)
		return s.Store.WithContext(ctx).SyncMeetings(start, end, map[string]bool{calID: true}, active, events)
	}
	if api {
		id := s.googleCalendarID()
		events, err := g.Meetings(ctx, id, start, end)
		if err := sync(id, events, err); err != nil {
			return n, err
		}
	}
	for _, p := range viaWeb {
		key := keyOf(p)
		events, err := s.googleWebMeetings(ctx, p, key, start, end)
		if err := sync(key, events, err); err != nil {
			return n, err
		}
	}
	for u := range active {
		if google[u] {
			continue
		}
		events, err := calendar.FetchICS(ctx, s.HTTP, u, start, end, s.Days.Loc, s.myEmails())
		if err := sync(u, events, err); err != nil {
			return n, err
		}
	}
	if len(failed) > 0 {
		return n, core.Err("calendar.failed", "reason", strings.Join(failed, "; "))
	}
	return n, nil
}

func (s *Service) canReadMeetings() bool {
	st := s.current()
	if !st.Has(core.ModuleCalendar) {
		return false
	}
	for _, u := range st.IcsURLs {
		if strings.TrimSpace(u) != "" {
			return true
		}
	}
	return s.googleAccount().Connected() || s.googleWebReady()
}
