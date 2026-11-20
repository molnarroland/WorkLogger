package app

import (
	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/view"
)

// DTOs: JSON friendly, string day keys, int seconds, RFC 3339 times. They carry what internal/view computed,
// so the frontend formats and lays out but doesn't decide.

type WorklogDTO struct {
	ID          int64   `json:"id"`
	Day         string  `json:"day"`
	Start       string  `json:"start"`
	End         string  `json:"end"`
	Duration    int     `json:"duration"`
	IssueKey    *string `json:"issueKey"`
	Description string  `json:"description"`
	Comment     string  `json:"comment"` // what goes to Tempo; needed before Reviewed
	Source      string  `json:"source"`
	Estimated   int     `json:"estimated"`
	Status      string  `json:"status"`
	Locked      bool    `json:"locked"`
	TempoID     *int64  `json:"tempoId"`

	// Derived (internal/view): read-only, ignored by UpdateWorklog.
	NeedsIssue   bool   `json:"needsIssue"`
	NeedsComment bool   `json:"needsComment"`
	TempoText    string `json:"tempoText"`
	InTempo      bool   `json:"inTempo"`
	// EffectiveStatus is "exported" when Tempo already has the entry's ticket-day.
	EffectiveStatus string `json:"effectiveStatus"`
	CanDelete       bool   `json:"canDelete"`
	CanMove         bool   `json:"canMove"`
	// StartMinute is minutes since local midnight.
	StartMinute int `json:"startMinute"`
	// Focus is where the entry's source can be seen (nil for an entry made by hand).
	Focus *view.Focus `json:"focus"`
	// PlanIssue is the ticket Tempo planned for the day, suggested while the entry has none.
	PlanIssue *string `json:"planIssue"`
}

type SourceTotalDTO struct {
	Source  string `json:"source"`
	Seconds int    `json:"seconds"`
}

type PartDTO struct {
	// Kind is a source, or "noTicket".
	Kind    string `json:"kind"`
	Seconds int    `json:"seconds"`
}

type DayDTO struct {
	Day         string           `json:"day"`
	Expected    int              `json:"expected"`
	Label       string           `json:"label"`
	Absence     bool             `json:"absence"`
	Logged      int              `json:"logged"`
	Short       int              `json:"short"`
	Missing     int              `json:"missing"`
	NoComment   int              `json:"noComment"`
	Entries     int              `json:"entries"`
	Drafts      int              `json:"drafts"`
	State       string           `json:"state"`
	Progress    string           `json:"progress"`
	AcceptsWork bool             `json:"acceptsWork"`
	MoveTarget  bool             `json:"moveTarget"`
	PeriodIndex int              `json:"periodIndex"`
	Parts       []PartDTO        `json:"parts"`
	Sources     []SourceTotalDTO `json:"sources"`
	Next        *string          `json:"next"`
	PrevDay     string           `json:"prevDay"`
	NextDay     string           `json:"nextDay"`
}

type SummaryDTO struct {
	Logged         int       `json:"logged"`
	Expected       int       `json:"expected"`
	Short          int       `json:"short"`
	Behind         int       `json:"behind"`
	BehindDays     []string  `json:"behindDays"`
	InTempo        int       `json:"inTempo"`
	InTempoDays    int       `json:"inTempoDays"`
	ToLog          int       `json:"toLog"`
	DraftDays      []string  `json:"draftDays"`
	Missing        []int64   `json:"missing"`
	MissingDays    []string  `json:"missingDays"`
	MissingSeconds int       `json:"missingSeconds"`
	Parts          []PartDTO `json:"parts"`
	BarParts       []PartDTO `json:"barParts"`
}

type TodoDTO struct {
	Kind    string  `json:"kind"`
	Count   int     `json:"count"`
	Seconds int     `json:"seconds"`
	Day     *string `json:"day"`
}

type TicketDayDTO struct {
	Key         *string `json:"key"`
	Day         string  `json:"day"`
	Seconds     int     `json:"seconds"`
	Description string  `json:"description"`
	IDs         []int64 `json:"ids"`
	Entered     bool    `json:"entered"`
	Found       bool    `json:"found"`
	// Planned is Tempo Planner's time for the ticket on the day (seconds).
	Planned int `json:"planned"`
}

type TicketGroupDTO struct {
	Key              *string        `json:"key"`
	Days             []TicketDayDTO `json:"days"`
	Seconds          int            `json:"seconds"`
	EnteredDays      int            `json:"enteredDays"`
	EnteredSeconds   int            `json:"enteredSeconds"`
	RemainingSeconds int            `json:"remainingSeconds"`
	Done             bool           `json:"done"`
	Text             string         `json:"text"`
}

type IssueTotalDTO struct {
	Issue   *string `json:"issue"`
	Seconds int     `json:"seconds"`
}

type ReportGroupDTO struct {
	// ID is "2026-03-02", "2026-W10" or "2026-03".
	ID string `json:"id"`
	// Start is the day its label is made from: the day, the week's Monday, or the month's first.
	Start    string           `json:"start"`
	Total    int              `json:"total"`
	ByIssue  []IssueTotalDTO  `json:"byIssue"`
	BySource []SourceTotalDTO `json:"bySource"`
}

// ReportsDTO is the Totals page's groups for each period.
type ReportsDTO struct {
	Day   []ReportGroupDTO `json:"day"`
	Week  []ReportGroupDTO `json:"week"`
	Month []ReportGroupDTO `json:"month"`
}

func reportDTOs(groups []view.ReportGroup) []ReportGroupDTO {
	out := []ReportGroupDTO{}
	for _, g := range groups {
		d := ReportGroupDTO{ID: g.ID, Start: g.Start, Total: g.Total, ByIssue: []IssueTotalDTO{}, BySource: []SourceTotalDTO{}}
		for _, i := range g.ByIssue {
			d.ByIssue = append(d.ByIssue, IssueTotalDTO(i))
		}
		for _, s := range g.BySource {
			d.BySource = append(d.BySource, SourceTotalDTO{string(s.Source), s.Seconds})
		}
		out = append(out, d)
	}
	return out
}

type TempoProgressDTO struct {
	Entered int `json:"entered"`
	Total   int `json:"total"`
}

type MonthDTO struct {
	Month        string `json:"month"`
	Today        string `json:"today"`
	CurrentMonth string `json:"currentMonth"`
	From         string `json:"from"`
	To           string `json:"to"`
	Prev         string `json:"prev"`
	Next         string `json:"next"`
	// OpenDay is where the Days view opens; AnchorDay the Week view.
	OpenDay   string `json:"openDay"`
	AnchorDay string `json:"anchorDay"`
	// Weeks is the calendar grid, Monday first, padded with the neighbouring months' days.
	Weeks    [][]string   `json:"weeks"`
	Days     []DayDTO     `json:"days"`
	Worklogs []WorklogDTO `json:"worklogs"`
	// InTempo: "KEY|day" ticket-days Tempo already has (read-back).
	InTempo []string `json:"inTempo"`
	// Settled is the month's days in weeks Tempo has in full: Collect skips them, and they can be cleaned up.
	Settled       []string         `json:"settled"`
	Summary       SummaryDTO       `json:"summary"`
	Todos         []TodoDTO        `json:"todos"`
	Attention     []string         `json:"attention"`
	Drafts        []string         `json:"drafts"`
	Queue         []string         `json:"queue"`
	Tickets       []TicketGroupDTO `json:"tickets"`
	TempoProgress TempoProgressDTO `json:"tempoProgress"`
	NeedsTicket   int              `json:"needsTicket"`
	Reports       ReportsDTO       `json:"reports"`
	// Exports is what the export sheet sends for each setting of its switches.
	Exports []ExportChoiceDTO `json:"exports"`
}

func entryDTO(e view.Entry) WorklogDTO {
	w := e.Worklog
	return WorklogDTO{
		ID: w.ID, Day: w.Day, Start: rfc(w.Start), End: rfc(w.End()), Duration: w.Duration, IssueKey: w.IssueKey,
		Description: w.Description, Comment: w.Comment, Source: string(w.Source), Estimated: w.Estimated,
		Status: string(w.Status), Locked: w.Locked, TempoID: w.TempoID,
		NeedsIssue: e.NeedsIssue, NeedsComment: e.NeedsComment, TempoText: e.TempoText, InTempo: e.InTempo,
		EffectiveStatus: string(e.EffectiveStatus), CanDelete: e.CanDelete, CanMove: e.CanMove,
		StartMinute: e.StartMinute, Focus: e.Focus, PlanIssue: e.PlanIssue,
	}
}

// worklogDTO is an entry outside a month view (no Tempo read-back).
func (s *Service) worklogDTO(w core.Worklog) WorklogDTO {
	return entryDTO(view.NewEntry(w, nil, s.Days))
}

func partDTOs(parts []view.Part) []PartDTO {
	out := []PartDTO{}
	for _, p := range parts {
		out = append(out, PartDTO(p))
	}
	return out
}

type WeekDTO struct {
	Day string `json:"day"`
	// Days run Monday to Sunday.
	Days     []DayDTO     `json:"days"`
	Worklogs []WorklogDTO `json:"worklogs"`
	Logged   int          `json:"logged"`
	Expected int          `json:"expected"`
	Prev     string       `json:"prev"`
	Next     string       `json:"next"`
	// Month is the week's month as ISO weeks count it (its Thursday's).
	Month string `json:"month"`
}

func dayDTO(d view.Day) DayDTO {
	sources := []SourceTotalDTO{}
	for _, s := range d.Sources {
		sources = append(sources, SourceTotalDTO{string(s.Source), s.Seconds})
	}
	return DayDTO{
		Sources: sources,
		Day:     d.Day, Expected: d.Expected, Label: d.Label, Absence: d.Absence, Logged: d.Logged, Short: d.Short,
		Missing: d.Missing, NoComment: d.NoComment, Entries: d.Entries, Drafts: d.Drafts,
		State: string(d.State), Progress: string(d.Progress), AcceptsWork: d.AcceptsWork, MoveTarget: d.MoveTarget,
		PeriodIndex: d.PeriodIndex, Parts: partDTOs(d.Parts), Next: d.Next, PrevDay: d.PrevDay, NextDay: d.NextDay,
	}
}

func weekDTO(w view.Week) WeekDTO {
	out := WeekDTO{
		Day: w.Day, Days: []DayDTO{}, Worklogs: []WorklogDTO{}, Logged: w.Logged, Expected: w.Expected,
		Prev: w.Prev, Next: w.Next, Month: w.Month,
	}
	for _, d := range w.Days {
		out.Days = append(out.Days, dayDTO(d))
	}
	for _, e := range w.Entries {
		out.Worklogs = append(out.Worklogs, entryDTO(e))
	}
	return out
}

func monthDTO(m view.Month, inTempo []string) MonthDTO {
	out := MonthDTO{
		Month: m.Month, Today: m.Today, CurrentMonth: m.CurrentMonth, From: m.From, To: m.To, Prev: m.Prev,
		Next: m.Next, OpenDay: m.OpenDay, AnchorDay: m.AnchorDay, Weeks: m.Weeks, Days: []DayDTO{}, Worklogs: []WorklogDTO{}, InTempo: inTempo, Settled: []string{},
		Todos: []TodoDTO{}, Attention: m.Attention, Drafts: m.Drafts, Queue: m.Queue, Tickets: []TicketGroupDTO{},
		TempoProgress: TempoProgressDTO(m.TempoProgress), NeedsTicket: m.NeedsTicket,
		Exports: exportDTOs(m.Exports),
		Reports: ReportsDTO{
			Day: reportDTOs(m.Reports[view.PeriodDay]), Week: reportDTOs(m.Reports[view.PeriodWeek]),
			Month: reportDTOs(m.Reports[view.PeriodMonth]),
		},
	}
	for _, d := range m.Days {
		out.Days = append(out.Days, dayDTO(d))
	}
	for _, e := range m.Entries {
		out.Worklogs = append(out.Worklogs, entryDTO(e))
	}
	s := m.Summary
	out.Summary = SummaryDTO{
		Logged: s.Logged, Expected: s.Expected, Short: s.Short, Behind: s.Behind, BehindDays: s.BehindDays, InTempo: s.InTempo,
		InTempoDays: s.InTempoDays, ToLog: s.ToLog, DraftDays: s.DraftDays, Missing: s.Missing,
		MissingDays: s.MissingDays, MissingSeconds: s.MissingSeconds, Parts: partDTOs(s.Parts),
		BarParts: partDTOs(s.BarParts),
	}
	for _, t := range m.Todos {
		out.Todos = append(out.Todos, TodoDTO{Kind: string(t.Kind), Count: t.Count, Seconds: t.Seconds, Day: t.Day})
	}
	for _, g := range m.Tickets {
		gd := TicketGroupDTO{Key: g.Key, Days: []TicketDayDTO{}, Seconds: g.Seconds, EnteredDays: g.EnteredDays, EnteredSeconds: g.EnteredSeconds, RemainingSeconds: g.RemainingSeconds, Done: g.Done, Text: g.Text}
		for _, d := range g.Days {
			gd.Days = append(gd.Days, TicketDayDTO(d))
		}
		out.Tickets = append(out.Tickets, gd)
	}
	return out
}

// SessionDayDTO is one day of the Work sessions page.
type SessionDayDTO struct {
	Day   string `json:"day"`
	Label string `json:"label"`
	// Status is workday, restday, holiday or leave.
	Status   string       `json:"status"`
	Sessions []SessionDTO `json:"sessions"`
	// Net is the work sessions minus the breaks; Reported HR's worked time.
	Net      int  `json:"net"`
	Reported int  `json:"reported"`
	Mismatch bool `json:"mismatch"`
}

type SessionsDTO struct {
	Days []SessionDayDTO `json:"days"`
}

func sessionsDTO(days []view.SessionDay, d core.Days) SessionsDTO {
	out := SessionsDTO{Days: []SessionDayDTO{}}
	for _, day := range days {
		sd := SessionDayDTO{
			Day: day.Day, Label: day.Label, Status: day.Status, Sessions: []SessionDTO{}, Net: day.Net,
			Reported: day.Reported, Mismatch: day.Mismatch,
		}
		midnight := d.Date(day.Day)
		for _, w := range day.Sessions {
			sd.Sessions = append(sd.Sessions, SessionDTO{
				ID: w.ID, Day: w.Day, Start: rfc(w.Start), End: rfc(w.End), Kind: string(w.Kind), Label: w.Label,
				StartMinute: int(w.Start.Sub(midnight).Minutes()), EndMinute: int(w.End.Sub(midnight).Minutes()),
			})
		}
		out.Days = append(out.Days, sd)
	}
	return out
}

// SetupStepDTO is something the chosen modules still need: done on a profile of kind Profile (Settings →
// Profiles), or on Page (with Tab) when Profile is empty.
type SetupStepDTO struct {
	ID      string `json:"id"`
	Done    bool   `json:"done"`
	Profile string `json:"profile"`
	Page    string `json:"page"`
	Tab     string `json:"tab"`
}

func setupDTOs(steps []view.SetupStep) []SetupStepDTO {
	out := []SetupStepDTO{}
	for _, s := range steps {
		out = append(out, SetupStepDTO{ID: s.ID, Done: s.Done, Profile: string(s.Profile), Page: s.Page, Tab: s.Tab})
	}
	return out
}

type ExportChoiceDTO struct {
	OnlyReviewed    bool    `json:"onlyReviewed"`
	IncludeExported bool    `json:"includeExported"`
	IDs             []int64 `json:"ids"`
	Seconds         int     `json:"seconds"`
	// Missing counts the chosen entries left out because they have no ticket.
	Missing int `json:"missing"`
}

func exportDTOs(choices []view.ExportChoice) []ExportChoiceDTO {
	out := []ExportChoiceDTO{}
	for _, c := range choices {
		out = append(out, ExportChoiceDTO(c))
	}
	return out
}

// MeetingsDTO is the meetings of a range and the untagged title worth a rule (nil: none comes up twice).
type MeetingsDTO struct {
	Meetings   []MeetingDTO   `json:"meetings"`
	Suggestion *SuggestionDTO `json:"suggestion"`
}

type SuggestionDTO struct {
	Title string `json:"title"`
	Count int    `json:"count"`
}

func meetingsDTO(rows []view.MeetingRow, d core.Days) MeetingsDTO {
	out := MeetingsDTO{Meetings: []MeetingDTO{}}
	for _, m := range rows {
		out.Meetings = append(out.Meetings, MeetingDTO{
			ID: m.ID, Day: d.Key(m.Start), Title: m.Title, Start: rfc(m.Start), End: rfc(m.End), Seconds: m.Duration(),
			Skipped: m.Skipped, SkippedByKeyword: m.SkippedByKeyword, IssueKey: m.IssueKey,
		})
	}
	if sug := view.MeetingSuggestion(rows); sug != nil {
		out.Suggestion = &SuggestionDTO{sug.Title, sug.Count}
	}
	return out
}

// ChatsDTO is the conversation-days under their days, and the range's active time.
type ChatsDTO struct {
	Days   []ChatGroupDTO `json:"days"`
	Active int            `json:"active"`
}

type ChatGroupDTO struct {
	Day     string       `json:"day"`
	Seconds int          `json:"seconds"`
	Chats   []ChatDayDTO `json:"chats"`
}

func chatsDTO(list []ChatDayDTO) ChatsDTO {
	out := ChatsDTO{Days: []ChatGroupDTO{}}
	for _, g := range view.GroupByDay(list, func(c ChatDayDTO) string { return c.Day }, func(c ChatDayDTO) int { return c.ActiveSeconds }) {
		out.Days = append(out.Days, ChatGroupDTO{Day: g.Day, Seconds: g.Seconds, Chats: g.Items})
		out.Active += g.Seconds
	}
	return out
}

// ── Repositories, sessions, meetings, chats, jobs ───────────────────────────

type RepoDTO struct {
	ID         int64   `json:"id"`
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	IssueKey   *string `json:"issueKey"`
	ProjectKey *string `json:"projectKey"`
	Enabled    bool    `json:"enabled"`
	Commits    int     `json:"commits"`
	// Your commits on each day of the month that has any ("YYYY-MM-DD" → count), for the page's date filter.
	Days map[string]int `json:"days"`
}

type ProjectDTO struct {
	Key          string  `json:"key"`
	DefaultIssue *string `json:"defaultIssue"`
}

type SessionDTO struct {
	ID    int64  `json:"id"`
	Day   string `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
	// StartMinute and EndMinute are minutes since the day's local midnight (past 1440 when it runs over).
	StartMinute int `json:"startMinute"`
	EndMinute   int `json:"endMinute"`
}

type MeetingDTO struct {
	Day string `json:"day"`
	// Seconds is its length.
	Seconds          int     `json:"seconds"`
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Start            string  `json:"start"`
	End              string  `json:"end"`
	Skipped          bool    `json:"skipped"`
	SkippedByKeyword bool    `json:"skippedByKeyword"`
	IssueKey         *string `json:"issueKey"`
}

type CalendarStatus struct {
	GoogleConnected bool `json:"googleConnected"`
	// GoogleWeb: no API connection needed, Google Calendar is read from calendar.google.com with the Google Chat
	// window's sign-in.
	GoogleWeb bool `json:"googleWeb"`
}

type ChatDayDTO struct {
	ID             string  `json:"id"`
	Day            string  `json:"day"`
	ConversationID string  `json:"conversationId"`
	Name           string  `json:"name"`
	Type           string  `json:"type"` // dm | space
	Count          int     `json:"count"`
	ActiveSeconds  int     `json:"activeSeconds"`
	First          string  `json:"first"`
	Last           string  `json:"last"`
	IssueKey       *string `json:"issueKey"`
	Exported       bool    `json:"exported"`
	// RuleIssue is the issue an "Always" rule logs the conversation to.
	RuleIssue *string `json:"ruleIssue"`
}

type ChatMessageDTO struct {
	Time   string   `json:"time"`
	Sender string   `json:"sender"`
	Me     bool     `json:"me"`
	Text   string   `json:"text"`
	Files  []string `json:"files"`
}

// CollectProgress is collect:progress: what a running job is doing.
type CollectProgress struct {
	Source  string       `json:"source"`
	Message core.Message `json:"message"`
}

// CollectDone is collect:done: how a job ended (its result, or what went wrong).
type CollectDone struct {
	Cancelled bool         `json:"cancelled"`
	Failed    bool         `json:"failed"` // the job ended with an error (Message says which)
	Message   core.Message `json:"message"`
	// Settled is how many days Collect left out because Tempo has their weeks in full (0: none, or forced).
	Settled int `json:"settled"`
}

const (
	EventProgress = "collect:progress"
	EventDone     = "collect:done"
	EventChatDay  = "chat:day" // a fetched Google Chat day is saved; data is the day
)
