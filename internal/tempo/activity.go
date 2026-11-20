package tempo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Activity is one thing you did in Jira, as its activity stream shows it: created an issue, changed one of its
// fields (status, assignee, …) or commented.
type Activity struct {
	At       time.Time
	IssueKey string
	Summary  string
	Kind     string // "created", "change" or "comment"
	Field    string // a change's field, as Jira names it ("status", "assignee", …)
	From, To string // a change's old and new value, as Jira shows them
	Text     string // a comment's text
}

// Fields whose changes are bookkeeping, not something you did: the board order and the time tracking a worklog
// moves (worklogs are WorkLogger's own subject).
var quietFields = map[string]bool{
	"rank": true, "timespent": true, "timeestimate": true, "timeoriginalestimate": true, "worklogid": true,
}

// Activity is your Jira activity for the days (inclusive): the issues `updatedBy` finds (creating one, changing
// a field, commenting all count), read with their changelog and comments, and only your own entries kept.
// Known limit: a search returns an issue's latest 100 changes; an issue busier than that in the range loses the
// oldest (GET /issue/{key}/changelog would page them).
func (c *Client) Activity(ctx context.Context, accountID, fromDay, toDay string) ([]Activity, error) {
	start, _ := c.Days.Bounds(fromDay)
	_, end := c.Days.Bounds(toDay)
	// updatedBy takes days; the day after the range makes its end inclusive, and parseActivity trims to the range.
	jql := fmt.Sprintf(`issue in updatedBy("%s", "%s", "%s") ORDER BY updated DESC`,
		strings.ReplaceAll(accountID, `"`, ""), fromDay, c.Days.Shift(toDay, 1))
	out := []Activity{}
	token := ""
	for {
		q := url.Values{
			"jql": {jql}, "fields": {"summary,created,creator,comment"}, "expand": {"changelog"}, "maxResults": {"50"},
		}
		if token != "" {
			q.Set("nextPageToken", token)
		}
		data, err := c.jiraSend(ctx, "GET", "/rest/api/3/search/jql?"+q.Encode(), nil, "Reading your Jira activity")
		if err != nil {
			return nil, err
		}
		page, next, err := parseActivity(data, accountID, start, end)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			break
		}
		token = next
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

// jiraTime is how Jira writes times: 2026-03-03T09:00:00.000+0100.
const jiraTime = "2006-01-02T15:04:05.000-0700"

type jiraUser struct {
	AccountID string `json:"accountId"`
}

// parseActivity reads one search page: your entries in [start, end), and the next page's token ("" at the end).
func parseActivity(data []byte, me string, start, end time.Time) ([]Activity, string, error) {
	var page struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string    `json:"summary"`
				Created string    `json:"created"`
				Creator *jiraUser `json:"creator"`
				Comment *struct {
					Comments []struct {
						Author  *jiraUser `json:"author"`
						Created string    `json:"created"`
						Body    *ADF      `json:"body"`
					} `json:"comments"`
				} `json:"comment"`
			} `json:"fields"`
			Changelog *struct {
				Histories []struct {
					Author  *jiraUser `json:"author"`
					Created string    `json:"created"`
					Items   []struct {
						Field      string  `json:"field"`
						FromString *string `json:"fromString"`
						ToString   *string `json:"toString"`
					} `json:"items"`
				} `json:"histories"`
			} `json:"changelog"`
		} `json:"issues"`
		NextPageToken *string `json:"nextPageToken"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, "", err
	}
	mine := func(u *jiraUser, at string) (time.Time, bool) {
		t, err := time.Parse(jiraTime, at)
		return t, err == nil && u != nil && u.AccountID == me && !t.Before(start) && t.Before(end)
	}
	var out []Activity
	for _, is := range page.Issues {
		base := Activity{IssueKey: is.Key, Summary: is.Fields.Summary}
		if t, ok := mine(is.Fields.Creator, is.Fields.Created); ok {
			a := base
			a.At, a.Kind = t, "created"
			out = append(out, a)
		}
		if is.Changelog != nil {
			for _, h := range is.Changelog.Histories {
				t, ok := mine(h.Author, h.Created)
				if !ok {
					continue
				}
				for _, it := range h.Items {
					if quietFields[strings.ToLower(it.Field)] {
						continue
					}
					a := base
					a.At, a.Kind, a.Field, a.From, a.To = t, "change", it.Field, deref(it.FromString), deref(it.ToString)
					out = append(out, a)
				}
			}
		}
		if is.Fields.Comment != nil {
			for _, cm := range is.Fields.Comment.Comments {
				if t, ok := mine(cm.Author, cm.Created); ok {
					a := base
					a.At, a.Kind = t, "comment"
					if cm.Body != nil {
						a.Text = strings.TrimSpace(cm.Body.Text())
					}
					out = append(out, a)
				}
			}
		}
	}
	next := ""
	if page.NextPageToken != nil {
		next = *page.NextPageToken
	}
	return out, next, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
