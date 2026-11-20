package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/imap"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/tempo"
)

// ── Email ───────────────────────────────────────────────────────────────────
//
// Only the Date and Subject headers of the Sent folder are read, over TLS, and the mailbox is opened read-only.

func (s *Service) emailProfile(id string) (core.Profile, error) {
	if p, ok := s.current().Profile(id); ok && p.Kind == core.ProfileEmail {
		return p, nil
	}
	return core.Profile{}, fmt.Errorf("no Email profile %q", id)
}

// signInEmail opens a signed-in IMAP session for an Email profile, logged in the activity console.
func (s *Service) signInEmail(ctx context.Context, p core.Profile) (*imap.Session, error) {
	if strings.TrimSpace(p.URL) == "" || strings.TrimSpace(p.Email) == "" {
		return nil, core.Err("email.needServer")
	}
	password, _ := s.Secrets.Get(p.EmailPasswordKey())
	if password == "" {
		return nil, core.Err("email.needPassword")
	}
	dial := s.DialIMAP
	if dial == nil {
		dial = imap.DialTLS
	}
	start := s.Clock.Now()
	sess, err := imap.Login(ctx, dial, p.URL, p.Email, password)
	line := "IMAP sign-in " + p.Email + " at " + imap.Address(p.URL)
	if err != nil {
		s.logActivity("email", "error", fmt.Sprintf("%s → %v (%s)", line, err, s.since(start)))
		return nil, err
	}
	s.logActivity("email", "info", fmt.Sprintf("%s → OK (%s)", line, s.since(start)))
	return sess, nil
}

// readSent reads the mail you sent over the days (inclusive) with every Email profile: what it found, how many
// profiles there are, and what failed ("profile: reason").
func (s *Service) readSent(ctx context.Context, from, to string) (list []store.SentEmail, profiles int, failed []string) {
	all := s.current().ProfilesOf(core.ProfileEmail)
	start, end, _ := s.span(from, to)
	for _, p := range all {
		got, err := func() ([]imap.Sent, error) {
			sess, err := s.signInEmail(ctx, p)
			if err != nil {
				return nil, err
			}
			defer sess.Close()
			return sess.SentBetween(start, end)
		}()
		if err != nil {
			s.logFailure("email", "Reading "+p.Title()+"'s sent mail failed", err)
			failed = append(failed, p.Title()+": "+err.Error())
			continue
		}
		s.logActivity("email", "info", fmt.Sprintf("%s: %d sent mails", p.Title(), len(got)))
		for _, m := range got {
			list = append(list, store.SentEmail{ID: m.ID, Subject: m.Subject, At: m.At})
		}
	}
	return list, len(all), failed
}

// collectEmail stores the mail you sent for the days (the Email module's part of Collect): how many thread-days
// it found, and what it skipped.
func (s *Service) collectEmail(ctx context.Context, from, to string) (int, []string) {
	if s.Secrets.Locked() {
		return 0, nil
	}
	s.progress("email", core.Say("progress.email"))
	list, profiles, failed := s.readSent(ctx, from, to)
	if profiles == 0 {
		return 0, []string{core.Say("email.needProfile").Text}
	}
	if len(failed) == profiles {
		return 0, failed
	}
	start, end, _ := s.span(from, to)
	threads := map[string]bool{}
	for _, m := range list {
		threads[strings.ToLower(core.ThreadSubject(m.Subject))+"|"+s.Days.Key(m.At)] = true
	}
	if err := s.Store.WithContext(ctx).SyncSentEmails(start, end, list, len(failed) == 0); err != nil {
		return 0, append(failed, err.Error())
	}
	return len(threads), failed
}

// CheckEmail signs in to an Email profile's server and finds its Sent folder.
func (s *Service) CheckEmail(profileID string) ([]tempo.Line, error) {
	p, err := s.emailProfile(profileID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	signIn, sent := "Server, user name and password", "Sent folder"
	no := false
	sess, err := s.signInEmail(ctx, p)
	if err != nil {
		detail := err.Error()
		var refused imap.ErrSignIn
		if errors.As(err, &refused) {
			detail = "The server refused the user name or password. Gmail, iCloud and Yahoo need an app password; " +
				"Microsoft 365 accepts only its own sign-in, not a password. (" + refused.Reason + ")"
		} else if ue := (*core.UserError)(nil); errors.As(err, &ue) {
			detail = core.MessageOf(err).Text
		}
		return []tempo.Line{{Title: signIn, Detail: detail, OK: &no}, {Title: sent}}, nil
	}
	defer sess.Close()
	yes := true
	lines := []tempo.Line{{Title: signIn, Detail: "Signed in to " + imap.Address(p.URL) + ".", OK: &yes}}
	if name, err := sess.FindSent(); err != nil {
		return append(lines, tempo.Line{Title: sent, Detail: err.Error(), OK: &no}), nil
	} else {
		return append(lines, tempo.Line{Title: sent, Detail: "Reading \"" + name + "\" (read-only).", OK: &yes}), nil
	}
}

// ── Email page ──────────────────────────────────────────────────────────────

// EmailThreadDTO is the mail you sent in one day on one subject.
type EmailThreadDTO struct {
	Subject string `json:"subject"`
	// Times is when you sent each ("15:04"); Subjects their own subjects (Re:, Fwd: and all), in the same order.
	Times    []string `json:"times"`
	Subjects []string `json:"subjects"`
	// IssueKey is a ticket key in the subject (its time goes to that ticket), else "".
	IssueKey string `json:"issueKey"`
	// Entry is the draft or entry the thread-day became (its suggested time), else nil.
	Entry *PlanEntryDTO `json:"entry"`
}

type EmailDayDTO struct {
	Day     string           `json:"day"`
	Threads []EmailThreadDTO `json:"threads"`
}

// EmailsDTO is the collected sent mail by day (newest first) and thread, how many mails and thread-days, and how
// many Email profiles there are (0: none set up).
type EmailsDTO struct {
	Days     []EmailDayDTO `json:"days"`
	Mails    int           `json:"mails"`
	Threads  int           `json:"threads"`
	Profiles int           `json:"profiles"`
}

// SentEmails is the sent mail Collect stored for the days (inclusive), by day and thread, each with the entry it
// became. Local data only: Collect reads the mailbox.
func (s *Service) SentEmails(from, to string) (EmailsDTO, error) {
	out := EmailsDTO{Days: []EmailDayDTO{}, Profiles: len(s.current().ProfilesOf(core.ProfileEmail))}
	start, end, err := s.span(from, to)
	if err != nil {
		return out, err
	}
	list, err := s.Store.SentEmails(start, end)
	if err != nil {
		return out, err
	}
	logs, err := s.Store.Worklogs(s.Days.Key(start), s.Days.Shift(s.Days.Key(end), -1))
	if err != nil {
		return out, err
	}
	b, err := s.builder()
	if err != nil {
		return out, err
	}
	known := b.KnownPrefixes()
	entries := map[string]*PlanEntryDTO{}
	for _, w := range logs {
		for _, r := range w.Refs() {
			if strings.HasPrefix(r, "email:") {
				entries[r] = &planEntries([]core.Worklog{w})[0]
			}
		}
	}
	type thread struct {
		dto EmailThreadDTO
		ref core.EmailThread
	}
	byDay := map[string][]*thread{}
	index := map[string]*thread{}
	for _, m := range list {
		at := m.At.In(s.Days.Loc)
		day := s.Days.Key(at)
		subject := core.ThreadSubject(m.Subject)
		id := day + "|" + strings.ToLower(subject)
		t := index[id]
		if t == nil {
			t = &thread{dto: EmailThreadDTO{Subject: subject, Times: []string{}, Subjects: []string{}}, ref: core.EmailThread{Day: day, Subject: subject}}
			if k, ok := core.FindKey(subject, known); ok {
				t.dto.IssueKey = k
			}
			index[id] = t
			byDay[day] = append(byDay[day], t)
			out.Threads++
		}
		t.dto.Times = append(t.dto.Times, at.Format("15:04"))
		t.dto.Subjects = append(t.dto.Subjects, m.Subject)
		out.Mails++
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	for _, d := range days {
		dd := EmailDayDTO{Day: d, Threads: []EmailThreadDTO{}}
		list := byDay[d]
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].dto.Times[len(list[i].dto.Times)-1] > list[j].dto.Times[len(list[j].dto.Times)-1]
		})
		for _, t := range list {
			t.dto.Entry = entries[t.ref.Ref()]
			dd.Threads = append(dd.Threads, t.dto)
		}
		out.Days = append(out.Days, dd)
	}
	return out, nil
}
