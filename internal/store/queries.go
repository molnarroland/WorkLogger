package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// ── Repositories & projects ─────────────────────────────────────────────────

func (s *Store) Repos() ([]core.Repo, error) {
	rows, err := s.query("SELECT id, path, name, issue_key, project_key, enabled FROM repos ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Repo
	for rows.Next() {
		var r core.Repo
		var issue, project sql.NullString
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &issue, &project, &r.Enabled); err != nil {
			return nil, err
		}
		r.IssueKey, r.ProjectKey = strPtr(issue), strPtr(project)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddRepo(path, name string) error {
	_, err := s.exec("INSERT OR IGNORE INTO repos(path, name) VALUES(?, ?)", path, name)
	return err
}

func (s *Store) UpdateRepo(r core.Repo) error {
	_, err := s.exec("UPDATE repos SET name = ?, issue_key = ?, project_key = ?, enabled = ? WHERE id = ?",
		r.Name, nullString(r.IssueKey), nullString(r.ProjectKey), r.Enabled, r.ID)
	return err
}

func (s *Store) DeleteRepo(id int64) error {
	_, err := s.exec("DELETE FROM repos WHERE id = ?", id)
	return err
}

func (s *Store) Projects() ([]core.ProjectMapping, error) {
	rows, err := s.query("SELECT key, default_issue FROM projects ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.ProjectMapping
	for rows.Next() {
		var p core.ProjectMapping
		var d sql.NullString
		if err := rows.Scan(&p.Key, &d); err != nil {
			return nil, err
		}
		p.DefaultIssue = strPtr(d)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpsertProject(p core.ProjectMapping) error {
	_, err := s.exec(`INSERT INTO projects(key, default_issue) VALUES(?, ?)
        ON CONFLICT(key) DO UPDATE SET default_issue = excluded.default_issue`, p.Key, nullString(p.DefaultIssue))
	return err
}

func (s *Store) DeleteProject(key string) error {
	_, err := s.exec("DELETE FROM projects WHERE key = ?", key)
	return err
}

// ── Commits ─────────────────────────────────────────────────────────────────

func (s *Store) SaveCommits(list []core.StoredCommit) error {
	for _, c := range list {
		if _, err := s.exec(`INSERT INTO commits(repo_id, hash, ts, author, email, subject, body, ref, files, insertions, deletions)
            VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(repo_id, hash) DO UPDATE SET
                ref = excluded.ref, files = excluded.files,
                insertions = excluded.insertions, deletions = excluded.deletions`,
			c.RepoID, c.Hash, unix(c.Date), c.Author, c.Email, c.Subject, c.Body, c.Ref,
			c.FilesChanged, c.Insertions, c.Deletions); err != nil {
			return err
		}
	}
	return nil
}

// CommitDays is your stored commits per repository in [from, to): how many on each day ("YYYY-MM-DD").
func (s *Store) CommitDays(from, to time.Time) (map[int64]map[string]int, error) {
	rows, err := s.query("SELECT repo_id, ts FROM commits WHERE ts >= ? AND ts < ?", unix(from), unix(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]int{}
	for rows.Next() {
		var id, ts int64
		if err := rows.Scan(&id, &ts); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]int{}
		}
		out[id][s.Days.Key(fromUnix(ts))]++
	}
	return out, rows.Err()
}

// Commits of enabled repositories on a day.
func (s *Store) Commits(day string) ([]core.StoredCommit, error) {
	start, end := s.Days.Bounds(day)
	rows, err := s.query(`SELECT c.repo_id, c.hash, c.ts, COALESCE(c.author, ''), COALESCE(c.email, ''), c.subject, c.body,
        c.ref, c.files, c.insertions, c.deletions
        FROM commits c JOIN repos r ON r.id = c.repo_id
        WHERE r.enabled = 1 AND c.ts >= ? AND c.ts < ? ORDER BY c.ts`, unix(start), unix(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.StoredCommit
	for rows.Next() {
		var c core.StoredCommit
		var ts int64
		if err := rows.Scan(&c.RepoID, &c.Hash, &ts, &c.Author, &c.Email, &c.Subject, &c.Body, &c.Ref,
			&c.FilesChanged, &c.Insertions, &c.Deletions); err != nil {
			return nil, err
		}
		c.Date = fromUnix(ts)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ── Work sessions and day records ───────────────────────────────────────────

func (s *Store) scanSessions(query string, args ...any) ([]core.WorkSession, error) {
	rows, err := s.query("SELECT id, day, start_ts, end_ts, kind, label FROM sessions WHERE "+query+" ORDER BY start_ts", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.WorkSession
	for rows.Next() {
		var w core.WorkSession
		var start, end int64
		var kind string
		if err := rows.Scan(&w.ID, &w.Day, &start, &end, &kind, &w.Label); err != nil {
			return nil, err
		}
		w.Start, w.End, w.Kind = fromUnix(start), fromUnix(end), core.SessionKind(kind)
		if w.Kind != core.SessionPause {
			w.Kind = core.SessionWork
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) Sessions(day string) ([]core.WorkSession, error) {
	return s.scanSessions("day = ?", day)
}

func (s *Store) SessionsRange(fromDay, toDay string) ([]core.WorkSession, error) {
	return s.scanSessions("day >= ? AND day <= ?", fromDay, toDay)
}

// ImportTimesheet replaces the sessions and day records of every day in the import (days without sessions, e.g.
// leave, are cleared too).
func (s *Store) ImportTimesheet(t core.ParsedTimesheet) error {
	return s.Tx(func(tx *Store) error {
		for _, day := range t.DayKeys() {
			if _, err := tx.exec("DELETE FROM sessions WHERE day = ?", day); err != nil {
				return err
			}
		}
		for _, w := range t.Sessions {
			if _, err := tx.exec("INSERT INTO sessions(day, start_ts, end_ts, kind, label) VALUES(?, ?, ?, ?, ?)",
				w.Day, unix(w.Start), unix(w.End), string(w.Kind), w.Label); err != nil {
				return err
			}
		}
		for _, d := range t.Days {
			if _, err := tx.exec(`INSERT INTO day_info(day, status, label, reported_worked, reported_breaks) VALUES(?, ?, ?, ?, ?)
                ON CONFLICT(day) DO UPDATE SET status = excluded.status, label = excluded.label,
                    reported_worked = excluded.reported_worked, reported_breaks = excluded.reported_breaks`,
				d.Day, string(d.Status), d.Label, nullIntPtr(d.ReportedWorked), nullIntPtr(d.ReportedBreaks)); err != nil {
				return err
			}
		}
		return nil
	})
}

func nullIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func (s *Store) DeleteSessions(fromDay, toDay string) error {
	return s.Tx(func(tx *Store) error {
		if _, err := tx.exec("DELETE FROM sessions WHERE day >= ? AND day <= ?", fromDay, toDay); err != nil {
			return err
		}
		_, err := tx.exec("DELETE FROM day_info WHERE day >= ? AND day <= ?", fromDay, toDay)
		return err
	})
}

func (s *Store) DayRecords(fromDay, toDay string) ([]core.DayRecord, error) {
	rows, err := s.query(`SELECT day, status, label, reported_worked, reported_breaks FROM day_info
        WHERE day >= ? AND day <= ? ORDER BY day`, fromDay, toDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.DayRecord
	for rows.Next() {
		var d core.DayRecord
		var status string
		var worked, breaks sql.NullInt64
		if err := rows.Scan(&d.Day, &status, &d.Label, &worked, &breaks); err != nil {
			return nil, err
		}
		d.Status = core.DayStatus(status)
		switch d.Status {
		case core.DayWorkday, core.DayRestDay, core.DayPublicHoliday, core.DayLeave:
		default:
			d.Status = core.DayOther
		}
		if worked.Valid {
			d.ReportedWorked = core.Ptr(int(worked.Int64))
		}
		if breaks.Valid {
			d.ReportedBreaks = core.Ptr(int(breaks.Int64))
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DayRecordMap is DayRecords keyed by day.
func (s *Store) DayRecordMap(fromDay, toDay string) (map[string]*core.DayRecord, error) {
	list, err := s.DayRecords(fromDay, toDay)
	out := map[string]*core.DayRecord{}
	for i := range list {
		out[list[i].Day] = &list[i]
	}
	return out, err
}

// ── Meetings ────────────────────────────────────────────────────────────────

func (s *Store) scanMeetings(query string, args ...any) ([]core.Meeting, error) {
	rows, err := s.query("SELECT event_id, calendar_id, title, details, start_ts, end_ts, skipped FROM meetings WHERE "+
		query+" ORDER BY start_ts", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Meeting
	for rows.Next() {
		var m core.Meeting
		var start, end int64
		if err := rows.Scan(&m.ID, &m.CalendarID, &m.Title, &m.Details, &start, &end, &m.Skipped); err != nil {
			return nil, err
		}
		m.Start, m.End = fromUnix(start), fromUnix(end)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) MeetingsOn(day string) ([]core.Meeting, error) {
	start, end := s.Days.Bounds(day)
	return s.Meetings(start, end)
}

func (s *Store) Meetings(from, to time.Time) ([]core.Meeting, error) {
	return s.scanMeetings("start_ts >= ? AND start_ts < ?", unix(from), unix(to))
}

// SyncMeetings stores the events of the given calendars for an interval. Events deleted in the calendar, and events
// of calendars that are no longer read, are removed. Keeps the "skipped" choice.
func (s *Store) SyncMeetings(from, to time.Time, calendarIDs, activeCalendarIDs map[string]bool, events []core.Meeting) error {
	return s.Tx(func(tx *Store) error {
		ids := map[string]bool{}
		for _, e := range events {
			ids[e.ID] = true
		}
		existing, err := tx.Meetings(from, to)
		if err != nil {
			return err
		}
		for _, m := range existing {
			stale := calendarIDs[m.CalendarID] && !ids[m.ID]
			if stale || !activeCalendarIDs[m.CalendarID] {
				if _, err := tx.exec("DELETE FROM meetings WHERE event_id = ?", m.ID); err != nil {
					return err
				}
			}
		}
		for _, e := range events {
			if _, err := tx.exec(`INSERT INTO meetings(event_id, calendar_id, title, details, start_ts, end_ts, skipped)
                VALUES(?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(event_id) DO UPDATE SET title = excluded.title, details = excluded.details,
                    start_ts = excluded.start_ts, end_ts = excluded.end_ts`,
				e.ID, e.CalendarID, e.Title, e.Details, unix(e.Start), unix(e.End), e.Skipped); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SetMeetingSkipped(id string, skipped bool) error {
	_, err := s.exec("UPDATE meetings SET skipped = ? WHERE event_id = ?", skipped, id)
	return err
}

// ── Worklogs ────────────────────────────────────────────────────────────────

const worklogCols = "id, day, start_ts, duration, issue_key, description, comment, source, source_ref, repo_id, estimated, status, locked, tempo_id"

func (s *Store) scanWorklogs(query string, args ...any) ([]core.Worklog, error) {
	rows, err := s.query("SELECT "+worklogCols+" FROM worklogs WHERE "+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Worklog
	for rows.Next() {
		var w core.Worklog
		var start int64
		var issue sql.NullString
		var repo, tempo sql.NullInt64
		var source, status string
		if err := rows.Scan(&w.ID, &w.Day, &start, &w.Duration, &issue, &w.Description, &w.Comment, &source, &w.SourceRef,
			&repo, &w.Estimated, &status, &w.Locked, &tempo); err != nil {
			return nil, err
		}
		w.Start, w.IssueKey, w.RepoID, w.TempoID = fromUnix(start), strPtr(issue), intPtr(repo), intPtr(tempo)
		w.Source, w.Status = core.Source(source), core.Status(status)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) Worklogs(fromDay, toDay string) ([]core.Worklog, error) {
	return s.scanWorklogs("day >= ? AND day <= ? ORDER BY day, start_ts", fromDay, toDay)
}

func (s *Store) WorklogsOn(day string) ([]core.Worklog, error) { return s.Worklogs(day, day) }

func (s *Store) WorklogsByID(ids []int64) ([]core.Worklog, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return s.scanWorklogs("id IN (?"+strings.Repeat(",?", len(ids)-1)+") ORDER BY day, start_ts", args...)
}

// WorklogBySourceRef is the entry made from a source (e.g. a chat-day), if any.
func (s *Store) WorklogBySourceRef(source core.Source, ref string) (*core.Worklog, error) {
	list, err := s.scanWorklogs("source = ? AND source_ref = ? LIMIT 1", string(source), ref)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

func (s *Store) InsertWorklog(w core.Worklog) (int64, error) {
	res, err := s.exec(`INSERT INTO worklogs(day, start_ts, duration, issue_key, description, comment, source, source_ref,
        repo_id, estimated, status, locked, tempo_id) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.Day, unix(w.Start), w.Duration, nullString(w.IssueKey), w.Description, w.Comment, string(w.Source), w.SourceRef,
		nullInt(w.RepoID), w.Estimated, string(w.Status), w.Locked, nullInt(w.TempoID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateWorklog(w core.Worklog) error {
	_, err := s.exec(`UPDATE worklogs SET day = ?, start_ts = ?, duration = ?, issue_key = ?, description = ?,
        comment = ?, status = ?, locked = ?, tempo_id = ? WHERE id = ?`,
		w.Day, unix(w.Start), w.Duration, nullString(w.IssueKey), w.Description, w.Comment, string(w.Status), w.Locked,
		nullInt(w.TempoID), w.ID)
	return err
}

// DeleteWorklog removes an entry; ErrNotFound when there is none with that id.
func (s *Store) DeleteWorklog(id int64) error {
	res, err := s.exec("DELETE FROM worklogs WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("worklog %d: %w", id, ErrNotFound)
	}
	return nil
}

// DeleteRegeneratable removes generated drafts so they can be rebuilt. Locked, reviewed, exported and manual
// entries stay; so do chats logged to a ticket by hand (those are locked).
func (s *Store) DeleteRegeneratable(fromDay, toDay string) error {
	_, err := s.exec(`DELETE FROM worklogs WHERE day >= ? AND day <= ? AND locked = 0 AND status = 'draft'
        AND source IN ('git', 'meeting', 'absence', 'period', 'chat', 'jira', 'confluence', 'email')`, fromDay, toDay)
	return err
}

// ── Jira issue id cache (Tempo API v4 needs numeric issue ids), per Jira site ──

func (s *Store) CachedIssueID(key, site string) (int64, bool) {
	var id int64
	err := s.q.QueryRowContext(s.context(), "SELECT issue_id FROM issue_cache WHERE site = ? AND key = ?", site, key).Scan(&id)
	return id, err == nil
}

// CachedIssueKey is the issue key for a Jira issue id seen before (pushing or reading Tempo).
func (s *Store) CachedIssueKey(id int64, site string) (string, bool) {
	var key string
	err := s.q.QueryRowContext(s.context(), "SELECT key FROM issue_cache WHERE site = ? AND issue_id = ?", site, id).Scan(&key)
	return key, err == nil
}

func (s *Store) CacheIssueID(key string, id int64, site string) error {
	_, err := s.exec("INSERT OR REPLACE INTO issue_cache(site, key, issue_id) VALUES(?, ?, ?)", site, key, id)
	return err
}

// ── Settings ────────────────────────────────────────────────────────────────

// LoadSettings reads the stored settings merged with the defaults; defaults when there are none or they don't decode.
func (s *Store) LoadSettings(newID func() string) core.Settings {
	var raw string
	if err := s.q.QueryRowContext(s.context(), "SELECT value FROM settings WHERE key = 'app'").Scan(&raw); err == nil {
		if st, ok := core.DecodeSettings([]byte(raw), newID); ok {
			return st
		}
	}
	st := core.DefaultSettings()
	st.KnownModules = append([]core.Module(nil), core.AllModules...)
	return st
}

func (s *Store) SaveSettings(st core.Settings) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.exec(`INSERT INTO settings(key, value) VALUES('app', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(data))
	return err
}

// ── Chats ───────────────────────────────────────────────────────────────────

func ms(t time.Time) int64 { return int64(math.Round(float64(t.UnixNano()) / 1e6)) }

// ImportChats adds the export's conversations and messages; messages already imported are kept once (text and
// files refreshed). Returns the number of new or changed messages.
func (s *Store) ImportChats(e core.ChatExport) (int, error) {
	added := 0
	err := s.Tx(func(tx *Store) error {
		for _, c := range e.Conversations {
			if _, err := tx.exec(`INSERT INTO chat_conversations(id, source, type, name) VALUES(?, ?, ?, ?)
                ON CONFLICT(id) DO UPDATE SET type = excluded.type, name = COALESCE(NULLIF(excluded.name, ''), name)`,
				c.ID, e.Source, c.Type, c.DisplayName()); err != nil {
				return err
			}
			for _, m := range c.Messages {
				sender := ""
				if m.Sender != nil {
					sender = *m.Sender
				}
				res, err := tx.exec(`INSERT INTO chat_messages(conversation_id, ts_ms, sender, me, text, files) VALUES(?, ?, ?, ?, ?, ?)
                    ON CONFLICT(conversation_id, ts_ms, sender) DO UPDATE SET text = excluded.text, files = excluded.files
                    WHERE text != excluded.text OR files != excluded.files`,
					c.ID, ms(m.Time), sender, m.Me, m.Body(), strings.Join(m.Files(), "\n"))
				if err != nil {
					return err
				}
				n, _ := res.RowsAffected()
				added += int(n)
			}
		}
		return nil
	})
	return added, err
}

// ChatDays is the conversations with messages in [from, to), split per day.
func (s *Store) ChatDays(from, to time.Time, gap time.Duration) ([]core.ChatDay, error) {
	rows, err := s.query(`SELECT m.conversation_id, m.ts_ms, c.name, c.type FROM chat_messages m
        JOIN chat_conversations c ON c.id = m.conversation_id
        WHERE m.ts_ms >= ? AND m.ts_ms < ? ORDER BY m.ts_ms`, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byKey := map[string]*core.ChatDay{}
	for rows.Next() {
		var id, name, typ string
		var tms int64
		if err := rows.Scan(&id, &tms, &name, &typ); err != nil {
			return nil, err
		}
		t := time.UnixMilli(tms)
		day := s.Days.Key(t)
		d, ok := byKey[id+"|"+day]
		if !ok {
			if name == "" || name == id { // older imports stored the id
				name = core.UnknownName(id)
			}
			d = &core.ChatDay{ConversationID: id, Name: name, Type: typ, Day: day, Gap: gap}
			byKey[id+"|"+day] = d
		}
		d.Times = append(d.Times, t)
	}
	out := make([]core.ChatDay, 0, len(byKey))
	for _, d := range byKey {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].Times[0].Before(out[j].Times[0])
	})
	return out, rows.Err()
}

// ChatLines is one conversation's messages on one day, oldest first.
func (s *Store) ChatLines(conversationID, day string) ([]core.ChatLine, error) {
	start, end := s.Days.Bounds(day)
	rows, err := s.query(`SELECT ts_ms, sender, me, text, files FROM chat_messages
        WHERE conversation_id = ? AND ts_ms >= ? AND ts_ms < ? ORDER BY ts_ms`, conversationID, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.ChatLine
	for rows.Next() {
		var l core.ChatLine
		var tms int64
		var files string
		if err := rows.Scan(&tms, &l.Sender, &l.Me, &l.Text, &files); err != nil {
			return nil, err
		}
		l.Time = time.UnixMilli(tms)
		for _, f := range strings.Split(files, "\n") {
			if f != "" {
				l.Files = append(l.Files, f)
			}
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) DeleteChats(from, to time.Time) error {
	_, err := s.exec("DELETE FROM chat_messages WHERE ts_ms >= ? AND ts_ms < ?", from.UnixMilli(), to.UnixMilli())
	return err
}

// DeleteSources deletes a day's commits, meetings and chat messages (what the drafts were built from), and
// returns how many rows went.
func (s *Store) DeleteSources(day string) (int, error) {
	start, end := s.Days.Bounds(day)
	total := 0
	for _, q := range []struct {
		sql      string
		from, to int64
	}{
		{`DELETE FROM commits WHERE ts >= ? AND ts < ?`, unix(start), unix(end)},
		{`DELETE FROM meetings WHERE start_ts >= ? AND start_ts < ?`, unix(start), unix(end)},
		{`DELETE FROM chat_messages WHERE ts_ms >= ? AND ts_ms < ?`, start.UnixMilli(), end.UnixMilli()},
		{`DELETE FROM jira_activity WHERE ts >= ? AND ts < ?`, unix(start), unix(end)},
		{`DELETE FROM confluence_activity WHERE ts >= ? AND ts < ?`, unix(start), unix(end)},
		{`DELETE FROM sent_emails WHERE ts >= ? AND ts < ?`, unix(start), unix(end)},
	} {
		r, err := s.exec(q.sql, q.from, q.to)
		if err != nil {
			return total, err
		}
		n, _ := r.RowsAffected()
		total += int(n)
	}
	return total, nil
}

// Vacuum rebuilds the file without the space deleted rows left behind.
func (s *Store) Vacuum() error {
	_, err := s.exec(`VACUUM`)
	return err
}

// ── Jira activity ───────────────────────────────────────────────────────────

// JiraActivity is one moment you did something on a ticket.
type JiraActivity struct {
	IssueKey, Summary string
	At                time.Time
}

// SyncJiraActivity stores your Jira activity in [from, to). With replace, what was stored there before goes first
// (activity undone in Jira disappears); without it (some Jira profile could not be read) it is only added to.
func (s *Store) SyncJiraActivity(from, to time.Time, list []JiraActivity, replace bool) error {
	return s.Tx(func(tx *Store) error {
		if replace {
			if _, err := tx.exec("DELETE FROM jira_activity WHERE ts >= ? AND ts < ?", unix(from), unix(to)); err != nil {
				return err
			}
		}
		for _, a := range list {
			if _, err := tx.exec(`INSERT INTO jira_activity(issue_key, ts, summary) VALUES(?, ?, ?)
                ON CONFLICT(issue_key, ts) DO UPDATE SET summary = excluded.summary`, a.IssueKey, unix(a.At), a.Summary); err != nil {
				return err
			}
		}
		return nil
	})
}

// JiraTickets is the day's Jira activity per ticket, in the order you started on them.
func (s *Store) JiraTickets(day string) ([]core.JiraTicket, error) {
	start, end := s.Days.Bounds(day)
	rows, err := s.query(`SELECT issue_key, MAX(summary), MIN(ts), MAX(ts) FROM jira_activity
        WHERE ts >= ? AND ts < ? GROUP BY issue_key ORDER BY MIN(ts), issue_key`, unix(start), unix(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.JiraTicket
	for rows.Next() {
		t := core.JiraTicket{Day: day}
		var first, last int64
		if err := rows.Scan(&t.IssueKey, &t.Summary, &first, &last); err != nil {
			return nil, err
		}
		t.First, t.Last = fromUnix(first), fromUnix(last)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ── Confluence activity ─────────────────────────────────────────────────────

// ConfluenceActivity is one moment you worked on a page.
type ConfluenceActivity struct {
	PageID, Title, Space string
	At                   time.Time
}

// SyncConfluenceActivity stores your Confluence work in [from, to), like SyncJiraActivity.
func (s *Store) SyncConfluenceActivity(from, to time.Time, list []ConfluenceActivity, replace bool) error {
	return s.Tx(func(tx *Store) error {
		if replace {
			if _, err := tx.exec("DELETE FROM confluence_activity WHERE ts >= ? AND ts < ?", unix(from), unix(to)); err != nil {
				return err
			}
		}
		for _, a := range list {
			if _, err := tx.exec(`INSERT INTO confluence_activity(page_id, ts, title, space) VALUES(?, ?, ?, ?)
                ON CONFLICT(page_id, ts) DO UPDATE SET title = excluded.title, space = excluded.space`,
				a.PageID, unix(a.At), a.Title, a.Space); err != nil {
				return err
			}
		}
		return nil
	})
}

// ConfluencePages is the day's Confluence work per page, in the order you started on them.
func (s *Store) ConfluencePages(day string) ([]core.ConfluencePage, error) {
	start, end := s.Days.Bounds(day)
	rows, err := s.query(`SELECT page_id, MAX(title), MAX(space), MIN(ts), MAX(ts) FROM confluence_activity
        WHERE ts >= ? AND ts < ? GROUP BY page_id ORDER BY MIN(ts), page_id`, unix(start), unix(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.ConfluencePage
	for rows.Next() {
		p := core.ConfluencePage{Day: day}
		var first, last int64
		if err := rows.Scan(&p.PageID, &p.Title, &p.Space, &first, &last); err != nil {
			return nil, err
		}
		p.First, p.Last = fromUnix(first), fromUnix(last)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Sent mail ───────────────────────────────────────────────────────────────

// SentEmail is one mail you sent.
type SentEmail struct {
	ID, Subject string
	At          time.Time
}

// SyncSentEmails stores the mail you sent in [from, to), like SyncJiraActivity.
func (s *Store) SyncSentEmails(from, to time.Time, list []SentEmail, replace bool) error {
	return s.Tx(func(tx *Store) error {
		if replace {
			if _, err := tx.exec("DELETE FROM sent_emails WHERE ts >= ? AND ts < ?", unix(from), unix(to)); err != nil {
				return err
			}
		}
		for _, m := range list {
			if _, err := tx.exec(`INSERT INTO sent_emails(id, ts, subject) VALUES(?, ?, ?)
                ON CONFLICT(id) DO UPDATE SET ts = excluded.ts, subject = excluded.subject`, m.ID, unix(m.At), m.Subject); err != nil {
				return err
			}
		}
		return nil
	})
}

// SentEmails is the mail you sent in [from, to), oldest first.
func (s *Store) SentEmails(from, to time.Time) ([]SentEmail, error) {
	rows, err := s.query(`SELECT id, subject, ts FROM sent_emails WHERE ts >= ? AND ts < ? ORDER BY ts, id`, unix(from), unix(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SentEmail
	for rows.Next() {
		var m SentEmail
		var at int64
		if err := rows.Scan(&m.ID, &m.Subject, &at); err != nil {
			return nil, err
		}
		m.At = fromUnix(at)
		out = append(out, m)
	}
	return out, rows.Err()
}

// EmailThreads is the day's sent mail by thread (subject without Re:/Fwd:), in the order you started on them.
func (s *Store) EmailThreads(day string) ([]core.EmailThread, error) {
	start, end := s.Days.Bounds(day)
	list, err := s.SentEmails(start, end)
	if err != nil {
		return nil, err
	}
	var out []core.EmailThread
	index := map[string]int{}
	for _, m := range list {
		subject := core.ThreadSubject(m.Subject)
		k := strings.ToLower(subject)
		i, ok := index[k]
		if !ok {
			i = len(out)
			index[k] = i
			out = append(out, core.EmailThread{Day: day, Subject: subject, First: m.At})
		}
		out[i].Count++
		out[i].Last = m.At
	}
	return out, nil
}
