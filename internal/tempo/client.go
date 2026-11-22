// Package tempo talks to Jira Cloud and Tempo (REST v4) for one Jira account.
package tempo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// missing is a setting the user still has to give (a core.UserError with a "missing." code).
func missing(what string) error { return core.Err("missing." + what) }

type HTTPError struct {
	What string
	Code int
	Body string
}

func (e HTTPError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300]
	}
	return fmt.Sprintf("%s failed (HTTP %d): %s", e.What, e.Code, body)
}

// Worklog is a worklog that already exists in Tempo or Jira (yours, read back for the selected dates).
type Worklog struct {
	ID          int64
	IssueID     int64
	IssueKey    string // filled from the Jira issue id; "" when it can't be looked up
	Day         string // as Tempo stores it
	StartTime   string // HH:mm:ss
	Seconds     int
	Description string
}

// Payload is the body Tempo v4 expects for POST /4/worklogs (issueKey kept for readability; issueId resolved on push).
type Payload struct {
	IssueKey         string  `json:"issueKey"`
	IssueID          *int64  `json:"issueId,omitempty"`
	AuthorAccountID  *string `json:"authorAccountId,omitempty"`
	StartDate        string  `json:"startDate"`
	StartTime        string  `json:"startTime"`
	TimeSpentSeconds int     `json:"timeSpentSeconds"`
	Description      string  `json:"description"`
}

// Service is the Jira and worklog-provider calls for one account; Client is the real one.
type Service interface {
	Site() string
	ProjectKeys(ctx context.Context) ([]string, error)
	AccountID(ctx context.Context) (string, error)
	IssueID(ctx context.Context, key string) (int64, error)
	IssueKey(ctx context.Context, id int64) (string, error)
	Create(ctx context.Context, p Payload) (int64, error)
	Worklogs(ctx context.Context, accountID, fromDay, toDay string) ([]Worklog, error)
}

type Client struct {
	JiraBaseURL, JiraEmail, JiraToken string
	Provider                          core.WorklogProvider
	TempoBaseURL, TempoToken          string
	HTTP                              core.HTTPDoer
	Days                              core.Days

	accounts accountCache // the issues' Tempo accounts, looked up once per client
}

func NewClient(a core.JiraAccount, secrets core.SecretStore, h core.HTTPDoer, days core.Days) (*Client, error) {
	jira, _ := secrets.Get(a.JiraTokenKey())
	tempo, _ := secrets.Get(a.TempoTokenKey())
	c := &Client{
		JiraBaseURL: a.Site(), JiraEmail: a.Email, JiraToken: jira, Provider: a.Provider,
		TempoBaseURL: strings.Trim(a.TempoBaseURL, "/ "), TempoToken: tempo, HTTP: h, Days: days,
	}
	switch {
	case c.JiraBaseURL == "":
		return nil, missing("jiraSite")
	case c.JiraEmail == "" || c.JiraToken == "":
		return nil, missing("jiraToken")
	case c.Provider == core.ProviderTempo && c.TempoToken == "":
		return nil, missing("tempoToken")
	}
	return c, nil
}

func (c *Client) Site() string { return c.JiraBaseURL }

func basicAuth(email, token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+token))
}

func (c *Client) send(ctx context.Context, method, rawURL, auth string, body any, what string) ([]byte, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, r)
	if err != nil {
		return nil, missing("url")
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, HTTPError{what, resp.StatusCode, string(data)}
	}
	return data, nil
}

func (c *Client) jira(ctx context.Context, method, path string, body any, what string, out any) error {
	data, err := c.jiraSend(ctx, method, path, body, what)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (c *Client) AccountID(ctx context.Context) (string, error) {
	var me struct {
		AccountID string `json:"accountId"`
	}
	err := c.jira(ctx, "GET", "/rest/api/3/myself", nil, "Reading your Jira account", &me)
	return me.AccountID, err
}

func (c *Client) IssueID(ctx context.Context, key string) (int64, error) {
	var issue struct {
		ID string `json:"id"`
	}
	what := "Looking up " + key
	if err := c.jira(ctx, "GET", "/rest/api/3/issue/"+url.PathEscape(key)+"?fields=summary", nil, what, &issue); err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(issue.ID, 10, 64)
	if err != nil {
		return 0, HTTPError{what, 200, "non-numeric id"}
	}
	return id, nil
}

func (c *Client) IssueKey(ctx context.Context, id int64) (string, error) {
	var issue struct {
		Key string `json:"key"`
	}
	err := c.jira(ctx, "GET", fmt.Sprintf("/rest/api/3/issue/%d?fields=summary", id), nil, fmt.Sprintf("Looking up %d", id), &issue)
	return issue.Key, err
}

// ProjectKeys is the site's project keys (GET /rest/api/3/project/search, all pages).
func (c *Client) ProjectKeys(ctx context.Context) ([]string, error) {
	keys := []string{}
	for {
		var page struct {
			Values []struct {
				Key string `json:"key"`
			} `json:"values"`
			IsLast *bool `json:"isLast"`
		}
		if err := c.jira(ctx, "GET", fmt.Sprintf("/rest/api/3/project/search?maxResults=100&startAt=%d", len(keys)), nil,
			"Reading the Jira projects", &page); err != nil {
			return nil, err
		}
		for _, v := range page.Values {
			keys = append(keys, v.Key)
		}
		if page.IsLast == nil || *page.IsLast || len(page.Values) == 0 {
			return keys, nil
		}
	}
}

// Worklogs is your worklogs for the days (inclusive), from Tempo or from Jira.
func (c *Client) Worklogs(ctx context.Context, accountID, fromDay, toDay string) ([]Worklog, error) {
	if c.Provider == core.ProviderJira {
		return c.jiraWorklogs(ctx, accountID, fromDay, toDay)
	}
	return c.tempoWorklogs(ctx, accountID, fromDay, toDay)
}

// tempoWorklogs: GET /4/worklogs/user/{accountId}?from&to, following metadata.next through the pages.
func (c *Client) tempoWorklogs(ctx context.Context, accountID, fromDay, toDay string) ([]Worklog, error) {
	q := url.Values{"from": {fromDay}, "to": {toDay}, "limit": {"1000"}}
	next := c.TempoBaseURL + "/worklogs/user/" + url.PathEscape(accountID) + "?" + q.Encode()
	if _, err := url.Parse(next); err != nil || !strings.HasPrefix(next, "http") {
		return nil, missing("tempoUrl")
	}
	out := []Worklog{}
	for next != "" {
		data, err := c.send(ctx, "GET", next, "Bearer "+c.TempoToken, nil, "Reading your Tempo worklogs")
		if err != nil {
			return nil, err
		}
		var page struct {
			Metadata *struct {
				Next *string `json:"next"`
			} `json:"metadata"`
			Results []struct {
				TempoWorklogID int64 `json:"tempoWorklogId"`
				Issue          struct {
					ID int64 `json:"id"`
				} `json:"issue"`
				TimeSpentSeconds int     `json:"timeSpentSeconds"`
				StartDate        string  `json:"startDate"`
				StartTime        *string `json:"startTime"`
				Description      *string `json:"description"`
			} `json:"results"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, err
		}
		for _, r := range page.Results {
			w := Worklog{ID: r.TempoWorklogID, IssueID: r.Issue.ID, Day: r.StartDate, StartTime: "00:00:00", Seconds: r.TimeSpentSeconds}
			if r.StartTime != nil {
				w.StartTime = *r.StartTime
			}
			if r.Description != nil {
				w.Description = *r.Description
			}
			out = append(out, w)
		}
		next = ""
		if page.Metadata != nil && page.Metadata.Next != nil {
			next = *page.Metadata.Next
		}
	}
	return out, nil
}

// jiraWorklogs: the issues you logged on in the range (JQL), then each issue's worklogs that are yours.
func (c *Client) jiraWorklogs(ctx context.Context, accountID, fromDay, toDay string) ([]Worklog, error) {
	jql := fmt.Sprintf(`worklogAuthor = currentUser() AND worklogDate >= "%s" AND worklogDate <= "%s"`, fromDay, toDay)
	what := "Reading your Jira worklogs"
	type issue struct{ ID, Key string }
	var issues []issue
	token := ""
	for {
		q := url.Values{"jql": {jql}, "fields": {"key"}, "maxResults": {"100"}}
		if token != "" {
			q.Set("nextPageToken", token)
		}
		var page struct {
			Issues        []issue `json:"issues"`
			NextPageToken *string `json:"nextPageToken"`
		}
		if err := c.jira(ctx, "GET", "/rest/api/3/search/jql?"+q.Encode(), nil, what, &page); err != nil {
			return nil, err
		}
		issues = append(issues, page.Issues...)
		if page.NextPageToken == nil {
			break
		}
		token = *page.NextPageToken
	}
	start, _ := c.Days.Bounds(fromDay)
	_, end := c.Days.Bounds(toDay)
	out := []Worklog{}
	for _, is := range issues {
		issueID, err := strconv.ParseInt(is.ID, 10, 64)
		if err != nil {
			continue
		}
		for startAt := 0; ; {
			var page struct {
				Worklogs []struct {
					ID     string `json:"id"`
					Author *struct {
						AccountID string `json:"accountId"`
					} `json:"author"`
					Started          string `json:"started"` // 2026-03-03T09:00:00.000+0100
					TimeSpentSeconds int    `json:"timeSpentSeconds"`
					Comment          *ADF   `json:"comment"`
				} `json:"worklogs"`
				Total *int `json:"total"`
			}
			path := fmt.Sprintf("/rest/api/3/issue/%s/worklog?startedAfter=%d&startedBefore=%d&startAt=%d&maxResults=1000",
				is.ID, start.UnixMilli(), end.UnixMilli(), startAt)
			if err := c.jira(ctx, "GET", path, nil, what, &page); err != nil {
				return nil, err
			}
			for _, w := range page.Worklogs {
				if w.Author == nil || w.Author.AccountID != accountID || len(w.Started) < 19 {
					continue
				}
				day := w.Started[:10]
				id, err := strconv.ParseInt(w.ID, 10, 64)
				if day < fromDay || day > toDay || err != nil {
					continue
				}
				desc := ""
				if w.Comment != nil {
					desc = w.Comment.Text()
				}
				out = append(out, Worklog{
					ID: id, IssueID: issueID, IssueKey: is.Key, Day: day,
					StartTime: w.Started[11:19], Seconds: w.TimeSpentSeconds, Description: desc,
				})
			}
			startAt += len(page.Worklogs)
			total := 0
			if page.Total != nil {
				total = *page.Total
			}
			if len(page.Worklogs) == 0 || startAt >= total {
				break
			}
		}
	}
	return out, nil
}

// Create makes a worklog (in Tempo, or on the Jira issue) and returns its id.
func (c *Client) Create(ctx context.Context, p Payload) (int64, error) {
	what := "Creating the " + p.IssueKey + " worklog"
	if c.Provider == core.ProviderJira {
		hm := p.StartTime
		if len(hm) > 5 {
			hm = hm[:5]
		}
		start, ok := c.Days.AtHM(p.StartDate, hm)
		if !ok {
			return 0, missing("start")
		}
		body := map[string]any{
			"started":          start.In(c.Days.Loc).Format("2006-01-02T15:04:05.000-0700"),
			"timeSpentSeconds": p.TimeSpentSeconds, "comment": NewADF(p.Description),
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := c.jira(ctx, "POST", "/rest/api/3/issue/"+url.PathEscape(p.IssueKey)+"/worklog", body, what, &created); err != nil {
			return 0, err
		}
		id, err := strconv.ParseInt(created.ID, 10, 64)
		if err != nil {
			return 0, HTTPError{what, 200, "non-numeric id"}
		}
		return id, nil
	}
	if p.IssueID == nil || p.AuthorAccountID == nil {
		return 0, missing("ids")
	}
	body := map[string]any{
		"issueId": *p.IssueID, "authorAccountId": *p.AuthorAccountID, "startDate": p.StartDate,
		"startTime": p.StartTime, "timeSpentSeconds": p.TimeSpentSeconds, "description": p.Description,
	}
	// Sites that don't require an account take the worklog without one, so a failed lookup only explains a refusal.
	account, why, err := c.IssueAccount(ctx, *p.IssueID)
	if err != nil {
		why = "Its account couldn't be read: " + err.Error()
	}
	if account != "" {
		body["attributes"] = []map[string]string{{"key": AccountAttribute, "value": account}}
	}
	data, err := c.send(ctx, "POST", c.TempoBaseURL+"/worklogs", "Bearer "+c.TempoToken, body, what)
	var he HTTPError
	if errors.As(err, &he) && account == "" && strings.Contains(he.Body, AccountAttribute) {
		return 0, HTTPError{what, he.Code, why + ". Set its Account in Jira (or a default account for the project in Tempo). " + he.Body}
	}
	if err != nil {
		return 0, err
	}
	var created struct {
		ID int64 `json:"tempoWorklogId"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return 0, err
	}
	return created.ID, nil
}

// ADF is Jira's rich text (Atlassian Document Format), reduced to plain paragraphs.
type ADF struct {
	Type    string    `json:"type"`
	Version int       `json:"version,omitempty"`
	Content []ADFNode `json:"content"`
}

type ADFNode struct {
	Type    string    `json:"type"`
	Text    *string   `json:"text,omitempty"`
	Content []ADFNode `json:"content,omitempty"`
}

func NewADF(text string) ADF {
	para := ADFNode{Type: "paragraph", Content: []ADFNode{}}
	if text != "" {
		para.Content = []ADFNode{{Type: "text", Text: &text}}
	}
	return ADF{Type: "doc", Version: 1, Content: []ADFNode{para}}
}

// Text is all text nodes; paragraphs joined by spaces.
func (a ADF) Text() string {
	var flat func(n ADFNode) string
	flat = func(n ADFNode) string {
		if n.Text != nil {
			return *n.Text
		}
		s := ""
		for _, c := range n.Content {
			s += flat(c)
		}
		return s
	}
	var parts []string
	for _, n := range a.Content {
		if t := flat(n); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// Words is all text nodes with blocks (paragraphs, list items, headings…) and line breaks kept apart by a space,
// where Text runs a list's items together.
func (a ADF) Words() string {
	var b strings.Builder
	var walk func(n ADFNode)
	walk = func(n ADFNode) {
		switch {
		case n.Text != nil:
			b.WriteString(*n.Text)
		case n.Type == "hardBreak":
			b.WriteByte(' ')
		default:
			for _, c := range n.Content {
				walk(c)
			}
			b.WriteByte(' ')
		}
	}
	for _, n := range a.Content {
		walk(n)
	}
	return strings.TrimSpace(b.String())
}

// Coverage is the "KEY|day" ticket-days whose time Tempo already has: Tempo's time for the ticket on that day is
// at least what WorkLogger has (a minute of rounding allowed). Entries without a ticket never match.
func Coverage(logs []core.Worklog, tempo []Worklog) map[string]bool {
	inTempo := map[string]int{}
	for _, t := range tempo {
		if t.IssueKey != "" {
			inTempo[t.IssueKey+"|"+t.Day] += t.Seconds
		}
	}
	local := map[string]int{}
	for _, w := range logs {
		if w.Duration > 0 && w.IssueKey != nil {
			local[*w.IssueKey+"|"+w.Day] += w.Duration
		}
	}
	out := map[string]bool{}
	for ref, seconds := range local {
		if inTempo[ref] >= seconds-60 {
			out[ref] = true
		}
	}
	return out
}

// IssueCache is the store's Jira issue id cache.
type IssueCache interface {
	CachedIssueID(key, site string) (int64, bool)
	CachedIssueKey(id int64, site string) (string, bool)
	CacheIssueID(key string, id int64, site string) error
}

// Read is your worklogs for a range with their issues named. Tempo v4 only returns issue ids, so keys are looked
// up once and kept in the cache (the same one pushing uses).
func Read(ctx context.Context, cache IssueCache, c Service, accountID, fromDay, toDay string) ([]Worklog, error) {
	logs, err := c.Worklogs(ctx, accountID, fromDay, toDay)
	if err != nil {
		return nil, err
	}
	keys := map[int64]string{}
	for i := range logs {
		if logs[i].IssueKey != "" {
			continue // Jira worklogs come with their keys
		}
		logs[i].IssueKey = issueKeyOf(ctx, cache, c, logs[i].IssueID, keys)
	}
	return logs, nil
}

func issueID(ctx context.Context, cache IssueCache, c Service, key string) (int64, error) {
	if id, ok := cache.CachedIssueID(key, c.Site()); ok {
		return id, nil
	}
	id, err := c.IssueID(ctx, key)
	if err != nil {
		return 0, err
	}
	return id, cache.CacheIssueID(key, id, c.Site())
}

// Push creates the worklogs one request per entry and calls sent for each one that worked (with its new id).
// progress gets the 1-based index of the entry being sent and the number of entries.
func Push(ctx context.Context, cache IssueCache, c Service, logs []core.Worklog, accountID string, days core.Days,
	sent func(w core.Worklog, tempoID int64) error, progress func(i, n int),
) (int, []string) {
	pending := Exportable(logs)
	payloads := Payloads(pending, accountID, days)
	n := 0
	var failures []string
	for i, w := range pending {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress(i+1, len(pending))
		}
		p := payloads[i]
		err := func() error {
			id, err := issueID(ctx, cache, c, p.IssueKey)
			if err != nil {
				return err
			}
			p.IssueID = &id
			tempoID, err := c.Create(ctx, p)
			if err != nil {
				return err
			}
			return sent(w, tempoID)
		}()
		if err != nil {
			failures = append(failures, p.IssueKey+" "+p.StartDate+": "+err.Error())
			continue
		}
		n++
	}
	return n, failures
}
