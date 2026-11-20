package tempo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
)

// AccountAttribute is Tempo's work attribute for the account a worklog is booked on. Sites that make it required
// refuse a worklog without it (HTTP 400), so it is copied from the issue's Account field, or else the default
// account linked to the issue's project.
const AccountAttribute = "_Account_"

// isAccountField: the schema of the Account field Tempo adds to Jira issues, as a Connect app field or as a Forge
// one ("ari:cloud:ecosystem::extension/<app>/<env>/static/account").
func isAccountField(custom string) bool {
	return custom == "io.tempo.jira__account" || strings.HasSuffix(custom, "/static/account")
}

// issueAccount is what was found for one issue: the account key, or why there is none.
type issueAccount struct {
	key, why string
}

type accountCache struct {
	mu        sync.Mutex
	byIssue   map[int64]issueAccount
	byProject map[string]string // Jira project id → default account key ("" when none)
	keyByID   map[int64]string  // Tempo account id → key; nil until read
}

// IssueAccount is the key of the Tempo account to book the issue's time on: the issue's Account field, else the
// default account linked to its project. "" with the reason when there is none.
func (c *Client) IssueAccount(ctx context.Context, issueID int64) (key, why string, err error) {
	c.accounts.mu.Lock()
	defer c.accounts.mu.Unlock()
	if a, ok := c.accounts.byIssue[issueID]; ok {
		return a.key, a.why, nil
	}
	a, err := c.lookUpAccount(ctx, issueID)
	if err != nil {
		return "", "", err
	}
	if c.accounts.byIssue == nil {
		c.accounts.byIssue = map[int64]issueAccount{}
	}
	c.accounts.byIssue[issueID] = a
	return a.key, a.why, nil
}

func (c *Client) lookUpAccount(ctx context.Context, issueID int64) (issueAccount, error) {
	// All fields with their schemas: the Account field is found by its type, without the field list (which a
	// scoped token may not be allowed to read).
	var issue struct {
		Key    string                     `json:"key"`
		Fields map[string]json.RawMessage `json:"fields"`
		Schema map[string]struct {
			Custom string `json:"custom"`
		} `json:"schema"`
		Names map[string]string `json:"names"`
	}
	what := fmt.Sprintf("Reading the account of issue %d", issueID)
	if err := c.jira(ctx, "GET", fmt.Sprintf("/rest/api/3/issue/%d?fields=*all&expand=names,schema", issueID), nil, what, &issue); err != nil {
		return issueAccount{}, err
	}
	field := ""
	for id, s := range issue.Schema {
		if isAccountField(s.Custom) {
			field = id
		}
	}
	if field == "" {
		for id, name := range issue.Names {
			if strings.EqualFold(name, "Account") && strings.HasPrefix(id, "customfield_") {
				field = id
			}
		}
	}
	if field != "" {
		key, err := c.accountKey(ctx, issue.Fields[field])
		if err != nil || key != "" {
			return issueAccount{key: key}, err
		}
	}
	var project struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	_ = json.Unmarshal(issue.Fields["project"], &project)
	if project.ID == "" {
		return issueAccount{why: issue.Key + " has no Account set"}, nil
	}
	key, err := c.projectAccount(ctx, project.ID)
	if err != nil {
		return issueAccount{}, err
	}
	if key == "" {
		return issueAccount{why: fmt.Sprintf("%s has no Account set, and project %s has no default Tempo account", issue.Key, project.Key)}, nil
	}
	return issueAccount{key: key}, nil
}

// projectAccount is the default account linked to the Jira project in Tempo (GET /4/account-links/project/{id});
// the only linked account when none is marked default.
func (c *Client) projectAccount(ctx context.Context, projectID string) (string, error) {
	if key, ok := c.accounts.byProject[projectID]; ok {
		return key, nil
	}
	data, err := c.send(ctx, "GET", c.TempoBaseURL+"/account-links/project/"+url.PathEscape(projectID), "Bearer "+c.TempoToken, nil,
		"Reading the Tempo accounts linked to project "+projectID)
	if err != nil {
		return "", err
	}
	type link struct {
		Default bool            `json:"default"`
		Account json.RawMessage `json:"account"`
	}
	var links []link
	if json.Unmarshal(data, &links) != nil {
		var page struct {
			Results []link `json:"results"`
		}
		_ = json.Unmarshal(data, &page)
		links = page.Results
	}
	var pick *link
	for i := range links {
		if links[i].Default {
			pick = &links[i]
		}
	}
	if pick == nil && len(links) == 1 {
		pick = &links[0]
	}
	key := ""
	if pick != nil {
		if key, err = c.accountKey(ctx, pick.Account); err != nil {
			return "", err
		}
	}
	if c.accounts.byProject == nil {
		c.accounts.byProject = map[string]string{}
	}
	c.accounts.byProject[projectID] = key
	return key, nil
}

// accountKey reads an account reference: the issue's Account field ({"id": 12, "value": "<name>",
// "optionProperties": {"key": "<key>", …}}) or an account link's account ({"self": ".../accounts/<key>"} or with
// key/id). Tempo wants the key; a bare id is looked up in Tempo's account list (which needs accounts:view).
func (c *Client) accountKey(ctx context.Context, raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var v struct {
		ID   json.Number `json:"id"`
		Key  string      `json:"key"`
		Self string      `json:"self"`
		Opts *struct {
			Key string `json:"key"`
		} `json:"optionProperties"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		var s string // a plain string is the key
		_ = json.Unmarshal(raw, &s)
		return s, nil
	}
	if v.Key != "" {
		return v.Key, nil
	}
	if v.Opts != nil && v.Opts.Key != "" {
		return v.Opts.Key, nil
	}
	id, err := strconv.ParseInt(v.ID.String(), 10, 64)
	if err != nil {
		if v.Self == "" {
			return "", nil
		}
		last := path.Base(strings.TrimRight(v.Self, "/"))
		if id, err = strconv.ParseInt(last, 10, 64); err != nil {
			return last, nil // .../accounts/<key>
		}
	}
	if err := c.readAccountKeys(ctx); err != nil {
		return "", err
	}
	if key, ok := c.accounts.keyByID[id]; ok {
		return key, nil
	}
	return "", HTTPError{"Reading the Tempo accounts", 200, fmt.Sprintf("no account with id %d", id)}
}

// readAccountKeys reads every Tempo account's id and key once (GET /4/accounts, all pages).
func (c *Client) readAccountKeys(ctx context.Context) error {
	if c.accounts.keyByID != nil {
		return nil
	}
	keys := map[int64]string{}
	next := c.TempoBaseURL + "/accounts?limit=1000"
	for next != "" {
		data, err := c.send(ctx, "GET", next, "Bearer "+c.TempoToken, nil, "Reading the Tempo accounts")
		if err != nil {
			return err
		}
		var page struct {
			Metadata *struct {
				Next *string `json:"next"`
			} `json:"metadata"`
			Results []struct {
				ID  int64  `json:"id"`
				Key string `json:"key"`
			} `json:"results"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return err
		}
		for _, a := range page.Results {
			keys[a.ID] = a.Key
		}
		next = ""
		if page.Metadata != nil && page.Metadata.Next != nil {
			next = *page.Metadata.Next
		}
	}
	c.accounts.keyByID = keys
	return nil
}
