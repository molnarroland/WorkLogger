package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/web"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// fakeSite is a signed-in Google Chat window whose calendar.google.com serves an export zip.
type fakeSite struct {
	*ts.FakePage
	signed bool
}

func (f *fakeSite) Show(context.Context, string) error { return nil }
func (f *fakeSite) SignIn(context.Context) error       { return nil }
func (f *fakeSite) LastSignedIn() *bool                { return &f.signed }
func (f *fakeSite) SignOut(context.Context) error      { return nil }

func TestGoogleCalendarFromTheWeb(t *testing.T) {
	s, _ := newService(t)
	st := s.Settings()
	st.Profiles = []core.Profile{{ID: "googleChat", Kind: core.ProfileGoogleChat, Email: "me@x.hu"}}
	st.MeetingRules = nil
	_, err := s.SaveSettings(st)
	must(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("me@x.hu.ics")
	_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:standup\r\nSUMMARY:INT-12 Daily standup\r\n" +
		"DTSTART;TZID=Europe/Budapest:20260302T091500\r\nDTEND;TZID=Europe/Budapest:20260302T093000\r\n" +
		"RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"))
	other, _ := zw.Create("team@group.calendar.google.com.ics")
	_, _ = other.Write([]byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:x\r\nSUMMARY:Not mine\r\nDTSTART:20260303T100000Z\r\nDTEND:20260303T110000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"))
	must(t, zw.Close())
	z := base64.StdEncoding.EncodeToString(buf.Bytes())
	page := &ts.FakePage{Signed: true, Answer: func(string, map[string]any) any { return `{"zip":"` + z + `"}` }}
	page.Redirect = map[string]string{"https://calendar.google.com/calendar/?authuser=me%40x.hu": "https://calendar.google.com/calendar/u/0/r"}
	s.Sites = map[string]Site{"googleChat": &fakeSite{page, true}}

	if st := s.CalendarStatus(); st.GoogleConnected || !st.GoogleWeb {
		t.Errorf("status: %+v", st)
	}
	must(t, s.FetchGoogleCalendar("2026-03-01", "2026-03-31"))
	got, _ := s.Meetings("2026-03-01", "2026-03-31")
	ms := got.Meetings
	if len(ms) != 22 || ms[0].Title != "INT-12 Daily standup" || *ms[0].IssueKey != "INT-12" {
		t.Fatalf("%d meetings: %+v", len(ms), ms[:min(2, len(ms))])
	}
	must(t, s.Rebuild("2026-03-02", "2026-03-02"))
	m, _ := s.Month("2026-03")
	if len(m.Worklogs) != 1 || m.Worklogs[0].Source != "meeting" || m.Worklogs[0].Duration != 900 {
		t.Errorf("drafts: %+v", m.Worklogs)
	}
}

func TestGoogleChatStopsAtToday(t *testing.T) {
	web.ChatDayPause = 0
	s, rec := newService(t) // today is 2026-03-10
	st := s.Settings()
	st.Profiles = []core.Profile{{ID: "googleChat", Kind: core.ProfileGoogleChat, Email: "me@x.hu"}}
	_, err := s.SaveSettings(st)
	must(t, err)
	page := &ts.FakePage{Signed: true}
	s.Sites = map[string]Site{"googleChat": &fakeSite{page, true}}
	must(t, s.FetchGoogleChat("2026-03-08", "2026-03-31"))
	last := page.Opened[len(page.Opened)-1]
	if len(page.Opened) != 1+3 || !strings.Contains(last, "cdatestart=2026-3-10&cdateend=2026-3-10") {
		t.Errorf("searched past today: %v", page.Opened)
	}
	days := 0
	for _, e := range rec.events {
		if e == EventChatDay {
			days++
		}
	}
	if days != 3 {
		t.Errorf("each day is announced when saved: %v", rec.events)
	}
	if err := s.FetchGoogleChat("2026-04-01", "2026-04-30"); err == nil {
		t.Error("a month that hasn't come yet was fetched")
	}
}

func TestChatDraftsInReview(t *testing.T) {
	s, _ := newService(t)
	_, err := s.ImportChats(`{"source":"google-chat","conversations":[{"id":"dm/B","type":"dm","name":"Bea","messages":[
      {"time":"2026-03-03T09:00:00Z","sender":"me@x","me":true,"text":"a"},
      {"time":"2026-03-03T09:09:00Z","sender":"Bea","me":false,"text":"b"},
      {"time":"2026-03-03T09:18:00Z","sender":"me@x","me":true,"text":"c"}]}]}`)
	must(t, err)
	must(t, s.Rebuild("2026-03-01", "2026-03-31"))
	m, _ := s.Month("2026-03")
	if len(m.Worklogs) != 1 || m.Worklogs[0].Source != "chat" || m.Worklogs[0].IssueKey != nil || m.Worklogs[0].Duration != 15*60 {
		t.Fatalf("an 18-minute chat is a draft needing a ticket: %+v", m.Worklogs)
	}
	chats, _ := s.chatList("2026-03-01", "2026-03-31")
	must(t, s.LogChat(chats[0].ID, "proj-9"))
	must(t, s.Rebuild("2026-03-01", "2026-03-31"))
	m, _ = s.Month("2026-03")
	if len(m.Worklogs) != 1 || *m.Worklogs[0].IssueKey != "PROJ-9" || !m.Worklogs[0].Locked {
		t.Errorf("the ticketed chat is kept by rebuilds: %+v", m.Worklogs)
	}
}
