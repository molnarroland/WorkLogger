// Package core is the pure domain logic. No I/O, no time.Now().
package core

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
)

type Repo struct {
	ID         int64
	Path       string
	Name       string
	IssueKey   *string
	ProjectKey *string
	Enabled    bool
}

// ProjectMapping is a Jira project a repository can be assigned to. Tempo logs only to issues, so a project needs
// a catch-all issue (e.g. "PROJ-1 General development").
type ProjectMapping struct {
	Key          string
	DefaultIssue *string
}

type FileStat struct {
	Added, Deleted int
	Path           string
}

// GitCommit is a commit as read from `git log`.
type GitCommit struct {
	Hash, Author, Email, Subject, Body, Ref string
	Date                                    time.Time
	Files                                   []FileStat
}

// StoredCommit is a commit as stored in SQLite (ignored paths such as lockfiles already removed from the stats).
type StoredCommit struct {
	RepoID                                  int64
	Hash, Author, Email, Subject, Body, Ref string
	Date                                    time.Time
	FilesChanged, Insertions, Deletions     int
}

// NewStoredCommit drops files whose path contains any of ignoredPaths from the stats.
func NewStoredCommit(c GitCommit, repoID int64, ignoredPaths []string) StoredCommit {
	s := StoredCommit{
		RepoID: repoID, Hash: c.Hash, Date: c.Date, Author: c.Author, Email: c.Email,
		Subject: c.Subject, Body: c.Body, Ref: c.Ref,
	}
	for _, f := range c.Files {
		ignored := false
		for _, p := range ignoredPaths {
			if p != "" && strings.Contains(f.Path, p) {
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}
		s.FilesChanged++
		s.Insertions += f.Added
		s.Deletions += f.Deleted
	}
	return s
}

type SessionKind string

const (
	SessionWork  SessionKind = "work"
	SessionPause SessionKind = "pause"
)

// WorkSession is a work session or break imported from the work-time logger.
type WorkSession struct {
	ID         int64
	Day        string
	Start, End time.Time
	Kind       SessionKind
	Label      string
}

type Meeting struct {
	ID, CalendarID, Title, Details string
	Start, End                     time.Time
	Skipped                        bool
}

func (m Meeting) Duration() int { return int(m.End.Sub(m.Start).Seconds()) }

type Source string

const (
	SourceGit        Source = "git"
	SourceMeeting    Source = "meeting"
	SourceAbsence    Source = "absence"
	SourceManual     Source = "manual"
	SourcePeriod     Source = "period"
	SourceChat       Source = "chat"
	SourceJira       Source = "jira"
	SourceConfluence Source = "confluence"
	SourceEmail      Source = "email"
)

// AllSources in display order.
var AllSources = []Source{SourceGit, SourceJira, SourceConfluence, SourceEmail, SourceMeeting, SourceAbsence, SourceManual, SourcePeriod, SourceChat}

// JiraTicket is your activity on one Jira ticket in one day (issues you created, fields you changed, comments):
// its first and last moment.
type JiraTicket struct {
	Day, IssueKey, Summary string
	First, Last            time.Time
}

// Ref is the worklog source ref that claims this ticket-day.
func (t JiraTicket) Ref() string { return "jira:" + t.IssueKey + ":" + t.Day }

// ConfluencePage is your work on one Confluence page in one day (versions you saved, comments): its first and
// last moment.
type ConfluencePage struct {
	Day, PageID, Title, Space string
	First, Last               time.Time
}

// Ref is the worklog source ref that claims this page-day.
func (p ConfluencePage) Ref() string { return "confluence:" + p.PageID + ":" + p.Day }

// EmailThread is the mail you sent in one day on one subject (Re: and Fwd: aside): how many, first and last.
type EmailThread struct {
	Day, Subject string
	Count        int
	First, Last  time.Time
}

// Ref is the worklog source ref that claims this thread-day: the subject goes in as a hash, as refs are joined
// with commas.
func (t EmailThread) Ref() string {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(t.Subject)))
	return fmt.Sprintf("email:%x:%s", h.Sum64(), t.Day)
}

var replyPrefix = regexp.MustCompile(`(?i)^\s*((re|fwd?|fw|aw|wg|sv|vs|válasz|továbbítás|tovább)\s*(\[\d+\])?\s*:\s*)+`)

// ThreadSubject is a mail subject without its Re:, Fwd: (and localised) prefixes: the thread it belongs to.
func ThreadSubject(subject string) string {
	s := strings.TrimSpace(replyPrefix.ReplaceAllString(subject, ""))
	if s == "" {
		return "(no subject)"
	}
	return s
}

type Status string

// Exported covers both sent by the Tempo API and ticked off after entering it in Tempo by hand.
const (
	StatusDraft    Status = "draft"
	StatusReviewed Status = "reviewed"
	StatusExported Status = "exported"
)

type Worklog struct {
	ID          int64
	Day         string
	Start       time.Time
	Duration    int // seconds
	IssueKey    *string
	Description string // WorkLogger's own note (commit messages, meeting title)
	Comment     string // what goes to Tempo; required before the entry can be marked reviewed
	Source      Source
	SourceRef   string // commit hashes or calendar event id, comma separated
	RepoID      *int64
	Estimated   int // raw estimate before daily rounding, seconds
	Status      Status
	Locked      bool // kept when drafts are rebuilt
	TempoID     *int64
}

// TempoText is the worklog's text in Tempo and the exports: the comment, else the generated description.
func (w Worklog) TempoText() string {
	if strings.TrimSpace(w.Comment) != "" {
		return w.Comment
	}
	return w.Description
}

func (w Worklog) Refs() []string {
	var out []string
	for _, r := range strings.Split(w.SourceRef, ",") {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func (w Worklog) End() time.Time { return w.Start.Add(time.Duration(w.Duration) * time.Second) }

// Exportable: Tempo can take it (it has time and an issue).
func (w Worklog) Exportable() bool { return w.Duration > 0 && w.IssueKey != nil }

// NeedsIssue: has time but no issue, so Tempo can't take it and exports skip it.
func (w Worklog) NeedsIssue() bool { return w.IssueKey == nil && w.Duration > 0 }

func TotalDuration(logs []Worklog) int {
	n := 0
	for _, w := range logs {
		n += w.Duration
	}
	return n
}

// DayStatus is the kind of day as reported by the HR system.
type DayStatus string

const (
	DayWorkday       DayStatus = "workday"
	DayRestDay       DayStatus = "restDay"
	DayPublicHoliday DayStatus = "publicHoliday"
	DayLeave         DayStatus = "leave"
	DayOther         DayStatus = "other"
)

// DayStatusFromHRmaster: HRmaster marks days with `Type_CalendarDayStatus_<X>`; leave of any kind is `LabourCode`.
func DayStatusFromHRmaster(t string) DayStatus {
	switch t {
	case "Workday":
		return DayWorkday
	case "RestDay":
		return DayRestDay
	case "PublicHoliday":
		return DayPublicHoliday
	case "LabourCode":
		return DayLeave
	}
	return DayOther
}

func (s DayStatus) IsAbsence() bool { return s == DayLeave || s == DayPublicHoliday }

// DayRecord is what the HR system says about one day, independent of the individual sessions.
type DayRecord struct {
	Day            string
	Status         DayStatus
	Label          string // e.g. "Munkanap (Home Office)", "Szabadság", "Pihenőnap"
	ReportedWorked *int   // "Ledolgozott órák", seconds
	ReportedBreaks *int   // "Napi nem fizetett szünet", seconds
	// ReadOn is the day the record was read from HRmaster ("" for records read before this was kept). A record
	// read on or before its own day may still change: its hours weren't final yet.
	ReadOn string
}

// Ptr returns a pointer to v (for optional fields).
func Ptr[T any](v T) *T { return &v }
