package core

import (
	"encoding/json"
	"slices"
	"strings"
)

type RoundingMode string

// RoundingSessions behaves like fixedHours and is kept only so older saved settings still decode.
const (
	RoundingEstimate   RoundingMode = "estimate"
	RoundingFixedHours RoundingMode = "fixedHours"
	RoundingSessions   RoundingMode = "sessions"
)

type Module string

const (
	ModuleGit       Module = "git"
	ModuleCalendar  Module = "calendar"
	ModuleWorkTimes Module = "workTimes"
	ModuleChats     Module = "chats"
	// ModuleJira: what you did on Jira tickets (created, changed, commented) becomes drafts, for those who plan
	// and refine work rather than commit code.
	ModuleJira Module = "jira"
	// ModuleConfluence: pages you write or comment on in Confluence become drafts.
	ModuleConfluence Module = "confluence"
	// ModuleEmail: the mail you send (subjects and times, read over IMAP) becomes drafts.
	ModuleEmail Module = "email"
)

var AllModules = []Module{ModuleGit, ModuleCalendar, ModuleWorkTimes, ModuleChats, ModuleJira, ModuleConfluence, ModuleEmail}

// optInModules aren't switched on by an update (addNewModules) or for a new install: they change drafts for
// people who don't need them, so the first-run questions or Settings switch them on.
var optInModules = map[Module]bool{ModuleJira: true, ModuleConfluence: true, ModuleEmail: true}

type MeetingRule struct {
	ID       string `json:"id"`
	Contains string `json:"contains"`
	IssueKey string `json:"issueKey"`
}

// ChatRule: a conversation always logged to the same ticket (Chats → "Always").
type ChatRule struct {
	ConversationID string `json:"conversationId"`
	Name           string `json:"name"`
	IssueKey       string `json:"issueKey"`
}

// ProjectPeriod is a stretch of days booked to one issue. Day keys, inclusive.
type ProjectPeriod struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	IssueKey string `json:"issueKey"`
	From     string `json:"from"`
	To       string `json:"to"`
	// Comment goes to Tempo with the time the period fills in, so those entries can be reviewed as they are.
	Comment string `json:"comment"`
}

func (p ProjectPeriod) Contains(day string) bool { return p.From <= day && day <= p.To }

type WorklogProvider string

const (
	ProviderTempo WorklogProvider = "tempo" // Tempo Timesheets (Tempo REST API)
	ProviderJira  WorklogProvider = "jira"  // Jira's own worklogs
	ProviderOther WorklogProvider = "other" // another tool: only files and checklists
)

// JiraAccount is one Jira site with its own sign-in and worklog provider. Tokens live in the keyring.
type JiraAccount struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	BaseURL      string          `json:"baseURL"`
	Email        string          `json:"email"`
	AccountID    string          `json:"accountId"` // filled in automatically
	Provider     WorklogProvider `json:"provider"`
	TempoBaseURL string          `json:"tempoBaseURL"`
	ProjectKeys  []string        `json:"projectKeys"` // read from Jira; picks the account for an issue key
}

func (a JiraAccount) Site() string { return strings.Trim(a.BaseURL, "/ ") }

func (a JiraAccount) Title() string {
	if a.Name != "" {
		return a.Name
	}
	s := a.Site()
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	return strings.SplitN(s, "/", 2)[0]
}

// ProfileKind is what a profile connects to. Jira profiles are the JiraAccounts; the others are Profiles.
type ProfileKind string

const (
	ProfileGoogleChat ProfileKind = "googleChat" // a Google account: Chat messages, and Calendar without the API
	ProfileHRmaster   ProfileKind = "hrmaster"   // an HRmaster site: work sessions
	ProfileTempo      ProfileKind = "tempo"      // Tempo for one Jira profile's issues
	ProfileConfluence ProfileKind = "confluence" // a Confluence site: the pages you work on
	ProfileEmail      ProfileKind = "email"      // a mailbox over IMAP: the mail you sent
)

// Profile is one signed-in account WorkLogger reads from (Google, HRmaster) or Tempo for a Jira profile.
// Several of a kind are all read and merged. Each signed-in profile has its own site window and sign-in state.
type Profile struct {
	ID   string      `json:"id"`
	Kind ProfileKind `json:"kind"`
	Name string      `json:"name"`
	// Google: the account's address. Picks the account in Google's multi-sign-in and is "me" in chats.
	// Email: the IMAP user name (usually the address).
	Email string `json:"email"`
	// HRmaster: the company's address. Tempo: the API base URL. Confluence: the site, when no Jira profile is linked.
	// Email: the IMAP server ("imap.gmail.com", 993 unless a port is given).
	URL string `json:"url"`
	// Tempo: the Jira profile whose issues it logs (its Tempo token is kept under that account).
	// Confluence: the Jira profile whose site, email and token it uses (optional; its own token wins when saved).
	JiraAccountID string `json:"jiraAccountId"`
}

// Title is the profile's name, else what it points at.
func (p Profile) Title() string {
	switch {
	case p.Name != "":
		return p.Name
	case p.Email != "":
		return p.Email
	case p.URL != "":
		u := p.URL
		if i := strings.Index(u, "://"); i >= 0 {
			u = u[i+3:]
		}
		return strings.SplitN(u, "/", 2)[0]
	}
	switch p.Kind {
	case ProfileGoogleChat:
		return "Google"
	case ProfileHRmaster:
		return "HRmaster"
	case ProfileConfluence:
		return "Confluence"
	case ProfileEmail:
		return "Email"
	}
	return "Tempo"
}

const DefaultTempoURL = "https://api.tempo.io/4"

func (a JiraAccount) JiraTokenKey() string  { return "jira.apiToken." + a.ID }
func (a JiraAccount) TempoTokenKey() string { return "tempo.apiToken." + a.ID }

// ConfluenceTokenKey is a Confluence profile's own token (instead of its linked Jira profile's).
func (p Profile) ConfluenceTokenKey() string { return "confluence.apiToken." + p.ID }

// EmailPasswordKey is an Email profile's IMAP password (an app password where the provider has them).
func (p Profile) EmailPasswordKey() string { return "email.password." + p.ID }

// Keyring account names that aren't per Jira account.
const (
	SecretLegacyJira    = "jira.apiToken" // the single token from before accounts; moved to the first account
	SecretLegacyTempo   = "tempo.apiToken"
	SecretGoogleSecret  = "google.clientSecret"
	SecretGoogleRefresh = "google.refreshToken"
)

// Settings are the user's settings, stored as one JSON document. A few keys are no longer used (macCalendarIds) or
// only mirror newer ones (workspaceRoot, chatEmail, hrmasterURL); they are kept so older databases round-trip.
type Settings struct {
	Modules      []Module `json:"modules"`
	KnownModules []Module `json:"knownModules"`
	TempoByHand  bool     `json:"tempoByHand"`
	Welcomed     bool     `json:"welcomed"`

	WorkspaceRoot       string   `json:"workspaceRoot"`  // the first of WorkspaceRoots (older databases)
	WorkspaceRoots      []string `json:"workspaceRoots"` // every folder scanned for repositories
	AuthorEmails        []string `json:"authorEmails"`
	ExcludedFolders     []string `json:"excludedFolders"`
	IgnoredPathPatterns []string `json:"ignoredPathPatterns"`

	MinCommitMinutes    int `json:"minCommitMinutes"`
	MaxCommitMinutes    int `json:"maxCommitMinutes"`
	MaxCommitGapMinutes int `json:"maxCommitGapMinutes"`

	RoundingMode       RoundingMode `json:"roundingMode"`
	FixedDailyHours    float64      `json:"fixedDailyHours"`
	GranularityMinutes int          `json:"granularityMinutes"`
	DayRoundUpMinutes  int          `json:"dayRoundUpMinutes"`
	DefaultDayStart    string       `json:"defaultDayStart"`
	DefaultDayEnd      string       `json:"defaultDayEnd"`
	IncludeWeekends    bool         `json:"includeWeekends"`

	// Language of the UI: "system" (the OS language when it is one we have, else English), "en" or "hu".
	Language string `json:"language"`

	// Look: "system" | "light" | "dark", and the accent colour.
	Appearance  string `json:"appearance"`
	AccentColor string `json:"accentColor"`
	// The rest of the look, "system" = follow the OS. Skin: "win" | "w10" | "mac" | "macc" | "gtk" | "qt" | "cros".
	// Transparency: "full" | "reduced". Contrast: "normal" | "more". Motion: "full" | "reduced".
	// TextScale: 0 = the OS size, else 1, 1.25, 1.5, 1.75 or 2.
	Skin         string  `json:"skin"`
	Transparency string  `json:"transparency"`
	Contrast     string  `json:"contrast"`
	Motion       string  `json:"motion"`
	TextScale    float64 `json:"textScale"`

	ChatGapMinutes int        `json:"chatGapMinutes"`
	ChatRules      []ChatRule `json:"chatRules"`

	// The first Google and HRmaster profile's address, kept for older databases (SyncProfiles writes them).
	ChatEmail   string `json:"chatEmail"`
	HrmasterURL string `json:"hrmasterURL"`

	Profiles []Profile `json:"profiles"`

	AbsenceIssueKey string        `json:"absenceIssueKey"`
	AbsenceRules    []MeetingRule `json:"absenceRules"`

	Periods []ProjectPeriod `json:"periods"`

	MacCalendarIDs []string `json:"macCalendarIds"`

	GoogleClientID      string        `json:"googleClientId"`
	GoogleCalendarID    string        `json:"googleCalendarId"`
	IcsURLs             []string      `json:"icsUrls"`
	DefaultMeetingIssue string        `json:"defaultMeetingIssue"`
	MeetingRules        []MeetingRule `json:"meetingRules"`
	SkipMeetingKeywords []string      `json:"skipMeetingKeywords"`

	JiraAccounts         []JiraAccount `json:"jiraAccounts"`
	DefaultJiraAccountID *string       `json:"defaultJiraAccountId"`
}

func DefaultSettings() Settings {
	return Settings{
		Modules:      []Module{ModuleGit, ModuleCalendar, ModuleWorkTimes, ModuleChats},
		KnownModules: []Module{ModuleGit, ModuleCalendar, ModuleWorkTimes},
		TempoByHand:  true,
		AuthorEmails: []string{}, ExcludedFolders: []string{},
		IgnoredPathPatterns: []string{
			"package-lock.json", "composer.lock", "yarn.lock", "pnpm-lock.yaml", "Podfile.lock",
			".min.js", ".min.css", "/dist/", "/vendor/", "node_modules/", ".snap", ".svg",
		},
		MinCommitMinutes: 10, MaxCommitMinutes: 240, MaxCommitGapMinutes: 180,
		RoundingMode: RoundingFixedHours, FixedDailyHours: 8, GranularityMinutes: 15, DayRoundUpMinutes: 15,
		DefaultDayStart: "09:00", DefaultDayEnd: "17:30",
		Language:   "system",
		Appearance: "system", AccentColor: "system",
		Skin: "system", Transparency: "system", Contrast: "system", Motion: "system",
		ChatGapMinutes: 10, ChatRules: []ChatRule{},
		AbsenceRules: []MeetingRule{}, Periods: []ProjectPeriod{}, MacCalendarIDs: []string{},
		GoogleCalendarID: "primary", IcsURLs: []string{},
		MeetingRules: []MeetingRule{}, SkipMeetingKeywords: []string{},
		JiraAccounts: []JiraAccount{}, Profiles: []Profile{}, WorkspaceRoots: []string{},
	}
}

// ProfilesOf lists the profiles of one kind, in order.
func (s Settings) ProfilesOf(kind ProfileKind) []Profile {
	var out []Profile
	for _, p := range s.Profiles {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

// Profile finds a profile by id.
func (s Settings) Profile(id string) (Profile, bool) {
	for _, p := range s.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Roots are the workspace folders, without blanks or repeats.
func (s Settings) Roots() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range s.WorkspaceRoots {
		if r = strings.TrimSpace(r); r != "" && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// SyncProfiles keeps the profiles and the fields derived from them in step:
//   - a Jira account linked by a Tempo profile keeps its worklogs in Tempo, at that profile's URL; an account
//     no Tempo profile links that was set to Tempo falls back to Jira's own worklogs
//   - a Tempo profile pointing at a missing Jira account is dropped, and so is a second one for the same account;
//     a Confluence profile linking one is unlinked
//   - chatEmail, hrmasterURL and workspaceRoot mirror the first Google / HRmaster profile and folder (older databases' keys)
func (s *Settings) SyncProfiles() {
	accounts := map[string]bool{}
	for _, a := range s.JiraAccounts {
		accounts[a.ID] = true
	}
	tempoFor := map[string]Profile{}
	kept := s.Profiles[:0:0]
	for _, p := range s.Profiles {
		if p.Kind == ProfileTempo {
			if !accounts[p.JiraAccountID] {
				continue
			}
			if _, dup := tempoFor[p.JiraAccountID]; dup {
				continue
			}
			if strings.TrimSpace(p.URL) == "" {
				p.URL = DefaultTempoURL
			}
			tempoFor[p.JiraAccountID] = p
		}
		if p.Kind == ProfileConfluence && !accounts[p.JiraAccountID] {
			p.JiraAccountID = ""
		}
		kept = append(kept, p)
	}
	s.Profiles = kept
	for i, a := range s.JiraAccounts {
		if t, ok := tempoFor[a.ID]; ok {
			s.JiraAccounts[i].Provider, s.JiraAccounts[i].TempoBaseURL = ProviderTempo, t.URL
		} else if a.Provider == ProviderTempo {
			s.JiraAccounts[i].Provider = ProviderJira
		}
	}
	s.ChatEmail, s.HrmasterURL = "", ""
	if g := s.ProfilesOf(ProfileGoogleChat); len(g) > 0 {
		s.ChatEmail = g[0].Email
	}
	if h := s.ProfilesOf(ProfileHRmaster); len(h) > 0 {
		s.HrmasterURL = h[0].URL
	}
	s.WorkspaceRoots = s.Roots()
	if s.WorkspaceRoots == nil {
		s.WorkspaceRoots = []string{}
	}
	s.WorkspaceRoot = ""
	if len(s.WorkspaceRoots) > 0 {
		s.WorkspaceRoot = s.WorkspaceRoots[0]
	}
}

// migrateProfiles turns the settings from before profiles into them, keeping the ids the old site windows used
// ("hrmaster", "googleChat") so their sign-in state carries over; Jira accounts set to Tempo get a Tempo profile.
func (s *Settings) migrateProfiles() {
	if strings.TrimSpace(s.HrmasterURL) != "" {
		s.Profiles = append(s.Profiles, Profile{ID: "hrmaster", Kind: ProfileHRmaster, URL: s.HrmasterURL})
	}
	if strings.TrimSpace(s.ChatEmail) != "" {
		s.Profiles = append(s.Profiles, Profile{ID: "googleChat", Kind: ProfileGoogleChat, Email: s.ChatEmail})
	}
	for _, a := range s.JiraAccounts {
		if a.Provider == ProviderTempo {
			s.Profiles = append(s.Profiles, Profile{ID: "tempo-" + a.ID, Kind: ProfileTempo, URL: a.TempoBaseURL, JiraAccountID: a.ID})
		}
	}
}

func (s Settings) Has(m Module) bool {
	for _, x := range s.Modules {
		if x == m {
			return true
		}
	}
	return false
}

// DefaultJiraAccount is the picked account, else the first.
func (s Settings) DefaultJiraAccount() *JiraAccount {
	for i := range s.JiraAccounts {
		if s.DefaultJiraAccountID != nil && s.JiraAccounts[i].ID == *s.DefaultJiraAccountID {
			return &s.JiraAccounts[i]
		}
	}
	if len(s.JiraAccounts) > 0 {
		return &s.JiraAccounts[0]
	}
	return nil
}

// JiraAccountFor is the account whose project keys include the issue's project ("PROJ-12" → PROJ), else the default.
func (s Settings) JiraAccountFor(issueKey string) *JiraAccount {
	project := strings.ToUpper(KeyPrefix(issueKey))
	for i, a := range s.JiraAccounts {
		for _, k := range a.ProjectKeys {
			if k == project {
				return &s.JiraAccounts[i]
			}
		}
	}
	return s.DefaultJiraAccount()
}

// DecodeSettings reads stored settings on top of the defaults so new fields never break old databases, and
// switches on modules added since the settings were saved. newID makes the account for pre-account settings.
func DecodeSettings(data []byte, newID func() string) (Settings, bool) {
	s := DefaultSettings()
	var stored map[string]json.RawMessage
	if json.Unmarshal(data, &stored) != nil || json.Unmarshal(data, &s) != nil {
		return DefaultSettings(), false
	}
	// Before accounts there was one Jira site in flat fields; it becomes the first account.
	if _, ok := stored["jiraAccounts"]; !ok {
		var flat struct {
			Site      string  `json:"jiraBaseURL"`
			Email     string  `json:"jiraEmail"`
			AccountID string  `json:"jiraAccountId"`
			Tempo     *string `json:"tempoBaseURL"`
		}
		_ = json.Unmarshal(data, &flat)
		if flat.Site != "" {
			a := NewJiraAccount(newID())
			a.BaseURL, a.Email, a.AccountID = flat.Site, flat.Email, flat.AccountID
			if flat.Tempo != nil {
				a.TempoBaseURL = *flat.Tempo
			}
			s.JiraAccounts = []JiraAccount{a}
			s.DefaultJiraAccountID = &a.ID
		}
	}
	if _, ok := stored["profiles"]; !ok {
		s.migrateProfiles()
	}
	if _, ok := stored["workspaceRoots"]; !ok && strings.TrimSpace(s.WorkspaceRoot) != "" {
		s.WorkspaceRoots = []string{s.WorkspaceRoot}
	}
	s.addNewModules()
	s.FillNil()
	s.SyncProfiles()
	return s, true
}

// FillNil replaces null lists (from JSON null or an old database) with empty ones, so the frontend gets arrays.
func (s *Settings) FillNil() {
	for _, p := range []*[]string{
		&s.AuthorEmails, &s.ExcludedFolders, &s.IgnoredPathPatterns, &s.MacCalendarIDs,
		&s.IcsURLs, &s.SkipMeetingKeywords, &s.WorkspaceRoots,
	} {
		if *p == nil {
			*p = []string{}
		}
	}
	if s.Modules == nil {
		s.Modules = []Module{}
	}
	if s.KnownModules == nil {
		s.KnownModules = []Module{}
	}
	if s.ChatRules == nil {
		s.ChatRules = []ChatRule{}
	}
	if s.AbsenceRules == nil {
		s.AbsenceRules = []MeetingRule{}
	}
	if s.MeetingRules == nil {
		s.MeetingRules = []MeetingRule{}
	}
	if s.Periods == nil {
		s.Periods = []ProjectPeriod{}
	}
	if s.JiraAccounts == nil {
		s.JiraAccounts = []JiraAccount{}
	}
	if s.Profiles == nil {
		s.Profiles = []Profile{}
	}
	for i := range s.JiraAccounts {
		if s.JiraAccounts[i].ProjectKeys == nil {
			s.JiraAccounts[i].ProjectKeys = []string{}
		}
	}
}

// addNewModules: modules added in a later version start switched on.
func (s *Settings) addNewModules() {
	known := map[Module]bool{}
	for _, m := range s.KnownModules {
		known[m] = true
	}
	added := false
	for _, m := range AllModules {
		if !known[m] {
			added = true
			if !s.Has(m) && !optInModules[m] {
				s.Modules = append(s.Modules, m)
			}
		}
	}
	if added {
		s.KnownModules = append([]Module(nil), AllModules...)
	}
}

func NewJiraAccount(id string) JiraAccount {
	return JiraAccount{ID: id, Provider: ProviderTempo, TempoBaseURL: "https://api.tempo.io/4", ProjectKeys: []string{}}
}

// FixDefaultJiraAccount keeps DefaultJiraAccountID on an existing Jira profile: the first one when it is unset
// or gone, nil when there is none.
func (s *Settings) FixDefaultJiraAccount() {
	if a := s.DefaultJiraAccount(); a != nil {
		s.DefaultJiraAccountID = Ptr(a.ID)
	} else {
		s.DefaultJiraAccountID = nil
	}
}

// Clone is a deep copy: changing the copy's lists or accounts leaves s as it was.
func (s Settings) Clone() Settings {
	c := s
	c.Modules = slices.Clone(s.Modules)
	c.KnownModules = slices.Clone(s.KnownModules)
	c.WorkspaceRoots = slices.Clone(s.WorkspaceRoots)
	c.AuthorEmails = slices.Clone(s.AuthorEmails)
	c.ExcludedFolders = slices.Clone(s.ExcludedFolders)
	c.IgnoredPathPatterns = slices.Clone(s.IgnoredPathPatterns)
	c.ChatRules = slices.Clone(s.ChatRules)
	c.Profiles = slices.Clone(s.Profiles)
	c.AbsenceRules = slices.Clone(s.AbsenceRules)
	c.Periods = slices.Clone(s.Periods)
	c.MacCalendarIDs = slices.Clone(s.MacCalendarIDs)
	c.IcsURLs = slices.Clone(s.IcsURLs)
	c.MeetingRules = slices.Clone(s.MeetingRules)
	c.SkipMeetingKeywords = slices.Clone(s.SkipMeetingKeywords)
	c.JiraAccounts = make([]JiraAccount, len(s.JiraAccounts))
	for i, a := range s.JiraAccounts {
		a.ProjectKeys = slices.Clone(a.ProjectKeys)
		c.JiraAccounts[i] = a
	}
	if s.JiraAccounts == nil {
		c.JiraAccounts = nil
	}
	if s.DefaultJiraAccountID != nil {
		c.DefaultJiraAccountID = Ptr(*s.DefaultJiraAccountID)
	}
	return c
}

// Normalize puts unknown language and appearance values back to "system" (text scale 0, which also follows the OS) and gives
// an empty Google calendar id its default, so a hand-edited or older database can't put the UI in a state it
// doesn't know.
func (s *Settings) Normalize() {
	if s.GoogleCalendarID == "" {
		s.GoogleCalendarID = "primary"
	}
	s.Language = oneOf(s.Language, "en", "hu")
	s.Appearance = oneOf(s.Appearance, "light", "dark")
	s.AccentColor = oneOf(s.AccentColor, "blue", "green", "red", "orange", "purple")
	s.Skin = oneOf(s.Skin, "win", "w10", "mac", "macc", "gtk", "qt", "cros")
	s.Transparency = oneOf(s.Transparency, "full", "reduced")
	s.Contrast = oneOf(s.Contrast, "normal", "more")
	s.Motion = oneOf(s.Motion, "full", "reduced")
	switch s.TextScale {
	case 0, 1, 1.25, 1.5, 1.75, 2:
	default:
		s.TextScale = 0
	}
}

// oneOf keeps v when it is one of the allowed values, else "system".
func oneOf(v string, allowed ...string) string {
	if slices.Contains(allowed, v) {
		return v
	}
	return "system"
}
