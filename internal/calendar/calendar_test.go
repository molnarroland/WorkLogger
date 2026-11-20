package calendar

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

const feed = "BEGIN:VCALENDAR\r\n" +
	"BEGIN:VEVENT\r\nUID:standup\r\nSUMMARY:Daily standup\r\nDTSTART;TZID=Europe/Budapest:20260302T091500\r\n" +
	"DTEND;TZID=Europe/Budapest:20260302T093000\r\nRRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;UNTIL=20260306T235959Z\r\n" +
	"EXDATE;TZID=Europe/Budapest:20260304T091500\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:standup\r\nRECURRENCE-ID;TZID=Europe/Budapest:20260305T091500\r\nSUMMARY:Daily standup (moved)\r\n" +
	"DTSTART;TZID=Europe/Budapest:20260305T100000\r\nDTEND;TZID=Europe/Budapest:20260305T101500\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:review\r\nSUMMARY:PROJ-5 review\\, part 1\r\nDESCRIPTION:Line one\\nLine\r\n  two\r\n" +
	"DTSTART:20260303T130000Z\r\nDURATION:PT1H30M\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:allday\r\nSUMMARY:Holiday\r\nDTSTART;VALUE=DATE:20260303\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:declined\r\nSUMMARY:Declined\r\nATTENDEE;CN=Me;PARTSTAT=DECLINED:mailto:ME@x.hu\r\nDTSTART:20260303T100000Z\r\nDTEND:20260303T110000Z\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:wfh\r\nSUMMARY:Home\r\nTRANSP:TRANSPARENT\r\nDTSTART:20260303T080000Z\r\nDTEND:20260303T160000Z\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:gone\r\nSTATUS:CANCELLED\r\nSUMMARY:Cancelled\r\nDTSTART:20260303T150000Z\r\nDTEND:20260303T160000Z\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseICS(t *testing.T) {
	from, to := ts.At("2026-03-02", "00:00"), ts.At("2026-03-09", "00:00")
	ms := ParseICS(feed, "feed", from, to, ts.Budapest, []string{"me@x.hu"})
	var got []string
	for _, m := range ms {
		got = append(got, ts.Days.Key(m.Start)+" "+ts.Days.HM(m.Start)+"-"+ts.Days.HM(m.End)+" "+m.Title)
	}
	want := []string{
		"2026-03-02 09:15-09:30 Daily standup", "2026-03-03 09:15-09:30 Daily standup", "2026-03-06 09:15-09:30 Daily standup",
		"2026-03-05 10:00-10:15 Daily standup (moved)", "2026-03-03 14:00-15:30 PROJ-5 review, part 1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s", strings.Join(got, "\n"))
	}
	if ms[4].Details != "Line one\nLine two" || ms[4].ID != "review" || ms[0].ID == ms[1].ID {
		t.Errorf("details/ids: %q %s %s", ms[4].Details, ms[0].ID, ms[1].ID)
	}
	if n := len(ParseICS(feed, "feed", from, ts.At("2026-03-03", "00:00"), ts.Budapest, nil)); n != 1 {
		t.Errorf("range cut: %d", n)
	}
}

func TestExpandCountAndInterval(t *testing.T) {
	start := ts.At("2026-03-02", "10:00")
	got := expand(start, "FREQ=DAILY;INTERVAL=2;COUNT=3", nil, start.AddDate(1, 0, 0))
	if len(got) != 3 || ts.Days.Key(got[2]) != "2026-03-06" {
		t.Errorf("%v", got)
	}
	got = expand(start, "FREQ=WEEKLY;INTERVAL=2", nil, ts.At("2026-04-01", "00:00"))
	if len(got) != 3 || ts.Days.Key(got[1]) != "2026-03-16" {
		t.Errorf("%v", got)
	}
}

func TestGoogleMeetings(t *testing.T) {
	h := &ts.FakeHTTP{Handle: func(r *http.Request, body string) (int, string) {
		if strings.Contains(r.URL.String(), "token") {
			v, _ := url.ParseQuery(body)
			if v.Get("grant_type") != "refresh_token" || v.Get("refresh_token") != "R" {
				return 400, "bad"
			}
			return 200, `{"access_token":"A","expires_in":3600}`
		}
		if r.Header.Get("Authorization") != "Bearer A" {
			return 401, ""
		}
		if r.URL.Query().Get("pageToken") == "" {
			return 200, `{"nextPageToken":"p2","items":[
              {"id":"1","summary":"Planning","start":{"dateTime":"2026-03-03T09:00:00+01:00"},"end":{"dateTime":"2026-03-03T10:00:00+01:00"}},
              {"id":"2","summary":"Day off","start":{"date":"2026-03-04"},"end":{"date":"2026-03-05"}},
              {"id":"3","status":"cancelled","start":{"dateTime":"2026-03-03T11:00:00+01:00"},"end":{"dateTime":"2026-03-03T12:00:00+01:00"}}]}`
		}
		return 200, `{"items":[
          {"id":"4","eventType":"focusTime","start":{"dateTime":"2026-03-03T13:00:00+01:00"},"end":{"dateTime":"2026-03-03T14:00:00+01:00"}},
          {"id":"5","summary":"Declined","attendees":[{"self":true,"responseStatus":"declined"}],"start":{"dateTime":"2026-03-03T15:00:00+01:00"},"end":{"dateTime":"2026-03-03T16:00:00+01:00"}},
          {"id":"6","start":{"dateTime":"2026-03-03T16:00:00+01:00"},"end":{"dateTime":"2026-03-03T16:30:00+01:00"}}]}`
	}}
	g := &Google{
		ClientID: "c", ClientSecret: "s", Secrets: ts.NewSecrets(map[string]string{core.SecretGoogleRefresh: "R"}),
		HTTP: h, Clock: ts.FixedClock{T: time.Now()},
	}
	ms, err := g.Meetings(context.Background(), "primary", ts.At("2026-03-01", "00:00"), ts.At("2026-04-01", "00:00"))
	if err != nil || len(ms) != 2 || ms[0].ID != "1" || ms[1].Title != "(no title)" || ms[0].CalendarID != "primary" {
		t.Fatalf("%v %+v", err, ms)
	}
	if !g.Connected() {
		t.Error("connected")
	}
	_ = g.SignOut()
	if g.Connected() {
		t.Error("signOut")
	}
}
