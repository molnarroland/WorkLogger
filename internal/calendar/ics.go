package calendar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// FetchICS reads a calendar feed (Outlook's "publish calendar" link, a Google secret address, …) and returns its
// timed meetings in [from, to). The feed URL is the calendar id.
func FetchICS(ctx context.Context, h core.HTTPDoer, feedURL string, from, to time.Time, loc *time.Location, me []string) ([]core.Meeting, error) {
	u := strings.TrimSpace(feedURL)
	if strings.HasPrefix(u, "webcal://") {
		u = "https://" + strings.TrimPrefix(u, "webcal://")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("calendar feed %q: %w", feedURL, err)
	}
	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("calendar feed answered with HTTP %d", resp.StatusCode)
	}
	return ParseICS(string(data), feedURL, from, to, loc, me), nil
}

type icsProp struct {
	params map[string]string
	value  string
}

type icsEvent map[string][]icsProp

func (e icsEvent) get(name string) (icsProp, bool) {
	if p := e[name]; len(p) > 0 {
		return p[0], true
	}
	return icsProp{}, false
}

func (e icsEvent) text(name string) string {
	p, _ := e.get(name)
	r := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return r.Replace(p.value)
}

// ParseICS returns the timed, not cancelled events of an iCalendar text that fall in [from, to), recurring ones
// expanded. Like the Google API reader it skips all-day events, free (transparent) blocks such as working
// location, and events one of the `me` addresses declined.
func ParseICS(text, calendarID string, from, to time.Time, loc *time.Location, me []string) []core.Meeting {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\n ", ""), "\n\t", "") // unfold
	var events []icsEvent
	var cur icsEvent
	for _, line := range strings.Split(text, "\n") {
		switch {
		case line == "BEGIN:VEVENT":
			cur = icsEvent{}
		case line == "END:VEVENT":
			if cur != nil {
				events = append(events, cur)
			}
			cur = nil
		case cur != nil:
			colon := strings.IndexByte(line, ':')
			if colon < 0 {
				continue
			}
			parts := strings.Split(line[:colon], ";")
			p := icsProp{params: map[string]string{}, value: line[colon+1:]}
			for _, kv := range parts[1:] {
				if k, v, ok := strings.Cut(kv, "="); ok {
					p.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
				}
			}
			name := strings.ToUpper(parts[0])
			cur[name] = append(cur[name], p)
		}
	}

	// Overrides of single occurrences (RECURRENCE-ID) replace the generated ones.
	overridden := map[string]bool{}
	for _, e := range events {
		if rid, ok := e.get("RECURRENCE-ID"); ok {
			if t, ok := icsTime(rid, loc); ok {
				overridden[e.text("UID")+"@"+strconv.FormatInt(t.Unix(), 10)] = true
			}
		}
	}

	var out []core.Meeting
	for _, e := range events {
		if strings.EqualFold(e.text("STATUS"), "CANCELLED") || strings.EqualFold(e.text("TRANSP"), "TRANSPARENT") ||
			declined(e, me) {
			continue
		}
		sp, ok := e.get("DTSTART")
		if !ok || sp.params["VALUE"] == "DATE" || len(sp.value) == 8 {
			continue
		}
		start, ok := icsTime(sp, loc)
		if !ok {
			continue
		}
		var length time.Duration
		if ep, ok := e.get("DTEND"); ok {
			if end, ok := icsTime(ep, loc); ok {
				length = end.Sub(start)
			}
		} else if dp, ok := e.get("DURATION"); ok {
			length = icsDuration(dp.value)
		}
		if length <= 0 {
			continue
		}
		uid := e.text("UID")
		m := core.Meeting{CalendarID: calendarID, Title: e.text("SUMMARY"), Details: e.text("DESCRIPTION")}
		if m.Title == "" {
			m.Title = "(no title)"
		}
		_, recurring := e.get("RRULE")
		_, isOverride := e.get("RECURRENCE-ID")
		starts := []time.Time{start}
		if recurring && !isOverride {
			rr, _ := e.get("RRULE")
			starts = expand(start, rr.value, exdates(e, loc), to)
		}
		for _, s := range starts {
			id := uid + "@" + strconv.FormatInt(s.Unix(), 10)
			if !recurring && !isOverride {
				id = uid
			} else if recurring && !isOverride && overridden[id] {
				continue
			}
			if s.Before(from) || !s.Before(to) {
				continue
			}
			m.ID, m.Start, m.End = id, s, s.Add(length)
			out = append(out, m)
		}
	}
	return out
}

func icsTime(p icsProp, fallback *time.Location) (time.Time, bool) {
	v := p.value
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse("20060102T150405Z", v)
		return t, err == nil
	}
	loc := fallback
	if tz := p.params["TZID"]; tz != "" {
		// Known limit: IANA names only; Outlook's Windows zone names ("Central Europe Standard Time") fall back to the
		// local zone. Map them (CLDR windowsZones) if feeds from other zones show up.
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	t, err := time.ParseInLocation("20060102T150405", v, loc)
	return t, err == nil
}

func exdates(e icsEvent, loc *time.Location) map[int64]bool {
	out := map[int64]bool{}
	for _, p := range e["EXDATE"] {
		for _, v := range strings.Split(p.value, ",") {
			if t, ok := icsTime(icsProp{params: p.params, value: v}, loc); ok {
				out[t.Unix()] = true
			}
		}
	}
	return out
}

// icsDuration reads "PT1H30M", "P1D", "PT45M".
func icsDuration(v string) time.Duration {
	var d time.Duration
	num := ""
	for _, r := range strings.TrimPrefix(strings.TrimPrefix(v, "+"), "P") {
		switch {
		case r >= '0' && r <= '9':
			num += string(r)
		case r == 'T':
		default:
			n, _ := strconv.Atoi(num)
			num = ""
			switch r {
			case 'W':
				d += time.Duration(n) * 7 * 24 * time.Hour
			case 'D':
				d += time.Duration(n) * 24 * time.Hour
			case 'H':
				d += time.Duration(n) * time.Hour
			case 'M':
				d += time.Duration(n) * time.Minute
			case 'S':
				d += time.Duration(n) * time.Second
			}
		}
	}
	return d
}

var weekdays = map[string]time.Weekday{
	"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday,
	"TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday,
}

// expand lists the starts of a recurring event up to `until`.
// Known limit: FREQ=DAILY/WEEKLY (BYDAY)/MONTHLY/YEARLY with INTERVAL, COUNT, UNTIL and EXDATE; BYSETPOS, BYMONTHDAY
// lists and ordinal BYDAY ("2TU") are ignored. Covers standups and weekly meetings; add rules when a feed needs them.
func expand(start time.Time, rule string, ex map[int64]bool, until time.Time) []time.Time {
	r := map[string]string{}
	for _, kv := range strings.Split(rule, ";") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			r[strings.ToUpper(k)] = v
		}
	}
	interval, _ := strconv.Atoi(r["INTERVAL"])
	interval = max(1, interval)
	count, _ := strconv.Atoi(r["COUNT"])
	if u := r["UNTIL"]; u != "" {
		var t time.Time
		ok := false
		if len(u) == 8 { // a date: the whole day counts
			if d, err := time.ParseInLocation("20060102", u, start.Location()); err == nil {
				t, ok = d.AddDate(0, 0, 1), true
			}
		} else if d, good := icsTime(icsProp{value: u}, start.Location()); good {
			t, ok = d.Add(time.Second), true
		}
		if ok && t.Before(until) {
			until = t
		}
	}
	var days []time.Weekday
	for _, d := range strings.Split(r["BYDAY"], ",") {
		if wd, ok := weekdays[strings.ToUpper(strings.TrimSpace(d))]; ok {
			days = append(days, wd)
		}
	}
	var out []time.Time
	n := 0
	add := func(t time.Time) bool { // false when done
		if !t.Before(until) || (count > 0 && n >= count) {
			return false
		}
		n++
		if !ex[t.Unix()] {
			out = append(out, t)
		}
		return true
	}
	y, mo, d := start.Date()
	h, mi, s := start.Clock()
	at := func(y int, mo time.Month, d int) time.Time { return time.Date(y, mo, d, h, mi, s, 0, start.Location()) }
	for i := 0; i < 5000; i++ {
		switch r["FREQ"] {
		case "DAILY":
			if !add(at(y, mo, d+i*interval)) {
				return out
			}
		case "WEEKLY":
			if len(days) == 0 {
				if !add(at(y, mo, d+7*i*interval)) {
					return out
				}
				continue
			}
			// The week (Monday first) i·interval weeks after the start's week.
			offset := (int(start.Weekday()) + 6) % 7
			monday := at(y, mo, d-offset+7*i*interval)
			if !monday.Before(until) {
				return out
			}
			for k := 0; k < 7; k++ {
				t := at(monday.Year(), monday.Month(), monday.Day()+k)
				if t.Before(start) || !containsDay(days, t.Weekday()) {
					continue
				}
				if !add(t) {
					return out
				}
			}
		case "MONTHLY":
			t := at(y, mo+time.Month(i*interval), d)
			if t.Day() != d {
				continue // no such day in that month (the 31st)
			}
			if !add(t) {
				return out
			}
		case "YEARLY":
			if !add(at(y+i*interval, mo, d)) {
				return out
			}
		default:
			return []time.Time{start}
		}
	}
	return out
}

func containsDay(days []time.Weekday, d time.Weekday) bool {
	for _, x := range days {
		if x == d {
			return true
		}
	}
	return false
}

// declined: one of my addresses is an attendee with PARTSTAT=DECLINED.
func declined(e icsEvent, me []string) bool {
	for _, a := range e["ATTENDEE"] {
		addr := strings.TrimPrefix(strings.ToLower(a.value), "mailto:")
		for _, m := range me {
			if m != "" && addr == strings.ToLower(m) && strings.EqualFold(a.params["PARTSTAT"], "DECLINED") {
				return true
			}
		}
	}
	return false
}
