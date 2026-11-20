package core_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

func TestCommitEstimator(t *testing.T) {
	s := def() // min 10, max 240, gap ≤ 180 minutes
	commit := func(subject string, files, insertions int) core.StoredCommit {
		return ts.Commit("2026-03-03", "10:00", ts.Subject(subject), ts.Size(files, insertions))
	}
	if got := core.ComplexityMinutes(commit("Add feature", 0, 0)); got != 10 {
		t.Errorf("emptyCommitCostsTheBaseTen: %v", got)
	}
	// 10 lines in 1 file: 10 + 12·log2(2) + 4 = 26 minutes before keyword weighting.
	for subject, want := range map[string]float64{
		"Add feature": 26, "Fix typo": 13, "Refactor billing": 33.8, "Fix bug in invoices": 29.9, "Add tests": 28.6,
	} {
		if got := core.ComplexityMinutes(commit(subject, 1, 10)); math.Abs(got-want) > 0.001 {
			t.Errorf("keywordsWeightTheSize %q: %v", subject, got)
		}
	}
	if got := core.EstimateSeconds(s, commit("Add feature", 1, 10), nil); got != 25*60 {
		t.Errorf("withoutGapUsesComplexityRoundedToFiveMinutes: %d", got)
	}
	if got := core.EstimateSeconds(s, commit("Add feature", 1, 10), p(3600.0)); got != 45*60 {
		t.Errorf("gapIsBlendedWithComplexity: %d", got)
	}
	for _, gap := range []float64{0, -60, 181 * 60} {
		if got := core.EstimateSeconds(s, commit("Add feature", 1, 10), &gap); got != 25*60 {
			t.Errorf("zeroOrTooLongGapsAreIgnored %v: %d", gap, got)
		}
	}
	if core.EstimateSeconds(s, commit("wip", 0, 0), nil) != 10*60 ||
		core.EstimateSeconds(s, commit("Add feature", 50, 100_000), nil) != 240*60 {
		t.Error("clampsToMinimumAndMaximum")
	}
}

func TestJiraKey(t *testing.T) {
	none := map[string]bool{}
	find := func(text string, known map[string]bool) string {
		k, _ := core.FindKey(text, known)
		return k
	}
	if find("Fix ABC-12 login, see ABC-13", none) != "ABC-12" {
		t.Error("findsFirstKey")
	}
	text := "UTF-8 handling, FOO-1 and PROJ-2"
	if find(text, map[string]bool{"PROJ": true}) != "PROJ-2" || find(text, none) != "FOO-1" {
		t.Error("knownPrefixesWin")
	}
	for _, text := range []string{"UTF-8 and ISO-8601", "xABC-12", "ABC-12a", "abc-12", "A-1", "no keys here"} {
		if k, ok := core.FindKey(text, none); ok {
			t.Errorf("ignoresNonKeys %q: %s", text, k)
		}
	}
	if find("ISO-5 rollout", map[string]bool{"ISO": true}) != "ISO-5" {
		t.Error("deniedPrefixIsStillUsedWhenKnown")
	}
	if str(core.NormalizeKeyString("  proj-1 \n")) != "PROJ-1" || core.NormalizeKeyString("   ") != nil || core.NormalizeKey(nil) != nil {
		t.Error("normalize")
	}
	if core.KeyPrefix("PROJ-12") != "PROJ" {
		t.Error("prefix")
	}
	for in, want := range map[string]string{
		"ABC-12: Fix login": "Fix login", "[ABC-12] Fix login": "Fix login", "ABC-12 - Fix login": "Fix login",
		"ABC-12 – Fix login": "Fix login", "Fix ABC-12 login": "Fix ABC-12 login",
	} {
		if got := core.StripLeadingKey(in); got != want {
			t.Errorf("stripLeadingKey %q: %q", in, got)
		}
	}
}

func TestDays(t *testing.T) {
	d := ts.Days
	instant, _ := time.Parse(time.RFC3339, "2026-03-02T23:30:00Z")
	if d.Key(instant) != "2026-03-03" {
		t.Error("keysUseThePinnedTimeZone")
	}
	if _, ok := d.Parse("not a day"); ok || d.Key(ts.At("2026-03-03", "23:59")) != "2026-03-03" {
		t.Error("parseAndKeyRoundTrip")
	}
	if got := d.Range("2026-03-30", "2026-03-28"); len(got) != 3 || got[0] != "2026-03-28" || got[2] != "2026-03-30" {
		t.Errorf("daysAreInclusiveAndOrderInsensitive: %v", got)
	}
	s, e := d.Bounds("2026-03-29") // clocks go forward: a 23-hour day
	s2, e2 := d.Bounds("2026-03-30")
	if e.Sub(s) != 23*time.Hour || e2.Sub(s2) != 24*time.Hour {
		t.Error("boundsFollowDaylightSaving")
	}
	for in, want := range map[string]string{"9:05": "09:05", "17:30": "17:30", " 8 : 00 ": "08:00"} {
		if at, ok := d.AtHM("2026-03-03", in); !ok || d.HM(at) != want || d.Key(at) != "2026-03-03" {
			t.Errorf("atParsesHoursAndMinutes %q", in)
		}
	}
	for _, in := range []string{"25:00", "10:60", "abc", "10", ""} {
		if _, ok := d.AtHM("2026-03-03", in); ok {
			t.Errorf("atRejectsInvalidTimes %q", in)
		}
	}
	if !d.IsWeekend("2026-03-07") || !d.IsWeekend("2026-03-08") || d.IsWeekend("2026-03-06") {
		t.Error("weekends")
	}
}

func TestIntervals(t *testing.T) {
	day := "2026-03-03"
	iv := func(a, b string) core.Interval { return ts.Interval(day, a, b) }
	got := core.Normalize([]core.Interval{iv("11:00", "12:00"), iv("09:00", "10:00"), iv("09:30", "11:00"), iv("13:00", "13:00")})
	if len(got) != 1 || got[0] != iv("09:00", "12:00") {
		t.Errorf("normalizeMergesOverlappingAndTouchingAndDropsEmpty: %v", got)
	}
	got = core.Subtract([]core.Interval{iv("09:00", "17:00")}, []core.Interval{iv("12:00", "12:30")})
	if len(got) != 2 || got[0] != iv("09:00", "12:00") || got[1] != iv("12:30", "17:00") {
		t.Errorf("subtractSplitsAroundCuts: %v", got)
	}
	base := []core.Interval{iv("09:00", "10:00")}
	if len(core.Subtract(base, []core.Interval{iv("08:00", "11:00")})) != 0 {
		t.Error("covering cut")
	}
	if got := core.Subtract(base, []core.Interval{iv("10:00", "11:00")}); len(got) != 1 || got[0] != base[0] {
		t.Error("disjoint cut")
	}
	if core.Total([]core.Interval{iv("09:00", "10:00"), iv("09:30", "10:30")}) != 5400*time.Second {
		t.Error("totalCountsOverlapOnce")
	}
	if core.Overlap([]core.Interval{iv("09:00", "12:00"), iv("13:00", "17:00")}, iv("11:00", "14:00")) != 7200*time.Second {
		t.Error("overlapWithWindow")
	}
	if iv("10:00", "09:00").Duration() != 0 {
		t.Error("negativeIntervalHasZeroDuration")
	}
}

func TestDurationFormat(t *testing.T) {
	for s, want := range map[int]string{0: "0m", 89: "1m", 1800: "30m", 3600: "1h", 5400: "1h 30m", 8 * 3600: "8h"} {
		if got := core.FormatDuration(s); got != want {
			t.Errorf("formats %d: %s", s, got)
		}
	}
	for _, in := range []string{"1h 30m", "1.5h", "1,5h", "90m", "1:30", "90", " 1H 30M "} {
		if got, ok := core.ParseDuration(in); !ok || got != 5400 {
			t.Errorf("parsesNinetyMinutes %q: %d", in, got)
		}
	}
	for _, in := range []string{"", "   ", "abc", "1:xx", "1:2:3"} {
		if _, ok := core.ParseDuration(in); ok {
			t.Errorf("rejectsGarbage %q", in)
		}
	}
}

// RELEASE_REVIEW C7: an edit of the copy must not reach the settings it came from before they are saved.
func TestSettingsCloneIsDeep(t *testing.T) {
	s := core.DefaultSettings()
	s.Periods = []core.ProjectPeriod{{ID: "p", From: "2026-10-01", To: "2026-10-31"}}
	s.JiraAccounts = []core.JiraAccount{{ID: "j", ProjectKeys: []string{"PROJ"}}}
	s.DefaultJiraAccountID = core.Ptr("j")
	c := s.Clone()
	c.Periods[0].To = "2026-10-02"
	c.JiraAccounts[0].ProjectKeys[0] = "WEB"
	*c.DefaultJiraAccountID = "x"
	c.Profiles = append(c.Profiles, core.Profile{ID: "new"})
	if s.Periods[0].To != "2026-10-31" || s.JiraAccounts[0].ProjectKeys[0] != "PROJ" || *s.DefaultJiraAccountID != "j" || len(s.Profiles) != 0 {
		t.Errorf("the original changed: %+v", s)
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := core.Settings{Language: "de", Appearance: "neon", AccentColor: "red", Skin: "mac", Transparency: "half", TextScale: 3}
	s.Normalize()
	if s.Language != "system" || s.Appearance != "system" || s.AccentColor != "red" || s.Skin != "mac" || s.Transparency != "system" ||
		s.TextScale != 0 || s.GoogleCalendarID != "primary" {
		t.Errorf("%+v", s)
	}
}

// Every message the backend can send is translated in both UI languages, with the same placeholders (B7).
func TestMessagesAreTranslated(t *testing.T) {
	placeholders := regexp.MustCompile(`\{\w+\}`)
	set := func(s string) string {
		p := placeholders.FindAllString(s, -1)
		sort.Strings(p)
		return strings.Join(p, ",")
	}
	for _, lang := range []string{"en", "hu"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "app", "i18n", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var dict map[string]any
		if err := json.Unmarshal(data, &dict); err != nil {
			t.Fatal(err)
		}
		for _, code := range core.Codes() {
			v, ok := dict["msg."+code].(string)
			if !ok {
				t.Errorf("%s.json: msg.%s is missing", lang, code)
				continue
			}
			if set(v) != set(core.English[code]) {
				t.Errorf("%s.json: msg.%s has %s, Go has %s", lang, code, set(v), set(core.English[code]))
			}
		}
	}
	if m := core.Say("push.done", "n", 3); m.Text != "Sent 3 worklogs to Tempo." || m.Params["n"] != 3 {
		t.Errorf("%+v", m)
	}
	if m := core.Say("collect.nothing", "from", "a", "to", "b").WithSkipped([]string{"x", "y"}); m.Skipped != "x; y" ||
		!strings.HasSuffix(m.Text, " Skipped: x; y") {
		t.Errorf("%+v", m)
	}
}
