package core

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Days computes day keys ("2006-01-02") and calendar math in one time zone. Weeks start on Monday (ISO).
// The app passes the system zone; tests pin Europe/Budapest.
type Days struct{ Loc *time.Location }

const dayLayout = "2006-01-02"

func (d Days) Key(t time.Time) string { return t.In(d.Loc).Format(dayLayout) }

// Parse returns the start of the day, or false when k isn't a day key.
func (d Days) Parse(k string) (time.Time, bool) {
	t, err := time.ParseInLocation(dayLayout, k, d.Loc)
	return t, err == nil
}

// Date is Parse, falling back to the zero time for a bad key (callers always pass valid keys).
func (d Days) Date(k string) time.Time {
	t, _ := d.Parse(k)
	return t
}

func (d Days) StartOfDay(t time.Time) time.Time {
	y, m, dd := t.In(d.Loc).Date()
	return time.Date(y, m, dd, 0, 0, 0, 0, d.Loc)
}

func (d Days) AddDays(t time.Time, n int) time.Time {
	y, m, dd := t.In(d.Loc).Date()
	return time.Date(y, m, dd+n, 0, 0, 0, 0, d.Loc)
}

func (d Days) DayAfter(t time.Time) time.Time { return d.AddDays(t, 1) }

// Shift moves a day key by n days.
func (d Days) Shift(day string, n int) string { return d.Key(d.AddDays(d.Date(day), n)) }

// Bounds is [start of day, start of next day).
func (d Days) Bounds(day string) (time.Time, time.Time) {
	s := d.Date(day)
	return s, d.DayAfter(s)
}

// Range lists the day keys from..to inclusive, in either order.
func (d Days) Range(from, to string) []string {
	if from > to {
		from, to = to, from
	}
	var out []string
	for k := from; k <= to; k = d.Shift(k, 1) {
		out = append(out, k)
	}
	return out
}

func (d Days) At(day string, hour, minute int) (time.Time, bool) {
	s, ok := d.Parse(day)
	if !ok || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, false
	}
	return time.Date(s.Year(), s.Month(), s.Day(), hour, minute, 0, 0, d.Loc), true
}

// AtHM parses "HH:MM" on a day.
func (d Days) AtHM(day, hm string) (time.Time, bool) {
	var p []int
	for _, s := range strings.Split(hm, ":") {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			p = append(p, n)
		}
	}
	if len(p) < 2 {
		return time.Time{}, false
	}
	return d.At(day, p[0], p[1])
}

func (d Days) IsWeekend(day string) bool {
	wd := d.Date(day).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

func (d Days) HM(t time.Time) string { return t.In(d.Loc).Format("15:04") }

// Title is "Tue, 3 Nov 2026".
func (d Days) Title(day string) string { return d.Date(day).Format("Mon, 2 Jan 2006") }

// Month returns the first and last day key of a "2006-01" month.
func (d Days) Month(month string) (string, string, bool) {
	t, err := time.ParseInLocation("2006-01", month, d.Loc)
	if err != nil {
		return "", "", false
	}
	return d.Key(t), d.Key(t.AddDate(0, 1, -1)), true
}

// Interval is [Start, End).
type Interval struct{ Start, End time.Time }

func (i Interval) Duration() time.Duration {
	if i.End.Before(i.Start) {
		return 0
	}
	return i.End.Sub(i.Start)
}

func Normalize(list []Interval) []Interval {
	var in []Interval
	for _, i := range list {
		if i.End.After(i.Start) {
			in = append(in, i)
		}
	}
	sort.SliceStable(in, func(a, b int) bool { return in[a].Start.Before(in[b].Start) })
	var out []Interval
	for _, i := range in {
		if n := len(out); n > 0 && !i.Start.After(out[n-1].End) {
			if i.End.After(out[n-1].End) {
				out[n-1].End = i.End
			}
		} else {
			out = append(out, i)
		}
	}
	return out
}

func Subtract(base, cuts []Interval) []Interval {
	result := Normalize(base)
	for _, c := range Normalize(cuts) {
		var next []Interval
		for _, b := range result {
			if !c.End.After(b.Start) || !c.Start.Before(b.End) {
				next = append(next, b)
				continue
			}
			if c.Start.After(b.Start) {
				next = append(next, Interval{b.Start, c.Start})
			}
			if c.End.Before(b.End) {
				next = append(next, Interval{c.End, b.End})
			}
		}
		result = next
	}
	return result
}

func Total(list []Interval) time.Duration {
	var t time.Duration
	for _, i := range Normalize(list) {
		t += i.Duration()
	}
	return t
}

func Overlap(list []Interval, w Interval) time.Duration {
	var t time.Duration
	for _, i := range Normalize(list) {
		s, e := maxTime(i.Start, w.Start), minTime(i.End, w.End)
		if e.After(s) {
			t += e.Sub(s)
		}
	}
	return t
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// FormatDuration: 5400 → "1h 30m", 1800 → "30m", 3600 → "1h".
func FormatDuration(seconds int) string {
	minutes := int(math.Round(float64(seconds) / 60))
	h, m := minutes/60, minutes%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

var unitRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([hm])`)

// ParseDuration accepts "1h 30m", "1.5h", "90m", "1:30" or a bare number of minutes ("90"); "," works as the decimal point.
func ParseDuration(text string) (int, bool) {
	t := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(text)), ",", ".")
	if t == "" {
		return 0, false
	}
	if strings.Contains(t, ":") {
		p := strings.Split(t, ":")
		if len(p) != 2 {
			return 0, false
		}
		h, err1 := strconv.Atoi(strings.TrimSpace(p[0]))
		m, err2 := strconv.Atoi(strings.TrimSpace(p[1]))
		if err1 != nil || err2 != nil {
			return 0, false
		}
		return (h*60 + m) * 60, true
	}
	if minutes, err := strconv.ParseFloat(t, 64); err == nil {
		return int(math.Round(minutes * 60)), true
	}
	matches := unitRe.FindAllStringSubmatch(t, -1)
	if len(matches) == 0 {
		return 0, false
	}
	total := 0.0
	for _, m := range matches {
		v, _ := strconv.ParseFloat(m[1], 64)
		if m[2] == "h" {
			total += v * 3600
		} else {
			total += v * 60
		}
	}
	return int(math.Round(total)), true
}
