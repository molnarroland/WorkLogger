package tempo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Exportable is the entries Tempo can take: with time and an issue.
func Exportable(logs []core.Worklog) []core.Worklog {
	var out []core.Worklog
	for _, w := range logs {
		if w.Exportable() {
			out = append(out, w)
		}
	}
	return out
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `"`, `""`), "\n", " ") + `"`
}

// CSV in the format of Tempo's worklog importer ("Issue Key, Date Started, Time Spent, Comment").
func CSV(logs []core.Worklog) string {
	lines := []string{"Issue Key,Date Started,Time Spent,Comment"}
	for _, w := range Exportable(logs) {
		lines = append(lines, strings.Join([]string{
			*w.IssueKey, w.Start.UTC().Format("2006-01-02 15:04:05Z"),
			core.FormatDuration(w.Duration), quote(w.TempoText()),
		}, ","))
	}
	return strings.Join(lines, "\n") + "\n"
}

func Payloads(logs []core.Worklog, accountID string, days core.Days) []Payload {
	var author *string
	if accountID != "" {
		author = &accountID
	}
	out := []Payload{}
	for _, w := range Exportable(logs) {
		local := w.Start.In(days.Loc)
		out = append(out, Payload{
			IssueKey: *w.IssueKey, AuthorAccountID: author, StartDate: local.Format("2006-01-02"),
			StartTime: local.Format("15:04:05"), TimeSpentSeconds: w.Duration, Description: w.TempoText(),
		})
	}
	return out
}

// JSON is the payloads, pretty-printed with sorted keys.
func JSON(logs []core.Worklog, accountID string, days core.Days) ([]byte, error) {
	var generic []map[string]any // maps marshal with sorted keys
	data, _ := json.Marshal(Payloads(logs, accountID, days))
	_ = json.Unmarshal(data, &generic)
	return json.MarshalIndent(generic, "", "  ")
}

// Markdown is a checklist Claude (with Claude in Chrome) can follow to enter the worklogs in Tempo by hand.
func Markdown(logs []core.Worklog, from, to, jiraBaseURL string, days core.Days) string {
	ready := Exportable(logs)
	sort.SliceStable(ready, func(i, j int) bool { return ready[i].Start.Before(ready[j].Start) })
	byDay := map[string][]core.Worklog{}
	var keys []string
	for _, w := range ready {
		if _, ok := byDay[w.Day]; !ok {
			keys = append(keys, w.Day)
		}
		byDay[w.Day] = append(byDay[w.Day], w)
	}
	sort.Strings(keys)
	site := strings.Trim(jiraBaseURL, "/ ")
	myWork := "Jira → Apps → Tempo → My Work"
	if site != "" {
		myWork = site + "/plugins/servlet/ac/io.tempo.jira/tempo-app#!/my-work/week?type=TIME&date=" + from
	}
	var b strings.Builder
	fmt.Fprintf(&b, `# Tempo timesheet %s – %s

%d worklogs · %s in total · %d days

## Instructions for Claude

You are filling in my Tempo timesheet in Jira Cloud with Claude in Chrome. I am already signed in.

1. Open Tempo **My Work** in a new tab: %s
2. Go through the days below in order. Before logging a day, look at the worklogs already shown for it in Tempo.
   Skip an entry that already exists there (same issue, same duration), so nothing is logged twice.
3. For each remaining entry, click the day (or **Log Time**) and fill in the dialog:
   - **Issue**: type the issue key, then pick the matching issue from the list
   - **Date**: the day of the section
   - **Start time** and **Duration** exactly as in the table (duration format like `+"`1h 30m`"+`)
   - **Description**: the text from the table
   - Leave every other field (account, attributes, billable time) at its default, then click **Log**.
4. When a day is done, check that its total in Tempo matches the **Day total** below.
5. Never edit or delete worklogs that were already in Tempo, and do not submit the timesheet for approval.
6. If an issue can't be found, you lack permission, or anything looks unexpected, skip that entry and keep going.
   Do not guess a different issue.
7. At the end, reply with a table of every entry: logged, skipped (already there) or failed (with the reason).

## Overview

| Day | Day total | Entries |
| --- | --- | --- |
`, from, to, len(ready), core.FormatDuration(core.TotalDuration(ready)), len(keys), myWork)
	for _, d := range keys {
		fmt.Fprintf(&b, "| %s | %s | %d |\n", d, core.FormatDuration(core.TotalDuration(byDay[d])), len(byDay[d]))
	}
	for _, d := range keys {
		list := byDay[d]
		fmt.Fprintf(&b, "\n## %s — %s\n\nDay total: **%s**\n\n| # | Issue | Start | Duration | Description |\n| --- | --- | --- | --- | --- |\n",
			days.Title(d), d, core.FormatDuration(core.TotalDuration(list)))
		for i, w := range list {
			text := strings.ReplaceAll(strings.ReplaceAll(w.TempoText(), "|", `\|`), "\n", " ")
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n", i+1, *w.IssueKey, days.HM(w.Start), core.FormatDuration(w.Duration), text)
		}
	}
	return b.String()
}

// Line is one result of a connection check; OK nil means not checked (something before it failed).
type Line struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	OK     *bool  `json:"ok"`
}
