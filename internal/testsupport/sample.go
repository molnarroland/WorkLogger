package testsupport

import (
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// SampleDay is what the HR record says about one day of the sample month.
type SampleDay struct {
	Day      string
	Expected int
	Label    string
	Absence  bool
}

var sampleCoding = [][2]string{
	{"ORB-214", "Offline mode for saved trips"},
	{"ORB-227", "Faster search index rebuild"},
	{"ORB-231", "Export trips as calendar files"},
	{"NOVA-58", "Dashboard filters and saved views"},
	{"ORB-240", "Push notifications for gate changes"},
	{"NOVA-63", "Usage charts for the admin view"},
}

// SampleMonth is the story dataset (frontend/src/app/api/testing/fixtures.ts sampleMonth): coding, chats and a standup
// every workday, leave on the 9th, an entry without a ticket on the 27th, and the 29th–30th still empty. The
// first half of the month is already in Tempo. Ids count up from 1.
func SampleMonth(month string) ([]SampleDay, []core.Worklog) {
	from, to, ok := Days.Month(month)
	if !ok {
		panic("bad sample month " + month)
	}
	var days []SampleDay
	var logs []core.Worklog
	add := func(day, hm string, minutes int, key, text string, source core.Source, status core.Status, estimated int) {
		w := core.Worklog{
			ID: int64(len(logs) + 1), Day: day, Start: At(day, hm), Duration: minutes * 60, Description: text,
			Source: source, Estimated: minutes * 60, Status: status, Locked: status == core.StatusExported,
		}
		if key != "" {
			w.IssueKey = core.Ptr(key)
		}
		if status != core.StatusDraft {
			w.Comment = text // reviewed and exported entries already have their Tempo comment
		}
		if estimated > 0 {
			w.Estimated = estimated * 60
		}
		logs = append(logs, w)
	}
	// ref links the last entry to its source, as the builder does: the meeting's event id, the chat-day, or the
	// repository of the commits (fixtures.ts sampleRepos: 1 trip-planner for ORB, 2 web-dashboard for NOVA).
	ref := func(sourceRef string) { logs[len(logs)-1].SourceRef = sourceRef }
	repo := func(key string) {
		id := int64(1)
		if strings.HasPrefix(key, "NOVA-") {
			id = 2
		}
		logs[len(logs)-1].RepoID = core.Ptr(id)
	}
	chat := func(day string) { ref("chat:" + day + "|space/orb") }
	for _, day := range Days.Range(from, to) {
		t := Days.Date(day)
		weekday := (int(t.Weekday()) + 6) % 7 // 0 = Monday
		workday := weekday < 5
		d := t.Day()
		leave := workday && d == 9
		sd := SampleDay{Day: day, Absence: leave}
		if workday {
			sd.Expected = 8 * 3600
			sd.Label = "Munkanap"
		}
		if leave {
			sd.Label = "Szabadság"
		}
		days = append(days, sd)
		if !workday || d >= 29 {
			continue
		}
		if leave {
			add(day, "08:00", 480, "LEAVE-1", "Szabadság", core.SourceAbsence, core.StatusExported, 0)
			continue
		}
		status := core.StatusDraft
		if d <= 15 {
			status = core.StatusExported
		}
		key, text := sampleCoding[d%len(sampleCoding)][0], sampleCoding[d%len(sampleCoding)][1]
		add(day, "09:00", 15, "TEAM-12", "Daily standup", core.SourceMeeting, status, 0)
		ref(day + "-standup")
		if d == 27 {
			add(day, "09:15", 195, key, text, core.SourceGit, status, 160)
			repo(key)
			add(day, "12:30", 30, "ORB-214", "Trip planner channel: release questions", core.SourceChat, status, 0)
			chat(day)
			add(day, "13:00", 240, "", "Code review", core.SourceManual, core.StatusDraft, 0)
			continue
		}
		if weekday == 1 {
			add(day, "09:15", 60, "TEAM-7", "Sprint planning", core.SourceMeeting, status, 0)
			ref(day + "-planning")
			add(day, "10:15", 375, key, text, core.SourceGit, status, 300)
		} else {
			add(day, "09:15", 435, key, text, core.SourceGit, status, 380)
		}
		repo(key)
		add(day, "16:30", 30, "ORB-214", "Trip planner channel: release questions", core.SourceChat, status, 0)
		chat(day)
	}
	return days, logs
}

// SampleSessions is the story month's work sessions (fixtures.ts sampleSessions): 08:30–17:00 with a lunch
// break on workdays before the 29th, now and then a longer day so net and HR differ, and leave as a record only.
func SampleSessions(month string) ([]core.WorkSession, []core.DayRecord) {
	days, _ := SampleMonth(month)
	var sessions []core.WorkSession
	var records []core.DayRecord
	for _, d := range days {
		n := Days.Date(d.Day).Day()
		if d.Absence {
			records = append(records, core.DayRecord{Day: d.Day, Status: core.DayLeave, Label: d.Label, ReportedWorked: core.Ptr(8 * 3600)})
			continue
		}
		if d.Expected == 0 || n >= 29 {
			continue
		}
		end := "17:00"
		if n%7 == 2 {
			end = "17:40"
		}
		sessions = append(sessions,
			core.WorkSession{ID: int64(len(sessions) + 1), Day: d.Day, Start: At(d.Day, "08:30"), End: At(d.Day, end), Kind: core.SessionWork, Label: "Munkaidő 1"},
			core.WorkSession{ID: int64(len(sessions) + 2), Day: d.Day, Start: At(d.Day, "12:00"), End: At(d.Day, "12:30"), Kind: core.SessionPause, Label: "Ebéd"},
		)
		records = append(records, core.DayRecord{Day: d.Day, Status: core.DayWorkday, Label: "Munkanap (Home Office)", ReportedWorked: core.Ptr(8 * 3600)})
	}
	return sessions, records
}

// SampleMeetings is the story month's calendar (fixtures.ts sampleMeetings): a standup every workday that isn't
// leave, sprint planning on Tuesdays, a design review on Wednesdays, a 1:1 on Thursdays and a lunch talk on
// Fridays.
func SampleMeetings(month string) []core.Meeting {
	days, _ := SampleMonth(month)
	var out []core.Meeting
	for _, d := range days {
		if d.Expected == 0 || d.Absence {
			continue
		}
		m := func(id, title, from, to string) core.Meeting {
			return core.Meeting{ID: d.Day + "-" + id, CalendarID: "primary", Title: title, Start: At(d.Day, from), End: At(d.Day, to)}
		}
		out = append(out, m("standup", "Daily standup", "09:00", "09:15"))
		switch Days.Date(d.Day).Weekday() {
		case time.Tuesday:
			out = append(out, m("planning", "Sprint planning", "09:15", "10:15"))
		case time.Wednesday:
			out = append(out, m("design", "Design review: booking flow", "15:00", "15:45"))
		case time.Thursday:
			out = append(out, m("one-on-one", "1:1 with Mira", "14:00", "14:30"))
		case time.Friday:
			out = append(out, m("lunch", "Lunch & learn: Accessibility", "12:00", "13:00"))
		}
	}
	return out
}
