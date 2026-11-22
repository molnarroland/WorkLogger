package tempo

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// Issue is a Jira issue a search found.
type Issue struct {
	Key, Summary, Status string
	// Description is the start of the issue's description as plain text (at most descriptionRunes), "" if none.
	Description string
	Created     time.Time
}

// descriptionRunes is how much of an issue's description a search keeps.
const descriptionRunes = 280

// issueFields is what a search reads of each issue.
const issueFields = "summary,created,status,description"

// maxSearchResults is how many issues one search returns.
const maxSearchResults = 50

// SearchIssues finds the issues created in [fromDay, toDay] (inclusive) whose summary has every word of text (as
// a prefix), newest first, at most maxSearchResults. A text that is an issue key also finds that issue, whenever
// it was created.
func (c *Client) SearchIssues(ctx context.Context, fromDay, toDay, text string) ([]Issue, error) {
	out := []Issue{}
	seen := map[string]bool{}
	if key := strings.ToUpper(strings.TrimSpace(text)); issueKeyLike(key) {
		var is searchIssue
		// A key that doesn't exist (or isn't visible) is no error here: the search below still runs.
		if err := c.jira(ctx, "GET", "/rest/api/3/issue/"+url.PathEscape(key)+"?fields="+issueFields, nil, "Looking up "+key, &is); err == nil && is.Key != "" {
			out = append(out, is.issue())
			seen[is.Key] = true
		}
	}
	q := url.Values{
		"jql":        {SearchJQL(fromDay, c.Days.Shift(toDay, 1), text)},
		"fields":     {issueFields},
		"maxResults": {fmt.Sprint(maxSearchResults)},
	}
	var page struct {
		Issues []searchIssue `json:"issues"`
	}
	if err := c.jira(ctx, "GET", "/rest/api/3/search/jql?"+q.Encode(), nil, "Searching Jira issues", &page); err != nil {
		return nil, err
	}
	for _, is := range page.Issues {
		if !seen[is.Key] {
			seen[is.Key] = true
			out = append(out, is.issue())
		}
	}
	return out, nil
}

type searchIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Created string `json:"created"`
		Status  *struct {
			Name string `json:"name"`
		} `json:"status"`
		Description *ADF `json:"description"`
	} `json:"fields"`
}

func (s searchIssue) issue() Issue {
	is := Issue{Key: s.Key, Summary: s.Fields.Summary}
	if t, err := time.Parse(jiraTime, s.Fields.Created); err == nil {
		is.Created = t
	}
	if s.Fields.Status != nil {
		is.Status = s.Fields.Status.Name
	}
	if s.Fields.Description != nil {
		is.Description = Excerpt(s.Fields.Description.Words(), descriptionRunes)
	}
	return is
}

// Excerpt is text on one line (runs of white space made one space), cut at a word to at most n runes with "…".
func Excerpt(text string, n int) string {
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) <= n {
		return text
	}
	cut := string(r[:n-1])
	if i := strings.LastIndexByte(cut, ' '); i > len(cut)/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:") + "…"
}

// SearchJQL is the JQL for issues created in [fromDay, untilDay) with every word of text in their summary.
// Words keep only letters and digits, so nothing typed can break out of the query or upset Jira's text search, and
// go lower-case, so "OR" or "NOT" is a word and not an operator.
func SearchJQL(fromDay, untilDay, text string) string {
	jql := fmt.Sprintf(`created >= "%s" AND created < "%s"`, fromDay, untilDay)
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for _, w := range words {
		jql += fmt.Sprintf(` AND summary ~ "%s*"`, strings.ToLower(w))
	}
	return jql + " ORDER BY created DESC"
}

// issueKeyLike: PROJ-123.
func issueKeyLike(s string) bool {
	dash := strings.LastIndexByte(s, '-')
	if dash < 1 || dash == len(s)-1 || !unicode.IsLetter(rune(s[0])) {
		return false
	}
	for _, r := range s[:dash] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	for _, r := range s[dash+1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
