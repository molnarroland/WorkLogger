// Package app is the Wails-facing API: thin methods that validate, call core/store and emit events. One struct
// backs the whole frontend Api (frontend/src/app/api/api.ts), split over files by area.
package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/molnarroland/WorkLogger/internal/calendar"
	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
	"github.com/molnarroland/WorkLogger/internal/imap"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/tempo"
	"github.com/molnarroland/WorkLogger/internal/view"
)

// Keyring is the token store that starts locked (platform.Keyring).
type Keyring interface {
	core.SecretStore
	Unlock() bool
	Locked() bool
}

// Site is a signed-in website in its own browser profile (platform.Chrome).
type Site interface {
	core.PageDriver
	Show(ctx context.Context, url string) error
	SignIn(ctx context.Context) error // shows the site's login page
	LastSignedIn() *bool
	SignOut(ctx context.Context) error
}

// SiteSpec describes a profile's site window: which page proves the sign-in, where signing in starts, and how
// to tell a signed-in page by its address.
type SiteSpec struct {
	ID         string           // the profile id: names the window and the activity lines
	Kind       core.ProfileKind // what the profile connects to
	Name       string           // the profile's title, in messages
	Home       func() string    // the page that proves a sign-in, "" when not configured
	SignInURL  func() string    // where Sign in starts; Home when nil
	SignedInAt func(u string) bool
	StateFile  string // remembers the last sign-in check
}

// ReportRefused puts a refused runtime request (internal/webview GuardRuntime) in the activity console. A plain
// function, not a method, so Wails doesn't bind it for the frontend.
func ReportRefused(s *Service, message string) { s.logActivity("security", "error", message) }

// Shell is what the window system provides; main.go fills it from Wails.
type Shell struct {
	Emit         func(name string, data any)
	OpenURL      func(url string) error
	ChooseFolder func() (string, error)                 // "" when cancelled
	SaveFile     func(suggested string) (string, error) // "" when cancelled
	Reveal       func(path string) error
}

type Deps struct {
	Store   *store.Store
	Days    core.Days
	Clock   core.Clock
	Secrets Keyring
	HTTP    core.HTTPDoer
	Git     git.Git
	Shell   Shell
	// NewSite makes a profile's site window (internal/webview); StateDir keeps their last sign-in check.
	NewSite  func(SiteSpec) Site
	StateDir string
	// Sites replaces a profile's site window, by profile id (tests).
	Sites map[string]Site
	// DialIMAP connects to a mail server (tests); nil is TLS.
	DialIMAP imap.Dialer
}

type Service struct {
	Deps

	mu       sync.Mutex
	settings core.Settings
	google   *calendar.Google
	webSites map[string]Site // a window per signed-in profile, by profile id

	jobMu  sync.Mutex
	cancel context.CancelFunc // the running job, nil when idle

	actMu    sync.Mutex
	activity []ActivityLine // the console (activity.go)

	tempoMu    sync.Mutex
	tempoCache map[string][]tempo.Worklog // read-back by day (RefreshTempo)
	planCache  map[string][]tempo.Plan    // Tempo Planner's plans by day (RefreshTempo)
}

func New(d Deps) *Service {
	s := &Service{Deps: d, tempoCache: map[string][]tempo.Worklog{}, planCache: map[string][]tempo.Plan{}}
	// Everything that goes out in the background shows in the activity console.
	if s.Git.Run != nil {
		s.Git.Run = loggedRunner{s.Git.Run, s}
	}
	if s.HTTP != nil {
		s.HTTP = loggedHTTP{s.HTTP, s}
	}
	s.settings = d.Store.LoadSettings(NewID)
	return s
}

// NewID is a random UUID.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails: since Go 1.24 crypto/rand.Read crashes the program instead of returning an error
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func rfc(t time.Time) string { return t.Format(time.RFC3339) }

// ── Helpers ─────────────────────────────────────────────────────────────────

func (s *Service) current() core.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// update changes the settings and saves them.
func (s *Service) update(change func(*core.Settings)) error {
	return s.updateOr(func(st *core.Settings) error {
		change(st)
		return nil
	})
}

// updateOr is update for a change that may refuse: on an error nothing is saved or changed.
func (s *Service) updateOr(change func(*core.Settings) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.settings.Clone() // the change edits a copy (C7)
	if err := change(&next); err != nil {
		return err
	}
	next.FillNil()
	next.FixDefaultJiraAccount()
	if err := s.Store.SaveSettings(next); err != nil {
		return err
	}
	s.settings = next
	return nil
}

func (s *Service) builder() (core.Builder, error) {
	b := core.Builder{Settings: s.current(), Repos: map[int64]core.Repo{}, Projects: map[string]core.ProjectMapping{}, Days: s.Days}
	repos, err := s.Store.Repos()
	if err != nil {
		return b, err
	}
	projects, err := s.Store.Projects()
	if err != nil {
		return b, err
	}
	for _, r := range repos {
		b.Repos[r.ID] = r
	}
	for _, p := range projects {
		b.Projects[p.Key] = p
	}
	return b, nil
}

// span parses an inclusive day range and returns [start, end).
func (s *Service) span(fromDay, toDay string) (time.Time, time.Time, error) {
	f, ok1 := s.Days.Parse(fromDay)
	t, ok2 := s.Days.Parse(toDay)
	if !ok1 || !ok2 {
		return time.Time{}, time.Time{}, fmt.Errorf("not a day range: %q – %q", fromDay, toDay)
	}
	if t.Before(f) {
		f, t = t, f
	}
	return f, s.Days.DayAfter(t), nil
}

func (s *Service) emit(name string, data any) {
	if s.Shell.Emit != nil {
		s.Shell.Emit(name, data)
	}
}

// ── Lock ────────────────────────────────────────────────────────────────────

// Unlock reads the keyring item. False when it couldn't be read.
func (s *Service) Unlock() bool {
	if !s.Secrets.Unlock() {
		return false
	}
	s.moveLegacyTokens()
	return true
}

func (s *Service) IsLocked() bool { return s.Secrets.Locked() }

// moveLegacyTokens: before accounts, the tokens were stored once; they now belong to the default account.
func (s *Service) moveLegacyTokens() {
	a := s.current().DefaultJiraAccount()
	if a == nil {
		return
	}
	for old, key := range map[string]string{core.SecretLegacyJira: a.JiraTokenKey(), core.SecretLegacyTempo: a.TempoTokenKey()} {
		token, ok := s.Secrets.Get(old)
		if !ok {
			continue
		}
		if _, has := s.Secrets.Get(key); !has {
			s.logFailure("keyring", "Moving a saved token failed", s.Secrets.Set(key, token))
		}
		s.logFailure("keyring", "Removing an old token failed", s.Secrets.Set(old, ""))
	}
}

// ── Settings ────────────────────────────────────────────────────────────────

func (s *Service) Settings() core.Settings { return s.current() }

// SetupSteps is what the chosen modules still need, in the order to do it (the setup guide's last page and
// Settings → Profiles).
func (s *Service) SetupSteps() []SetupStepDTO { return setupDTOs(view.SetupSteps(s.current())) }

// SaveSettings stores the settings. Keys the frontend doesn't know (knownModules, macCalendarIds) are kept.
func (s *Service) SaveSettings(in core.Settings) (core.Settings, error) {
	old := s.current()
	for _, p := range in.Periods {
		if p.From > p.To {
			return old, core.Err("period.backwards", "title", p.Title)
		}
	}
	err := s.update(func(st *core.Settings) {
		known, mac := st.KnownModules, st.MacCalendarIDs
		*st = in
		st.KnownModules, st.MacCalendarIDs = known, mac
		st.FillNil()
		for i, p := range st.Profiles {
			if p.ID == "" {
				st.Profiles[i].ID = NewID()
			}
		}
		st.SyncProfiles()
		st.Normalize()
	})
	if err != nil {
		return old, err
	}
	if fmt.Sprint(old.ExcludedFolders) != fmt.Sprint(in.ExcludedFolders) {
		s.logFailure("git", "Removing the repositories in skipped folders failed", s.removeHiddenRepos())
	}
	return s.current(), nil
}

func (s *Service) DatabasePath() string { return s.Store.Path }

// ChooseFolder is the native folder picker; nil when cancelled.
func (s *Service) ChooseFolder() (*string, error) {
	p, err := s.Shell.ChooseFolder()
	if err != nil || p == "" {
		return nil, err
	}
	return &p, nil
}

func (s *Service) RevealPath(path string) error { return s.Shell.Reveal(git.ExpandHome(path)) }
