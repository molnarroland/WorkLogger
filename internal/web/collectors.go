// Package web reads HRmaster and Google Chat through a signed-in browser page. The collectors
// only see core.PageDriver; platform.Chrome is the real one.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// NotSignedInError means the site needs a sign-in in its browser window first.
type NotSignedInError struct{ Site string }

func (e NotSignedInError) Error() string { return "Sign in to " + e.Site + " in its window first." }

func IsNotSignedIn(err error) bool {
	var e NotSignedInError
	return errors.As(err, &e)
}

// ── HRmaster ────────────────────────────────────────────────────────────────

type Month struct{ Year, Month int }

// NormalizeHRmaster: "acme.hrmaster.hu", "https://acme.hrmaster.hu/Datacenter/…" → https://acme.hrmaster.hu
func NormalizeHRmaster(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return "", false
	}
	if !strings.Contains(t, "://") {
		t = "https://" + t
	}
	u, err := url.Parse(t)
	if err != nil || !strings.Contains(u.Hostname(), ".") {
		return "", false
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + u.Hostname(), true
}

// WorkingTimeURL is the work-time page of a company's HRmaster.
func WorkingTimeURL(base string) string {
	return strings.TrimRight(base, "/") + "/Datacenter/#/employeeWorkingTime/own"
}

func Months(days core.Days, fromDay, toDay string) []Month {
	f, t := days.Date(fromDay), days.Date(toDay)
	var out []Month
	for d := time.Date(f.Year(), f.Month(), 1, 0, 0, 0, 0, days.Loc); !d.After(t); d = d.AddDate(0, 1, 0) {
		out = append(out, Month{d.Year(), int(d.Month())})
	}
	return out
}

var monthNames = []string{
	"január", "február", "március", "április", "május", "június", "július", "augusztus",
	"szeptember", "október", "november", "december",
}

func hrMessage(code, label string, m Month) string {
	switch code {
	case "no-arrows":
		return fmt.Sprintf("HRmaster: the month arrows next to “%s” were not found.", label)
	case "stuck":
		return fmt.Sprintf("HRmaster: the month arrow next to “%s” did not change the month.", label)
	case "unreachable":
		return fmt.Sprintf("HRmaster: could not reach %d-%d.", m.Year, m.Month)
	case "no-calendar":
		return "HRmaster: signed in, but the work-time calendar was not recognised"
	}
	return "HRmaster: " + code
}

type MonthPage struct {
	Month Month
	HTML  string
}

// CollectHRmaster opens the work-time page, steps the month selector to each month and returns the calendar
// panel's HTML per month, for core.HRmasterParser.
func CollectHRmaster(ctx context.Context, page core.PageDriver, base string, months []Month, progress func(string)) ([]MonthPage, error) {
	if err := page.Open(ctx, WorkingTimeURL(base)); err != nil {
		return nil, err
	}
	if err := page.WaitUntilLoaded(ctx); err != nil {
		return nil, err
	}
	var out []MonthPage
	for i, m := range months {
		progress(fmt.Sprintf("HRmaster: reading %d. %s (%d of %d)…", m.Year, monthNames[m.Month-1], i+1, len(months)))
		raw, err := page.Run(ctx, hrmasterScript, map[string]any{"year": m.Year, "month": m.Month}, nil)
		if err != nil {
			return nil, err
		}
		var r struct {
			HTML, Label, Error, URL string
			Panel                   bool
		}
		s, ok := raw.(string)
		if !ok || json.Unmarshal([]byte(s), &r) != nil {
			return nil, core.Err("hrmaster.unreadable")
		}
		if r.Error == "no-panel" {
			return nil, NotSignedInError{"HRmaster"}
		}
		if r.Error == "no-calendar" {
			// What the page showed instead, for the activity console and the status line.
			return nil, fmt.Errorf("%s (page %s, calendar panel found: %v, month label: %q).",
				hrMessage(r.Error, r.Label, m), r.URL, r.Panel, r.Label)
		}
		if r.Error != "" {
			return nil, errors.New(hrMessage(r.Error, r.Label, m))
		}
		out = append(out, MonthPage{m, r.HTML})
	}
	return out, nil
}

// ── Google Chat ─────────────────────────────────────────────────────────────

// RawMessage is one message as the page script reports it.
type RawMessage struct {
	ID       string  `json:"id"`
	GID      *string `json:"gid"`
	Conv     *string `json:"conv"`
	TS       float64 `json:"ts"`
	OneOnOne bool    `json:"oneOnOne"`
	Sender   *string `json:"sender"`
	Me       bool    `json:"me"`
	Text     string  `json:"text"`
	Extra    *string `json:"extra"`
}

type search struct {
	title  string
	params func(email string) [][2]string
}

// The page searches still run (the extension's): my messages come from Chat's search request instead (chatRPC).
// Known limit: mentions still load and scroll a search page per day; move them to the request once its mention
// filter is known.
var searches = []search{
	{"mentions of me", func(string) [][2]string { return [][2]string{{"chasmention", "true"}} }},
}

var accountRe = regexp.MustCompile(`/u/(\d+)`)

// SearchURL is e.g. /u/0/app/search?q=&cfrom=me@x&cdatestart=2026-10-1&cdateend=2026-10-7&cdateoption=custom&cmembership=1
// for the days first..last (inclusive).
func SearchURL(account string, params [][2]string, first, last time.Time) string {
	urlDay := func(d time.Time) string { return fmt.Sprintf("%d-%d-%d", d.Year(), int(d.Month()), d.Day()) }
	all := append([][2]string{{"q", ""}}, params...)
	all = append(all, [2]string{"cdatestart", urlDay(first)}, [2]string{"cdateend", urlDay(last)},
		[2]string{"cdateoption", "custom"}, [2]string{"cmembership", "1"})
	var q []string
	for _, kv := range all {
		q = append(q, url.QueryEscape(kv[0])+"="+strings.ReplaceAll(url.QueryEscape(kv[1]), "%40", "@"))
	}
	return "https://chat.google.com/u/" + account + "/app/search?" + strings.Join(q, "&")
}

// ChatDayPause is the wait between two days, to go easy on Chat. A var so tests can skip it.
var ChatDayPause = 5 * time.Second

// GoogleURL is a Google app's address for one signed-in account: Google keeps several accounts signed in at
// once and picks one by ?authuser=<email> (redirecting to its /u/N/). Empty authuser: the default account.
func GoogleURL(base, authuser string) string {
	if authuser == "" {
		return base
	}
	return base + "?authuser=" + url.QueryEscape(authuser)
}

// CollectGoogleChat collects your Google Chat messages for fromDay..toDay in the signed-in page, a day at a time:
// my messages with Chat's search request, mentions with a page search. Each finished day goes to onDay (to be
// saved and shown) before the next one starts.
// authuser picks the Google account (GoogleURL); email is "me" in the messages.
func CollectGoogleChat(ctx context.Context, page core.PageDriver, days core.Days, email, authuser, fromDay, toDay string,
	progress func(string), onDay func(day string, e core.ChatExport) error,
) error {
	if !page.IsSignedIn(ctx) {
		return NotSignedInError{page.SiteName()}
	}
	if err := page.Open(ctx, GoogleURL("https://chat.google.com/", authuser)); err != nil {
		return err
	}
	if err := page.WaitUntilLoaded(ctx); err != nil {
		return err
	}
	account := "0"
	if m := accountRe.FindStringSubmatch(page.CurrentURL(ctx)); m != nil {
		account = m[1]
	}
	list := days.Range(fromDay, toDay)
	for i, day := range list {
		if i > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(ChatDayPause):
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		start, end := days.Bounds(day)
		startMs, endMs := float64(start.UnixMilli()), float64(end.UnixMilli())
		title := start.Format("Mon 2 Jan") + fmt.Sprintf(" (%d of %d)", i+1, len(list))
		found := map[string]RawMessage{}
		show := func(what string, scanned int, ts *float64) {
			at := ""
			if ts != nil {
				at = " · at " + days.HM(time.UnixMilli(int64(*ts)))
			}
			progress(fmt.Sprintf("Google Chat · %s · %s%s · %d read…", title, what, at, len(found)+scanned))
		}
		show("my messages", 0, nil)
		mine, err := chatRPC(ctx, page, account, start, end, func(n int, ts *float64) { show("my messages", n, ts) })
		if err != nil {
			return err
		}
		keep := func(msgs []RawMessage) {
			for _, m := range msgs {
				if m.GID != nil && m.TS >= startMs && m.TS < endMs {
					found[m.ID] = m
				}
			}
		}
		keep(mine)
		for _, s := range searches {
			show(s.title, 0, nil)
			if err := page.Open(ctx, SearchURL(account, s.params(email), start, start)); err != nil {
				return err
			}
			if err := page.WaitUntilLoaded(ctx); err != nil {
				return err
			}
			raw, err := page.Run(ctx, chatScript, nil, func(j string) {
				var p struct {
					N  int      `json:"n"`
					TS *float64 `json:"ts"`
				}
				if json.Unmarshal([]byte(j), &p) == nil {
					show(s.title, p.N, p.TS)
				}
			})
			if err != nil {
				return err
			}
			var msgs []RawMessage
			str, ok := raw.(string)
			if !ok || json.Unmarshal([]byte(str), &msgs) != nil {
				return core.Err("chat.pageChanged")
			}
			keep(msgs)
		}
		all := make([]RawMessage, 0, len(found))
		for _, m := range found {
			all = append(all, m)
		}
		if err := onDay(day, ChatExportOf(all, email)); err != nil {
			return err
		}
	}
	return nil
}

// chatRPC reads my messages in [start, end) with Chat's own search request (chatRPCScript).
func chatRPC(ctx context.Context, page core.PageDriver, account string, start, end time.Time, progress func(int, *float64)) ([]RawMessage, error) {
	raw, err := page.Run(ctx, chatRPCScript, map[string]any{"account": account, "startMs": start.UnixMilli(), "endMs": end.UnixMilli()},
		func(j string) {
			var p struct {
				N  int      `json:"n"`
				TS *float64 `json:"ts"`
			}
			if json.Unmarshal([]byte(j), &p) == nil {
				progress(p.N, p.TS)
			}
		})
	if err != nil {
		return nil, err
	}
	var r struct {
		Messages []RawMessage
		Error    string
	}
	if s, ok := raw.(string); !ok || json.Unmarshal([]byte(s), &r) != nil {
		return nil, core.Err("chat.searchUnreadable")
	}
	if r.Error != "" {
		return nil, core.Err("chat.searchFailed", "reason", r.Error)
	}
	return r.Messages, nil
}

// ChatExportOf groups messages into conversations, oldest first; my messages get my email as sender.
func ChatExportOf(messages []RawMessage, email string) core.ChatExport {
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].TS < messages[j].TS })
	var order []string
	groups := map[string][]RawMessage{}
	for _, m := range messages {
		if m.GID == nil {
			continue
		}
		if _, ok := groups[*m.GID]; !ok {
			order = append(order, *m.GID)
		}
		groups[*m.GID] = append(groups[*m.GID], m)
	}
	e := core.ChatExport{Source: "google-chat", Conversations: []core.ChatConversation{}}
	for _, gid := range order {
		list := groups[gid]
		c := core.ChatConversation{ID: gid, Type: strings.SplitN(gid, "/", 2)[0], Name: list[0].Conv}
		for _, m := range list {
			sender := m.Sender
			if m.Me {
				sender = &email
			}
			c.Messages = append(c.Messages, core.ChatMessage{
				Time: time.UnixMilli(int64(m.TS)), Sender: sender,
				Me: m.Me, Text: m.Text, Extra: m.Extra,
			})
		}
		e.Conversations = append(e.Conversations, c)
	}
	return e
}
