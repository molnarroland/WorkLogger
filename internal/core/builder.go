package core

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DayInput struct {
	Day      string
	Sessions []WorkSession
	Commits  []StoredCommit
	Meetings []Meeting
	// Chats are the day's conversations; the long ones become drafts (see chatDrafts).
	Chats []ChatDay
	// Jira is the day's activity per ticket (the Jira module); it shares the free time with the commits.
	Jira []JiraTicket
	// Confluence is the day's work per page (the Confluence module); it is weighed like a Jira ticket.
	Confluence []ConfluencePage
	// Email is the day's sent mail per thread (the Email module); each mail weighs a few minutes.
	Email []EmailThread
	// Kept are entries that survive a rebuild (locked, reviewed, exported, manual).
	Kept []Worklog
	// Info is the day status and totals from the HR system, when imported.
	Info *DayRecord
}

// Builder turns one day's commits, meetings and work sessions into draft worklogs.
type Builder struct {
	Settings Settings
	Repos    map[int64]Repo
	Projects map[string]ProjectMapping
	Days     Days
}

func (b Builder) KnownPrefixes() map[string]bool {
	s := map[string]bool{}
	for k := range b.Projects {
		s[k] = true
	}
	for _, a := range b.Settings.JiraAccounts {
		for _, k := range a.ProjectKeys {
			s[k] = true
		}
	}
	for _, r := range b.Repos {
		if r.ProjectKey != nil {
			s[*r.ProjectKey] = true
		}
		if r.IssueKey != nil {
			s[KeyPrefix(*r.IssueKey)] = true
		}
	}
	return s
}

// CommitIssue: commit message → branch name → repository issue → repository project's catch-all issue.
func (b Builder) CommitIssue(c StoredCommit) *string {
	known := b.KnownPrefixes()
	if k, ok := FindKey(c.Subject+"\n"+c.Body, known); ok {
		return &k
	}
	if k, ok := FindKey(strings.ToUpper(c.Ref), known); ok {
		return &k
	}
	repo, ok := b.Repos[c.RepoID]
	if !ok {
		return nil
	}
	if repo.IssueKey != nil {
		return repo.IssueKey
	}
	if repo.ProjectKey != nil {
		if p, ok := b.Projects[*repo.ProjectKey]; ok && p.DefaultIssue != nil {
			return p.DefaultIssue
		}
	}
	return nil
}

func firstRule(rules []MeetingRule, text string) (MeetingRule, bool) {
	lower := strings.ToLower(text)
	for _, r := range rules {
		if r.Contains != "" && strings.Contains(lower, strings.ToLower(r.Contains)) {
			return r, true
		}
	}
	return MeetingRule{}, false
}

// MeetingIssue: title/description key → keyword rules → default meeting issue.
func (b Builder) MeetingIssue(m Meeting) *string {
	if k, ok := FindKey(m.Title+"\n"+m.Details, b.KnownPrefixes()); ok {
		return &k
	}
	if r, ok := firstRule(b.Settings.MeetingRules, m.Title); ok {
		return NormalizeKeyString(r.IssueKey)
	}
	return NormalizeKeyString(b.Settings.DefaultMeetingIssue)
}

// SkippedByKeyword: the title has a "never log" keyword.
func (b Builder) SkippedByKeyword(m Meeting) bool {
	title := strings.ToLower(m.Title)
	for _, k := range b.Settings.SkipMeetingKeywords {
		if k != "" && strings.Contains(title, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func (b Builder) IsSkipped(m Meeting) bool { return m.Skipped || b.SkippedByKeyword(m) }

// IsWorkday: the HR system's status wins over the calendar (transferred Saturdays are workdays, a weekday holiday is not).
func (b Builder) IsWorkday(day string, info *DayRecord) bool {
	if info != nil && info.Status != DayOther {
		return info.Status == DayWorkday
	}
	return !b.Days.IsWeekend(day)
}

func (b Builder) fixedSeconds() int { return int(b.Settings.FixedDailyHours * 3600) }

// DailyTarget is how long the day should add up to, or nil to keep the raw estimates.
func (b Builder) DailyTarget(day string, info *DayRecord, windows []Interval, fromSessions bool) *int {
	fixed := b.fixedSeconds()
	worked := int(Total(windows).Seconds())
	if info != nil && info.ReportedWorked != nil && *info.ReportedWorked > 0 {
		worked = *info.ReportedWorked
	}
	if b.Settings.RoundingMode == RoundingEstimate {
		return nil
	}
	// Imported work sessions decide the day, rounded up (7h 22m → 7h 30m); the hours per day are only a fallback.
	if fromSessions {
		step := max(1, b.Settings.DayRoundUpMinutes) * 60
		return Ptr(int(math.Ceil(float64(worked)/float64(step))) * step)
	}
	if b.IsWorkday(day, info) {
		return &fixed
	}
	absence := info != nil && info.Status.IsAbsence()
	if b.Settings.IncludeWeekends && !absence {
		return &fixed
	}
	return nil
}

// CommitDay is where a day's commits are logged: the day itself, or the closest workday (the earlier one on a tie)
// for leave and for rest days without work sessions, so Saturday goes to Friday and Sunday to Monday.
func (b Builder) CommitDay(day string, info func(string) *DayRecord, worked func(string) bool) string {
	leave := info(day) != nil && info(day).Status == DayLeave
	idleRestDay := !b.IsWorkday(day, info(day)) && !worked(day) && !b.Settings.IncludeWeekends
	if !leave && !idleRestDay {
		return day
	}
	for offset := 1; offset <= 31; offset++ {
		for _, sign := range []int{-1, 1} {
			d := b.Days.Shift(day, sign*offset)
			if b.IsWorkday(d, info(d)) {
				return d
			}
		}
	}
	return day
}

// Period is the project period a day belongs to; the latest added wins when periods overlap.
func (b Builder) Period(day string) *ProjectPeriod {
	for i := len(b.Settings.Periods) - 1; i >= 0; i-- {
		p := b.Settings.Periods[i]
		if p.Contains(day) && NormalizeKeyString(p.IssueKey) != nil {
			return &p
		}
	}
	return nil
}

// AbsenceIssue for a leave / holiday day: label rule ("Betegszabadság" → HR-3) → default absence issue.
func (b Builder) AbsenceIssue(info DayRecord) *string {
	if r, ok := firstRule(b.Settings.AbsenceRules, info.Label); ok {
		return NormalizeKeyString(r.IssueKey)
	}
	return NormalizeKeyString(b.Settings.AbsenceIssueKey)
}

func hasWork(sessions []WorkSession) bool {
	for _, s := range sessions {
		if s.Kind == SessionWork {
			return true
		}
	}
	return false
}

func (b Builder) absenceWorklog(in DayInput, used map[string]bool) *Worklog {
	if in.Info == nil || !in.Info.Status.IsAbsence() {
		return nil
	}
	issue := b.AbsenceIssue(*in.Info)
	ref := "absence:" + in.Day
	if issue == nil || used[ref] || hasWork(in.Sessions) {
		return nil
	}
	// Leave counts as a full day unless the HR system reports otherwise; a holiday only when it reports hours.
	seconds := 0
	switch {
	case in.Info.ReportedWorked != nil:
		seconds = *in.Info.ReportedWorked
	case in.Info.Status == DayLeave:
		seconds = b.fixedSeconds()
	}
	if seconds <= 0 {
		return nil
	}
	start, ok := b.Days.AtHM(in.Day, b.Settings.DefaultDayStart)
	if !ok {
		start = b.Days.Date(in.Day)
	}
	desc := in.Info.Label
	if desc == "" {
		desc = "Absence"
	}
	return &Worklog{
		Day: in.Day, Start: start, Duration: seconds, IssueKey: issue, Description: desc,
		Source: SourceAbsence, SourceRef: ref, Estimated: seconds, Status: StatusDraft,
	}
}

func intervals(sessions []WorkSession, kind SessionKind) []Interval {
	var out []Interval
	for _, s := range sessions {
		if s.Kind == kind {
			out = append(out, Interval{s.Start, s.End})
		}
	}
	return out
}

// NetWork is the work sessions with the breaks taken out.
func NetWork(sessions []WorkSession) []Interval {
	return Subtract(intervals(sessions, SessionWork), intervals(sessions, SessionPause))
}

// WorkWindows is the working time of the day: imported sessions minus breaks, or the default workday.
func (b Builder) WorkWindows(in DayInput) ([]Interval, bool) {
	if len(intervals(in.Sessions, SessionWork)) > 0 {
		return NetWork(in.Sessions), true
	}
	s, ok1 := b.Days.AtHM(in.Day, b.Settings.DefaultDayStart)
	e, ok2 := b.Days.AtHM(in.Day, b.Settings.DefaultDayEnd)
	if !ok1 || !ok2 || !e.After(s) {
		return nil, false
	}
	return Subtract([]Interval{{s, e}}, intervals(in.Sessions, SessionPause)), false
}

// Expected is what the timesheet expects for a day: the rounding target, or the hours per day on a workday.
func (b Builder) Expected(day string, sessions []WorkSession, info *DayRecord) int {
	windows, fromSessions := b.WorkWindows(DayInput{Day: day, Sessions: sessions, Info: info})
	if t := b.DailyTarget(day, info, windows, fromSessions); t != nil {
		return *t
	}
	if b.IsWorkday(day, info) {
		return b.fixedSeconds()
	}
	return 0
}

type block struct {
	key      string
	issue    *string
	repoID   *int64
	seconds  int
	subjects []string
	hashes   []string
	first    time.Time
	source   Source
}

// Jira-only tickets and Confluence pages weigh the span of the day's activity on them, within these bounds.
// ponytail: span is a rough proxy for effort; per-action weights if users find tickets over- or under-booked.
const (
	jiraMinSeconds = 15 * 60
	jiraMaxSeconds = 2 * 3600
)

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (b Builder) Build(in DayInput) []Worklog {
	hasWorkSessions := hasWork(in.Sessions)
	used := map[string]bool{}
	for _, k := range in.Kept {
		for _, r := range k.Refs() {
			used[r] = true
		}
	}
	absence := b.absenceWorklog(in, used)
	// Leave is logged as one absence entry; commits were moved to the closest workday, meetings are not logged.
	if in.Info != nil && in.Info.Status == DayLeave {
		if absence != nil {
			return []Worklog{*absence}
		}
		return nil
	}
	period := b.Period(in.Day)
	fillsPeriod := period != nil && (hasWorkSessions || b.IsWorkday(in.Day, in.Info))
	chats := b.chatDrafts(in, used)
	if len(in.Commits) == 0 && len(in.Meetings) == 0 && len(chats) == 0 && len(in.Jira) == 0 && len(in.Confluence) == 0 && len(in.Email) == 0 && !hasWorkSessions && absence == nil && !fillsPeriod {
		return nil
	}

	windows, fromSessions := b.WorkWindows(in)
	gran := max(1, b.Settings.GranularityMinutes) * 60
	var out []Worklog
	if absence != nil {
		out = append(out, *absence)
	}

	// 1. Meetings keep their real time slot.
	var meetings []Meeting
	for _, m := range in.Meetings {
		if !b.IsSkipped(m) && !used[m.ID] && m.End.After(m.Start) {
			meetings = append(meetings, m)
		}
	}
	sort.SliceStable(meetings, func(i, j int) bool { return meetings[i].Start.Before(meetings[j].Start) })
	for _, m := range meetings {
		out = append(out, Worklog{
			Day: in.Day, Start: m.Start, Duration: roundTo(m.Duration(), gran, gran), IssueKey: b.MeetingIssue(m),
			Description: m.Title, Source: SourceMeeting, SourceRef: m.ID, Estimated: m.Duration(), Status: StatusDraft,
		})
	}
	var busy []Interval
	for _, m := range meetings {
		busy = append(busy, Interval{m.Start, m.End})
	}
	// 1b. Long chats too, from their first message, for the time spent chatting.
	for _, c := range chats {
		out = append(out, c)
		busy = append(busy, Interval{c.Start, c.End()})
	}
	if absence != nil {
		busy = append(busy, Interval{absence.Start, absence.End()})
	}
	for _, k := range in.Kept {
		busy = append(busy, Interval{k.Start, k.End()})
	}
	free := Subtract(windows, busy)

	// 2. Estimate commits and group them per issue (or per repository when no issue is known).
	var commits []StoredCommit
	for _, c := range in.Commits {
		if !used[c.Hash] {
			commits = append(commits, c)
		}
	}
	sort.SliceStable(commits, func(i, j int) bool { return commits[i].Date.Before(commits[j].Date) })
	var blocks []block
	var previous *time.Time
	if len(windows) > 0 {
		previous = &windows[0].Start
	}
	for _, c := range commits {
		var gap *float64
		// Commits moved here from a leave day have no gap on this day; they are estimated by size only.
		if previous != nil && c.Date.After(*previous) && b.Days.Key(c.Date) == in.Day {
			gap = Ptr(Overlap(free, Interval{*previous, c.Date}).Seconds())
		}
		seconds := EstimateSeconds(b.Settings, c, gap)
		if previous == nil || c.Date.After(*previous) {
			previous = Ptr(c.Date)
		}

		resolved := b.CommitIssue(c)
		key := "repo:" + strconv.FormatInt(c.RepoID, 10)
		if resolved != nil {
			key = *resolved
		}
		subject := StripLeadingKey(c.Subject)
		found := false
		for i := range blocks {
			if blocks[i].key == key {
				blocks[i].seconds += seconds
				if !contains(blocks[i].subjects, subject) {
					blocks[i].subjects = append(blocks[i].subjects, subject)
				}
				blocks[i].hashes = append(blocks[i].hashes, c.Hash)
				found = true
				break
			}
		}
		if !found {
			blocks = append(blocks, block{
				key: key, issue: resolved, repoID: Ptr(c.RepoID), seconds: seconds,
				subjects: []string{subject}, hashes: []string{c.Hash}, first: c.Date, source: SourceGit,
			})
		}
	}
	blocks = b.workBlocks(blocks, in, used)

	// 3. Daily target.
	target := b.DailyTarget(in.Day, in.Info, windows, fromSessions)
	fixedTotal := TotalDuration(out) + TotalDuration(in.Kept)

	var gitTarget *int
	if target != nil {
		gitTarget = Ptr(max(0, roundTo(*target, gran, 0)-fixedTotal))
	}
	weights := make([]int, len(blocks))
	for i, bl := range blocks {
		weights[i] = bl.seconds
	}
	// Inside a project period the work keeps its own tickets at its estimates (squeezed to fit the day if it runs
	// over), and the period's issue takes only what is still missing from the day.
	fill := 0
	fillRef := ""
	if fillsPeriod {
		fillRef = "period:" + strings.ToUpper(period.ID) + ":" + in.Day
		room := gitTarget
		if room == nil {
			room = Ptr(max(0, b.fixedSeconds()-fixedTotal))
		}
		estimated := 0
		for _, a := range Allocate(weights, nil, gran) {
			estimated += a
		}
		if estimated < *room {
			gitTarget = Ptr(estimated)
			if !used[fillRef] {
				fill = *room - estimated
			}
		}
	}
	allocations := Allocate(weights, gitTarget, gran)

	// 4. Place commit blocks into the free time, in the order the work happened.
	var cursor time.Time
	switch {
	case len(free) > 0:
		cursor = free[0].Start
	case len(commits) > 0:
		cursor = commits[0].Date
	default:
		if t, ok := b.Days.AtHM(in.Day, b.Settings.DefaultDayStart); ok {
			cursor = t
		} else {
			cursor = b.Days.Date(in.Day)
		}
	}
	for i, bl := range blocks {
		start, next := Place(allocations[i], cursor, free)
		cursor = next
		out = append(out, Worklog{
			Day: in.Day, Start: start, Duration: allocations[i], IssueKey: bl.issue, Description: b.describe(bl),
			Source: bl.source, SourceRef: strings.Join(bl.hashes, ","), RepoID: bl.repoID, Estimated: bl.seconds,
			Status: StatusDraft,
		})
	}
	if fill > 0 {
		start, _ := Place(fill, cursor, free)
		out = append(out, Worklog{
			Day: in.Day, Start: start, Duration: fill, IssueKey: NormalizeKeyString(period.IssueKey),
			Description: period.Title, Comment: strings.TrimSpace(period.Comment), Source: SourcePeriod,
			SourceRef: fillRef, Estimated: fill, Status: StatusDraft,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// workItem is a Jira ticket-day or a Confluence page-day: work without commits of its own.
type workItem struct {
	ref, key string
	issue    *string
	summary  string
	first    time.Time
	seconds  int // its weight
	source   Source
}

// spanSeconds weighs Jira tickets and Confluence pages: the span of the day's activity on them, within bounds.
func spanSeconds(first, last time.Time) int {
	return min(jiraMaxSeconds, max(jiraMinSeconds, int(last.Sub(first).Seconds())))
}

// A sent mail weighs emailSeconds, a thread-day at most emailMaxSeconds.
// ponytail: a flat rate per mail; weigh by length or recipients if threads come out over- or under-booked.
const (
	emailSeconds    = 10 * 60
	emailMaxSeconds = 3600
)

// workBlocks adds the day's Jira tickets, Confluence pages and mail threads to the commit blocks: one whose ticket already has a
// block only adds its ref (commits measure the work), any other becomes a block of its own, weighed by the span
// of the day's activity on it (a mail thread by how many you sent). A page's or thread's ticket is a key in its
// title or subject; without one it is a block of its own with no ticket (picked in Review). Blocks are then in the order the work started, so they are placed that way.
func (b Builder) workBlocks(blocks []block, in DayInput, used map[string]bool) []block {
	var items []workItem
	for _, t := range in.Jira {
		if key := NormalizeKeyString(t.IssueKey); key != nil {
			items = append(items, workItem{t.Ref(), *key, key, t.Summary, t.First, spanSeconds(t.First, t.Last), SourceJira})
		}
	}
	known := b.KnownPrefixes()
	for _, p := range in.Confluence {
		it := workItem{ref: p.Ref(), key: "confluence:" + p.PageID, summary: p.Title, first: p.First,
			seconds: spanSeconds(p.First, p.Last), source: SourceConfluence}
		if p.Space != "" {
			it.summary = "[" + p.Space + "] " + p.Title
		}
		if k, ok := FindKey(p.Title, known); ok {
			it.key, it.issue = k, &k
		}
		items = append(items, it)
	}
	for _, t := range in.Email {
		it := workItem{ref: t.Ref(), key: "email:" + strings.ToLower(t.Subject), summary: "[Email] " + t.Subject,
			first: t.First, seconds: min(emailMaxSeconds, t.Count*emailSeconds), source: SourceEmail}
		if k, ok := FindKey(t.Subject, known); ok {
			it.key, it.issue = k, &k
		}
		items = append(items, it)
	}
	for _, it := range items {
		if used[it.ref] {
			continue
		}
		merged := false
		for i := range blocks {
			if blocks[i].key == it.key {
				blocks[i].hashes = append(blocks[i].hashes, it.ref)
				merged = true
				break
			}
		}
		if !merged {
			blocks = append(blocks, block{
				key: it.key, issue: it.issue, seconds: it.seconds, subjects: []string{it.summary}, hashes: []string{it.ref},
				first: it.first, source: it.source,
			})
		}
	}
	sort.SliceStable(blocks, func(i, j int) bool { return blocks[i].first.Before(blocks[j].first) })
	return blocks
}

func (b Builder) describe(bl block) string {
	text := strings.Join(bl.subjects, "; ")
	if bl.issue == nil && bl.repoID != nil {
		if repo, ok := b.Repos[*bl.repoID]; ok {
			text = "[" + repo.Name + "] " + text
		}
	}
	if r := []rune(text); len(r) > 255 {
		return string(r[:252]) + "…"
	}
	return text
}

func roundTo(seconds, g, minimum int) int {
	return max(minimum, int(math.Round(float64(seconds)/float64(g)))*g)
}

// ChatDraftMinSeconds: a conversation-day with at least this much chatting becomes a draft by itself.
const ChatDraftMinSeconds = 15 * 60

// chatDrafts are the drafts for the day's long chats: unticketed (an "Always" rule's ticket when there is one), so
// they show in Review and Totals but need a ticket before they are logged. Shorter chats are logged from the Chats
// page.
func (b Builder) chatDrafts(in DayInput, used map[string]bool) []Worklog {
	gran := max(1, b.Settings.GranularityMinutes) * 60
	var out []Worklog
	for _, c := range in.Chats {
		active := c.ActiveSeconds()
		if active < ChatDraftMinSeconds || used[c.Ref()] || len(c.Times) == 0 {
			continue
		}
		var issue *string
		for _, r := range b.Settings.ChatRules {
			if r.ConversationID == c.ConversationID {
				issue = NormalizeKeyString(r.IssueKey)
				break
			}
		}
		out = append(out, Worklog{
			Day: in.Day, Start: c.Times[0], Duration: roundTo(active, gran, gran), IssueKey: issue,
			Description: "Chat: " + c.Name, Source: SourceChat, SourceRef: c.Ref(), Estimated: active, Status: StatusDraft,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// Allocate splits target across blocks proportionally to their estimates, in whole granularity units
// (largest-remainder method, so the day sums exactly to the target). Nil target keeps the estimates, rounded.
func Allocate(weights []int, target *int, g int) []int {
	if len(weights) == 0 {
		return nil
	}
	out := make([]int, len(weights))
	if target == nil {
		for i, w := range weights {
			out[i] = roundTo(w, g, g)
		}
		return out
	}
	units := *target / g
	if units <= 0 {
		return out
	}
	sum := 0
	for _, w := range weights {
		sum += w
	}
	total := float64(max(1, sum))
	exact := make([]float64, len(weights))
	base := make([]int, len(weights))
	remaining := units
	for i, w := range weights {
		exact[i] = float64(w) / total * float64(units)
		base[i] = int(math.Floor(exact[i]))
		remaining -= base[i]
	}
	order := make([]int, len(weights))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return exact[order[a]]-float64(base[order[a]]) > exact[order[b]]-float64(base[order[b]])
	})
	for _, i := range order {
		if remaining <= 0 {
			break
		}
		base[i]++
		remaining--
	}
	// Every block gets at least one unit when there is room, taken from the largest (the first largest on a tie).
	if units >= len(weights) {
		for i := range base {
			if base[i] != 0 {
				continue
			}
			donor := 0
			for j := range base {
				if base[j] > base[donor] {
					donor = j
				}
			}
			if base[donor] > 1 {
				base[donor]--
				base[i] = 1
			}
		}
	}
	for i := range base {
		out[i] = base[i] * g
	}
	return out
}

// Place returns the start of a block and where the next block may begin, skipping meetings and breaks.
func Place(duration int, cursor time.Time, free []Interval) (time.Time, time.Time) {
	remaining := time.Duration(duration) * time.Second
	var start *time.Time
	t := cursor
	for _, slot := range free {
		if !slot.End.After(t) {
			continue
		}
		s := maxTime(slot.Start, t)
		if start == nil {
			start = Ptr(s)
		}
		available := slot.End.Sub(s)
		if remaining <= available {
			return *start, s.Add(remaining)
		}
		remaining -= available
		t = slot.End
	}
	if start == nil {
		start = Ptr(t)
	}
	return *start, t.Add(remaining) // overflows past the last window
}
