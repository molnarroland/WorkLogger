package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

var ctx = context.Background()

func p[T any](v T) *T { return &v }

func TestHRmasterAddresses(t *testing.T) {
	for in, want := range map[string]string{
		"acme.hrmaster.hu": "https://acme.hrmaster.hu", " https://acme.hrmaster.hu/Datacenter/#/x ": "https://acme.hrmaster.hu",
	} {
		if got, ok := NormalizeHRmaster(in); !ok || got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
	for _, in := range []string{"", "nonsense"} {
		if _, ok := NormalizeHRmaster(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
	if WorkingTimeURL("https://acme.hrmaster.hu") != "https://acme.hrmaster.hu/Datacenter/#/employeeWorkingTime/own" {
		t.Error("workingTimePage")
	}
	if got := Months(ts.Days, "2026-11-20", "2027-01-05"); !reflect.DeepEqual(got, []Month{{2026, 11}, {2026, 12}, {2027, 1}}) {
		t.Errorf("rangeIsReadMonthByMonth: %v", got)
	}
}

func TestChatHelpers(t *testing.T) {
	u := SearchURL("1", [][2]string{{"cfrom", "me@x.hu"}}, ts.At("2026-03-01", "00:00"), ts.At("2026-03-07", "00:00"))
	if u != "https://chat.google.com/u/1/app/search?q=&cfrom=me@x.hu&cdatestart=2026-3-1&cdateend=2026-3-7&cdateoption=custom&cmembership=1" {
		t.Errorf("searchURLMatchesTheExtension: %s", u)
	}
	e := ChatExportOf([]RawMessage{
		{ID: "2", GID: p("space/A"), TS: 2000, Sender: p("Bea"), Text: "yo", Extra: p("PROJ | x | ,")},
		{ID: "1", GID: p("space/A"), TS: 1000, Sender: p("Ön"), Me: true, Text: "hi", Extra: p("PROJ | x | ,")},
		{ID: "3", GID: p("dm/B"), Conv: p("Bea"), TS: 1500, OneOnOne: true, Me: true, Text: "ping"},
	}, "me@x.hu")
	if e.Conversations[0].ID != "space/A" || e.Conversations[1].ID != "dm/B" || e.Conversations[0].Messages[0].Text != "hi" ||
		e.Conversations[0].Messages[1].Text != "yo" || *e.Conversations[0].Messages[0].Sender != "me@x.hu" ||
		e.Conversations[0].DisplayName() != "PROJ" || e.Conversations[1].DisplayName() != "Bea" {
		t.Errorf("exportGroupsConversationsAndFillsMyEmail: %+v", e)
	}
}

func msg(id, gid, day, hm string, me bool) string {
	g := "null"
	if gid != "" {
		g = `"` + gid + `"`
	}
	return fmt.Sprintf(`{"id":"%s","gid":%s,"conv":"PROJ","ts":%d,"oneOnOne":false,"sender":"Bea","me":%v,"text":"t%s"}`,
		id, g, ts.At(day, hm).UnixMilli(), me, id)
}

// collect runs CollectGoogleChat and gathers what each day handed over.
func collect(page *ts.FakePage, from, to string, progress func(string)) (map[string]core.ChatExport, []string, error) {
	ChatDayPause = 0
	got := map[string]core.ChatExport{}
	var order []string
	err := CollectGoogleChat(ctx, page, ts.Days, "me@x.hu", "", from, to, progress, func(day string, e core.ChatExport) error {
		got[day] = e
		order = append(order, day)
		return nil
	})
	return got, order, err
}

func TestChatFlow(t *testing.T) {
	page := &ts.FakePage{Signed: true, Redirect: map[string]string{"https://chat.google.com/": "https://chat.google.com/u/1/app/home"}}
	page.Answer = func(url string, args map[string]any) any {
		switch {
		case args["startMs"] == ts.At("2026-03-03", "00:00").UnixMilli():
			// my messages (the search request): one from the day, one from another day
			return `{"messages":[` + msg("1", "space/A", "2026-03-03", "09:00", true) + "," + msg("x", "space/A", "2026-03-05", "09:00", true) + "]}"
		case args["startMs"] != nil:
			return `{"messages":[]}`
		case strings.Contains(url, "cdatestart=2026-3-4"):
			// mentions: a duplicate of a day-3 message (other day: dropped), one more, and one without a conversation id
			return "[" + msg("1", "space/A", "2026-03-03", "09:00", true) + "," + msg("2", "dm/B", "2026-03-04", "10:00", false) + "," +
				msg("3", "", "2026-03-04", "11:00", true) + "]"
		}
		return "[]"
	}
	got, order, err := collect(page, "2026-03-03", "2026-03-04", func(string) {})
	if err != nil || !reflect.DeepEqual(order, []string{"2026-03-03", "2026-03-04"}) || len(page.Opened) != 3 ||
		!strings.Contains(page.Opened[2], "cdatestart=2026-3-4&cdateend=2026-3-4") {
		t.Fatalf("a request and a mentions search per day, each day handed over: %v %v %v", err, order, page.Opened)
	}
	if rpc := page.Runs[0]; rpc["account"] != "1" || rpc["endMs"] != ts.At("2026-03-04", "00:00").UnixMilli() {
		t.Errorf("the request covers the day: %v", rpc)
	}
	for _, u := range page.Opened[1:] {
		if !strings.HasPrefix(u, "https://chat.google.com/u/1/app/search?") {
			t.Errorf("opened %s", u)
		}
	}
	texts := func(e core.ChatExport) (out []string) {
		for _, c := range e.Conversations {
			for _, m := range c.Messages {
				out = append(out, m.Text)
			}
		}
		return out
	}
	d3, d4 := got["2026-03-03"], got["2026-03-04"]
	if !reflect.DeepEqual(texts(d3), []string{"t1"}) || *d3.Conversations[0].Messages[0].Sender != "me@x.hu" ||
		!reflect.DeepEqual(texts(d4), []string{"t2"}) || d4.Conversations[0].ID != "dm/B" {
		t.Errorf("keeps each day's messages, once: %+v %+v", d3, d4)
	}

	page = &ts.FakePage{Signed: true, Progress: []string{fmt.Sprintf(`{"n":3,"ts":%d}`, ts.At("2026-03-03", "14:32").UnixMilli())}}
	var lines []string
	_, _, _ = collect(page, "2026-03-03", "2026-03-03", func(l string) { lines = append(lines, l) })
	mine, mentions := false, false
	for _, l := range lines {
		mine = mine || strings.Contains(l, "(1 of 1) · my messages · at 14:32 · 3 read")
		mentions = mentions || strings.Contains(l, "(1 of 1) · mentions of me · at 14:32 · 3 read")
	}
	if !mine || !mentions {
		t.Errorf("chatProgressNamesTheDayTimeAndCounts: %v", lines)
	}

	page = &ts.FakePage{Signed: true, Answer: func(string, map[string]any) any { return `{"error":"HTTP 400"}` }}
	if _, _, err := collect(page, "2026-03-03", "2026-03-03", func(string) {}); err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("reportsAFailedSearchRequest: %v", err)
	}
	page = &ts.FakePage{}
	if _, _, err := collect(page, "2026-03-03", "2026-03-03", func(string) {}); !IsNotSignedIn(err) || len(page.Opened) != 0 {
		t.Errorf("chatNeedsSignIn: %v", err)
	}
	page = &ts.FakePage{Signed: true, Answer: func(_ string, args map[string]any) any {
		if args["startMs"] != nil {
			return `{"messages":[]}`
		}
		return "not json"
	}}
	if _, _, err := collect(page, "2026-03-03", "2026-03-03", func(string) {}); err == nil || !strings.Contains(err.Error(), "page changed") {
		t.Errorf("reportsAChangedPage: %v", err)
	}
}

func TestHRmasterFlow(t *testing.T) {
	page := &ts.FakePage{Answer: func(_ string, args map[string]any) any {
		return fmt.Sprintf(`{"html":"<p>%v-%v</p>"}`, args["year"], args["month"])
	}}
	pages, err := CollectHRmaster(ctx, page, "https://acme.hrmaster.hu", []Month{{2026, 10}, {2026, 11}}, func(string) {})
	if err != nil || !reflect.DeepEqual(page.Opened, []string{"https://acme.hrmaster.hu/Datacenter/#/employeeWorkingTime/own"}) ||
		pages[0].HTML != "<p>2026-10</p>" || pages[1].HTML != "<p>2026-11</p>" {
		t.Errorf("hrmasterReadsEachMonthWithItsArguments: %v %+v", err, pages)
	}
	page.Answer = func(string, map[string]any) any { return `{"error":"no-panel"}` }
	if _, err := CollectHRmaster(ctx, page, "https://acme.hrmaster.hu", []Month{{2026, 10}}, func(string) {}); !IsNotSignedIn(err) {
		t.Errorf("no-panel: %v", err)
	}
	page.Answer = func(string, map[string]any) any {
		return `{"error":"no-calendar","url":"https://acme.hrmaster.hu/Datacenter/#/home","label":"","panel":false}`
	}
	if _, err := CollectHRmaster(ctx, page, "https://acme.hrmaster.hu", []Month{{2026, 10}}, func(string) {}); err == nil ||
		IsNotSignedIn(err) || !strings.Contains(err.Error(), "/Datacenter/#/home") {
		t.Errorf("signed in without the calendar is not a sign-out: %v", err)
	}
	page.Answer = func(string, map[string]any) any { return `{"error":"unreachable"}` }
	if _, err := CollectHRmaster(ctx, page, "https://acme.hrmaster.hu", []Month{{2026, 10}}, func(string) {}); err == nil || err.Error() != "HRmaster: could not reach 2026-10." {
		t.Errorf("unreachable: %v", err)
	}
}

func calendarZip(t *testing.T, files map[string]string) string {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, text := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(text))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestGoogleCalendarExport(t *testing.T) {
	z := calendarZip(t, map[string]string{"me@x.hu.ics": "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", "team@group.calendar.google.com.ics": "x", "notes.txt": "ignored"})
	page := &ts.FakePage{Redirect: map[string]string{"https://calendar.google.com/calendar/": "https://calendar.google.com/calendar/u/1/r"}}
	page.Answer = func(string, map[string]any) any { return `{"zip":"` + z + `"}` }
	files, err := CollectGoogleCalendar(ctx, page, "")
	if err != nil || len(files) != 2 || page.Runs[0]["account"] != "1" {
		t.Fatalf("%v %v %v", err, files, page.Runs)
	}
	if id, text, all := PickCalendar(files, "", "ME@x.hu"); id != "me@x.hu" || !strings.HasPrefix(text, "BEGIN:VCALENDAR") || len(all) != 2 {
		t.Errorf("pick: %q %v", id, all)
	}
	if id, _, _ := PickCalendar(files, "other@x.hu"); id != "me@x.hu" {
		t.Errorf("the account's own calendar is the fallback: %q", id)
	}
	two := map[string]string{"me@x.hu": "", "you@x.hu": "", "hu.hungarian#holiday@group.v.calendar.google.com": ""}
	if id, _, _ := PickCalendar(two, "other@x.hu"); id != "" {
		t.Errorf("guessed between two people's calendars: %q", id)
	}

	signedOut := &ts.FakePage{Redirect: map[string]string{"https://calendar.google.com/calendar/": "https://accounts.google.com/v3/signin"}}
	if _, err := CollectGoogleCalendar(ctx, signedOut, ""); !IsNotSignedIn(err) {
		t.Errorf("signed out: %v", err)
	}
	failing := &ts.FakePage{Answer: func(string, map[string]any) any { return `{"error":"HTTP 403"}` }}
	failing.Redirect = map[string]string{"https://calendar.google.com/calendar/": "https://calendar.google.com/calendar/r"}
	if _, err := CollectGoogleCalendar(ctx, failing, ""); err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("failure: %v", err)
	}
}

func TestGoogleURLPicksTheAccount(t *testing.T) {
	if got := GoogleURL("https://chat.google.com/", ""); got != "https://chat.google.com/" {
		t.Fatal(got)
	}
	if got := GoogleURL("https://chat.google.com/", "me@x.hu"); got != "https://chat.google.com/?authuser=me%40x.hu" {
		t.Fatal(got)
	}
}
