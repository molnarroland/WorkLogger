package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Message is something the UI tells the user: an i18n code with its parameters, which the frontend translates
// (frontend/src/app/i18n, key "msg.<code>"), and the English text, for logs, the activity console and the
// frontend's fallback (RELEASE_REVIEW B7).
type Message struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
	// Skipped is what a job left out and why, as the tools reported it (not translated); shown after the message.
	Skipped string `json:"skipped,omitempty"`
	Text    string `json:"text"`
}

// UserError is an error the user can act on. It crosses to the frontend as the rejection's cause (Wails marshals
// it), so the frontend can show it in the user's language. Unexpected errors stay plain errors.
type UserError struct{ Message }

func (e *UserError) Error() string { return e.Text }

// Say makes the message for code; params come as name, value pairs.
func Say(code string, params ...any) Message {
	m := Message{Code: code}
	if len(params) > 0 {
		m.Params = map[string]any{}
		for i := 0; i+1 < len(params); i += 2 {
			m.Params[fmt.Sprint(params[i])] = params[i+1]
		}
	}
	m.Text = render(code, m.Params)
	return m
}

// Err is Say as an error.
func Err(code string, params ...any) error { return &UserError{Say(code, params...)} }

// WithSkipped adds what a job left out (several reasons are joined).
func (m Message) WithSkipped(reasons []string) Message {
	if len(reasons) > 0 {
		m.Skipped = strings.Join(reasons, "; ")
		m.Text += " Skipped: " + m.Skipped
	}
	return m
}

// MessageOf is the message of a user error, or the error's text with no code (the frontend shows it as is).
func MessageOf(err error) Message {
	var ue *UserError
	if errors.As(err, &ue) {
		return ue.Message
	}
	return Message{Text: err.Error()}
}

func render(code string, params map[string]any) string {
	text, ok := English[code]
	if !ok {
		return code
	}
	for k, v := range params {
		text = strings.ReplaceAll(text, "{"+k+"}", fmt.Sprint(v))
	}
	return text
}

// Codes lists the message codes, sorted (the i18n test checks that every one is translated).
func Codes() []string {
	out := make([]string, 0, len(English))
	for k := range English {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// English is every message's text, with {name} where a parameter goes. The frontend has the same keys under
// "msg." in en.json and hu.json.
var English = map[string]string{
	// Jobs
	"busy":                 "Another collect or fetch is still running.",
	"job.cancelled":        "Cancelled.",
	"collect.done":         "Collected {commits} commits and {meetings} meetings for {from} – {to}.",
	"collect.git":          "Collected {commits} commits for {from} – {to}.",
	"collect.meetings":     "Collected {meetings} meetings for {from} – {to}.",
	"collect.nothing":      "Nothing to collect for {from} – {to}.",
	"collect.jira":         "Collected your work on {tickets} Jira ticket-days for {from} – {to}.",
	"collect.confluence":   "Collected your work on {pages} Confluence page-days for {from} – {to}.",
	"collect.email":        "Collected your mail in {threads} thread-days for {from} – {to}.",
	"collect.meetingsJira": "Collected {meetings} meetings and your work on {tickets} Jira ticket-days for {from} – {to}.",
	"collect.allSettled":   "Tempo already has {from} – {to} in full, so nothing was collected.",
	"collect.needEmail":    "Add your commit email address under Repositories so only your commits are counted.",
	"rebuild.done":         "Drafts rebuilt for {from} – {to}.",
	"push.done":            "Sent {n} worklogs to Tempo.",
	"push.failed":          "Sent {n} worklogs to Tempo. Failed: {reason}",
	"push.dayDone":         "Sent {n} worklogs for {day} to Tempo ({already} already there, left out).",
	"push.dayHasDrafts":    "{day} still has {n} draft entries. Review them before sending the day to Tempo.",
	"push.future":          "{day} hasn't come yet; only past days and today can be sent to Tempo.",
	"hrmaster.imported":    "HRmaster: imported {n} days. Rebuild drafts to apply them.",
	"chat.fetched":         "Google Chat: {messages} new messages in {conversations} conversations.",
	"chat.fetchCancelled":  "Cancelled. Google Chat: {messages} new messages in {conversations} conversations.",
	"calendar.read":        "Read {n} meetings. Rebuild drafts to apply them.",

	// Progress
	"progress.git":            "Reading git history of {n} repositories…",
	"progress.calendar":       "Reading meetings from your calendars…",
	"progress.googleCalendar": "Reading meetings from Google Calendar…",
	"progress.drafts":         "Building drafts…",
	"progress.jira":           "Reading your Jira activity…",
	"progress.email":          "Reading the mail you sent…",
	"progress.confluence":     "Reading your Confluence pages…",
	"progress.push":           "Sending {i} of {n} to Tempo…",

	// Keyring
	"locked.tokens": "Unlock first: tokens are kept in the keyring.",
	"locked.google": "Unlock first: the Google sign-in is kept in the keyring.",

	// Profiles and their windows
	"site.signedOut":         "{name} is signed out. Sign in in its window, then fetch again.",
	"site.noWindow":          "{name} has no sign-in window.",
	"site.notStarted":        "{name}: the window did not start.",
	"site.notLoaded":         "{name} did not finish loading.",
	"site.reloading":         "{name}: the page kept reloading while it was read.",
	"profile.tempoTaken":     "That Jira profile already has a Tempo profile.",
	"hrmaster.needProfile":   "Add an HRmaster profile with your company's address in Settings → Profiles first.",
	"hrmaster.needAddress":   "Enter the HRmaster address of {name} in Settings → Profiles first.",
	"hrmaster.unreadable":    "HRmaster: the page could not be read.",
	"google.needProfile":     "Add a Google profile in Settings → Profiles first.",
	"email.needProfile":      "Add an Email profile in Settings → Profiles to read the mail you sent.",
	"email.needServer":       "Enter the mail server and your user name in Settings → Profiles.",
	"email.needPassword":     "Enter the mail password (an app password for Gmail, iCloud or Yahoo) in Settings → Profiles.",
	"confluence.needProfile": "Add a Confluence profile in Settings → Profiles to read your Confluence pages.",
	"jira.needProfile":       "Add a Jira profile with its API token in Settings → Profiles to read your Jira activity.",

	// Chats
	"chat.future":           "Google Chat: these days haven't come yet.",
	"chat.tooShort":         "This chat is too short to log.",
	"chat.notRecognized":    "Paste the base64 text of the chat export, or its JSON.",
	"chat.pageChanged":      "Google Chat's page changed; the message list could not be read.",
	"chat.searchUnreadable": "Google Chat: the search answer could not be read.",
	"chat.searchFailed":     "Google Chat's search request failed ({reason}); Chat may have changed.",

	// Calendars
	"calendar.needGoogle":       "Add a Google profile in Settings → Profiles, or connect the Calendar API.",
	"calendar.notYours":         "Google Calendar ({name}): none of the calendars ({calendars}) is yours; enter the profile's Google address in Settings → Profiles.",
	"calendar.failed":           "Calendar: {reason}",
	"calendar.exportUnreadable": "Google Calendar: the export could not be read.",
	"calendar.exportFailed":     "Google Calendar: the export failed ({reason}).",
	"calendar.notZip":           "Google Calendar: the export is not a zip file.",
	"google.notConfigured":      "Add the Google OAuth client ID and secret first.",
	"google.notConnected":       "Google Calendar is not connected.",
	"google.timedOut":           "Google sign-in timed out. Try connecting again.",
	"google.cancelled":          "Google sign-in was cancelled ({reason}).",

	// Worklogs and settings
	"worklog.needsComment": "Write the comment that goes to Tempo before marking the entry reviewed.",
	"worklog.needComments": "{n} of the entries have no comment for Tempo yet. Write them before marking them reviewed.",
	"worklog.sent":         "This entry is already in Tempo; change it there.",
	"worklog.badDuration":  "“{text}” is not a duration. Try 1h 30m, 1.5h or 90.",
	"issue.missing":        "Enter an issue key.",
	"issue.invalid":        "“{key}” is not an issue key.",
	"rule.needText":        "Enter the text to look for.",
	"period.backwards":     "The period “{title}” ends before it starts.",
	"folder.missing":       "Choose a folder.",
	"repos.needFolder":     "Add a workspace folder first.",

	// Tempo and Jira
	"tempo.readBackFailed":   "Reading back from Tempo failed: {reason}",
	"missing.jiraAccount":    "Add a Jira profile in Settings → Profiles first.",
	"missing.confluenceSite": "Enter the Confluence site, or link a Jira profile, in Settings → Profiles.",
	"missing.jiraSite":       "Enter the Jira site address in Settings → Profiles.",
	"missing.jiraToken":      "Enter the Jira email and API token in Settings → Profiles.",
	"missing.tempoToken":     "Enter the Tempo API token in Settings → Profiles.",
	"missing.url":            "The Jira site address isn't a valid URL. Check it in Settings → Profiles.",
	"missing.tempoUrl":       "The Tempo API address isn't a valid URL. Check it in Settings → Profiles.",
	"missing.start":          "The worklog has no start time.",
	"missing.ids":            "The issue or the Jira account id is missing.",
}
