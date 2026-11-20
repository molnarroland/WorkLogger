package view

import (
	"slices"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// SetupStep is something the chosen modules still need. It is done on a profile of kind Profile (Settings →
// Profiles), or on Page (with Tab) when Profile is empty.
type SetupStep struct {
	// ID is repositories, google, calendar, hrmaster, jira, confluence, email or tempo.
	ID      string
	Done    bool
	Profile core.ProfileKind
	Page    string
	Tab     string
}

// SetupSteps is what has to be set up for the chosen modules, in the order to do it. Repositories need folders;
// Chats a Google profile; Meetings a calendar (a Google profile counts, its calendar is read in its window); Work
// sessions an HRmaster profile; Jira activity a Jira profile; Confluence a Confluence profile; sending to Tempo a Jira profile and a Tempo profile
// for it.
func SetupSteps(s core.Settings) []SetupStep {
	filled := func(v string) bool { return strings.TrimSpace(v) != "" }
	has := func(kind core.ProfileKind, field func(core.Profile) string) bool {
		return slices.ContainsFunc(s.Profiles, func(p core.Profile) bool { return p.Kind == kind && filled(field(p)) })
	}
	google := has(core.ProfileGoogleChat, func(p core.Profile) string { return p.Email })
	steps := []SetupStep{}
	if s.Has(core.ModuleGit) {
		steps = append(steps, SetupStep{ID: "repositories", Done: len(s.WorkspaceRoots) > 0, Page: "repositories"})
	}
	if s.Has(core.ModuleChats) {
		steps = append(steps, SetupStep{ID: "google", Done: google, Profile: core.ProfileGoogleChat})
	}
	if s.Has(core.ModuleCalendar) {
		steps = append(steps, SetupStep{
			ID: "calendar", Done: google || len(s.IcsURLs) > 0 || filled(s.GoogleClientID), Page: "settings", Tab: "calendars",
		})
	}
	if s.Has(core.ModuleWorkTimes) {
		steps = append(steps, SetupStep{ID: "hrmaster", Done: has(core.ProfileHRmaster, func(p core.Profile) string { return p.URL }), Profile: core.ProfileHRmaster})
	}
	// The Jira module reads your activity, and sending to Tempo needs the issues: both need a Jira profile.
	if !s.TempoByHand || s.Has(core.ModuleJira) {
		jira := slices.ContainsFunc(s.JiraAccounts, func(a core.JiraAccount) bool { return filled(a.BaseURL) && filled(a.Email) })
		steps = append(steps, SetupStep{ID: "jira", Done: jira, Profile: "jira"})
	}
	if s.Has(core.ModuleEmail) {
		steps = append(steps, SetupStep{ID: "email", Done: has(core.ProfileEmail, func(p core.Profile) string { return p.URL }), Profile: core.ProfileEmail})
	}
	if s.Has(core.ModuleConfluence) {
		steps = append(steps, SetupStep{ID: "confluence", Done: len(s.ProfilesOf(core.ProfileConfluence)) > 0, Profile: core.ProfileConfluence})
	}
	if !s.TempoByHand {
		steps = append(steps, SetupStep{ID: "tempo", Done: has(core.ProfileTempo, func(p core.Profile) string { return p.JiraAccountID }), Profile: core.ProfileTempo})
	}
	return steps
}
