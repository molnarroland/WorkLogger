package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// calendarScript downloads Google Calendar's own export (Settings → Import & export → Export: every calendar as
// an .ics file in a zip) with the page's sign-in, and returns it base64-encoded. `account` is the /u/N index.
const calendarScript = `
const r = await fetch('/calendar/u/' + account + '/exporticalzip', { credentials: 'include' });
if (!r.ok) return JSON.stringify({ error: 'HTTP ' + r.status });
if (!/zip|octet-stream/.test(r.headers.get('content-type') || '')) return JSON.stringify({ error: 'not-zip' });
const bytes = new Uint8Array(await r.arrayBuffer());
let s = '';
for (let i = 0; i < bytes.length; i += 0x8000) s += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
return JSON.stringify({ zip: btoa(s) });
`

// CollectGoogleCalendar reads every calendar of the Google account signed in to the page (the Google Chat
// window shares the sign-in) as iCalendar text, keyed by calendar id ("me@x.hu", "…@group.calendar.google.com").
// No API key needed: it is the export the Calendar web app offers.
// authuser picks the Google account (GoogleURL).
func CollectGoogleCalendar(ctx context.Context, page core.PageDriver, authuser string) (map[string]string, error) {
	if err := page.Open(ctx, GoogleURL("https://calendar.google.com/calendar/", authuser)); err != nil {
		return nil, err
	}
	if err := page.WaitUntilLoaded(ctx); err != nil {
		return nil, err
	}
	cur := page.CurrentURL(ctx)
	u, _ := url.Parse(cur)
	if u == nil || u.Hostname() != "calendar.google.com" {
		return nil, NotSignedInError{"Google"}
	}
	account := "0"
	if m := accountRe.FindStringSubmatch(u.Path); m != nil {
		account = m[1]
	}
	raw, err := page.Run(ctx, calendarScript, map[string]any{"account": account}, nil)
	if err != nil {
		return nil, err
	}
	var r struct{ Zip, Error string }
	s, ok := raw.(string)
	if !ok || json.Unmarshal([]byte(s), &r) != nil {
		return nil, core.Err("calendar.exportUnreadable")
	}
	if r.Error != "" {
		return nil, core.Err("calendar.exportFailed", "reason", r.Error)
	}
	data, err := base64.StdEncoding.DecodeString(r.Zip)
	if err != nil {
		return nil, err
	}
	return UnzipCalendars(data)
}

// UnzipCalendars reads the export zip: one "<calendar id>.ics" per calendar.
func UnzipCalendars(data []byte) (map[string]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, core.Err("calendar.notZip")
	}
	out := map[string]string{}
	for _, f := range zr.File {
		name := path.Base(f.Name)
		if !strings.HasSuffix(strings.ToLower(name), ".ics") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		text, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		out[strings.TrimSuffix(name, path.Ext(name))] = string(text)
	}
	return out, nil
}

// PickCalendar finds the first of ids among the exported calendars (case-insensitive; Google names the primary
// calendar after the account's email), else the only calendar named after a person. It also lists what is there,
// for the error message.
func PickCalendar(files map[string]string, ids ...string) (id, text string, available []string) {
	for name := range files {
		available = append(available, name)
	}
	sort.Strings(available)
	for _, want := range ids {
		for _, name := range available {
			if want != "" && strings.EqualFold(name, want) {
				return name, files[name], available
			}
		}
	}
	// None named: the account's own calendar is the one named after a person's address (shared calendars end in
	// calendar.google.com, holidays and birthdays have a '#').
	var own []string
	for _, name := range available {
		if strings.Contains(name, "@") && !strings.Contains(name, "#") && !strings.HasSuffix(strings.ToLower(name), "calendar.google.com") {
			own = append(own, name)
		}
	}
	if len(own) == 1 {
		return own[0], files[own[0]], available
	}
	return "", "", available
}
