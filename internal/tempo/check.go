package tempo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Check tests the Jira site, email and token, and the Tempo token, with harmless calls, and infers which
// permissions (scopes) the tokens have: 401 reads as "token rejected", 403 as "missing permission" and 404 as
// "wrong address".
type Check struct {
	Account    core.JiraAccount
	JiraToken  string
	TempoToken string
	HTTP       core.HTTPDoer
	Today      string
}

func NewCheck(a core.JiraAccount, secrets core.SecretStore, h core.HTTPDoer, today string) Check {
	j, _ := secrets.Get(a.JiraTokenKey())
	t, _ := secrets.Get(a.TempoTokenKey())
	return Check{a, j, t, h, today}
}

func ok(b bool) *bool { return &b }

type answer struct {
	data        []byte
	code        int    // 0 when unreachable
	unreachable string // the error when the request didn't go through
}

func (a answer) ok() bool { return a.code >= 200 && a.code < 300 }

func (c Check) send(ctx context.Context, method, u, auth, body string) answer {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return answer{unreachable: u}
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return answer{unreachable: err.Error()}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return answer{data: data, code: resp.StatusCode}
}

func httpDetail(who string, code int) string {
	return who + " answered with HTTP " + strconv.Itoa(code) + "."
}

// Jira checks the site answers, the email + token sign in (your account), and issues can be read
// (read:jira-work). Returns the lines and, when signed in, your Jira account id.
func (c Check) Jira(ctx context.Context) ([]Line, string) {
	signIn, work := "Site, email and API token", "Read issues (read:jira-work)"
	site := c.Account.Site()
	if u, err := url.Parse(site); err != nil || u.Host == "" {
		return []Line{{signIn, "Enter your Jira site, e.g. https://yourcompany.atlassian.net.", ok(false)}, {work, "", nil}}, ""
	}
	if c.Account.Email == "" || c.JiraToken == "" {
		return []Line{{signIn, "Enter your Jira email and API token.", ok(false)}, {work, "", nil}}, ""
	}
	auth := basicAuth(c.Account.Email, c.JiraToken)
	api, scoped := site, ""
	me := c.send(ctx, "GET", api+"/rest/api/3/myself", auth, "")
	if me.code == 401 { // a token with scopes works only through the gateway
		if g, err := gateway(ctx, c.HTTP, site, "jira"); err == nil {
			if again := c.send(ctx, "GET", g+"/rest/api/3/myself", auth, ""); again.code != 401 {
				api, me, scoped = g, again, " A token with scopes: WorkLogger uses it through api.atlassian.com."
			}
		}
	}
	var lines []Line
	accountID := ""
	switch {
	case me.ok():
		var user struct {
			AccountID   string  `json:"accountId"`
			DisplayName *string `json:"displayName"`
		}
		name := "?"
		if json.Unmarshal(me.data, &user) == nil {
			accountID = user.AccountID
			if user.DisplayName != nil {
				name = *user.DisplayName
			} else if user.AccountID != "" {
				name = user.AccountID
			}
		}
		lines = append(lines, Line{signIn, "Signed in as " + name + " (read:jira-user)." + scoped, ok(true)})
	case me.code == 401:
		lines = append(lines, Line{signIn, "Jira rejected the email or API token, on the site and through api.atlassian.com. Check both, or create a new token.", ok(false)})
	case me.code == 403:
		lines = append(lines, Line{signIn, "The token can't read your account: it needs read:jira-user.", ok(false)})
	case me.code == 404:
		lines = append(lines, Line{signIn, "No Jira at this address. Use your site, e.g. https://yourcompany.atlassian.net.", ok(false)})
	case me.code != 0:
		lines = append(lines, Line{signIn, httpDetail("Jira", me.code), ok(false)})
	default:
		lines = append(lines, Line{signIn, "Can't reach the Jira site: " + me.unreachable, ok(false)})
	}
	if accountID == "" {
		return append(lines, Line{work, "", nil}), ""
	}
	switch p := c.send(ctx, "GET", api+"/rest/api/3/project/search?maxResults=1", auth, ""); {
	case p.ok():
		lines = append(lines, Line{work, "Issue keys can be looked up.", ok(true)})
	case p.code == 401 || p.code == 403:
		lines = append(lines, Line{work, "The token can't read issues: it needs read:jira-work (needed to send to Tempo and to name Tempo's issues).", ok(false)})
	case p.code != 0:
		lines = append(lines, Line{work, httpDetail("Jira", p.code), ok(false)})
	default:
		lines = append(lines, Line{work, p.unreachable, ok(false)})
	}
	return lines, accountID
}

// Tempo checks the token is accepted and can view your worklogs; creating is probed with an empty worklog, which
// Tempo rejects as invalid (400) when the token may create, and refuses (403) when it may not.
func (c Check) Tempo(ctx context.Context, accountID string) []Line {
	view, manage := "View worklogs", "Create worklogs (for Send to Tempo)"
	if c.TempoToken == "" {
		return []Line{{view, "Enter your Tempo API token.", ok(false)}, {manage, "", nil}}
	}
	if accountID == "" {
		return []Line{{view, "Check Jira first: Tempo needs your Jira account id.", nil}, {manage, "", nil}}
	}
	base := strings.Trim(c.Account.TempoBaseURL, "/ ")
	auth := "Bearer " + c.TempoToken
	var lines []Line
	switch a := c.send(ctx, "GET", base+"/worklogs/user/"+accountID+"?from="+c.Today+"&to="+c.Today+"&limit=1", auth, ""); {
	case a.ok():
		lines = append(lines, Line{view, "Your worklogs can be read.", ok(true)})
	case a.code == 401:
		return []Line{{view, "Tempo rejected the token: it is wrong, expired or revoked.", ok(false)}, {manage, "", nil}}
	case a.code == 403:
		lines = append(lines, Line{view, "The token can't view worklogs: give it the “View worklogs” permission.", ok(false)})
	case a.code == 404:
		return []Line{{view, "No Tempo API at this address. Use https://api.tempo.io/4 (or your region's, e.g. https://api.eu.tempo.io/4).", ok(false)}, {manage, "", nil}}
	case a.code != 0:
		lines = append(lines, Line{view, httpDetail("Tempo", a.code), ok(false)})
	default:
		return []Line{{view, "Can't reach Tempo: " + a.unreachable, ok(false)}, {manage, "", nil}}
	}
	switch a := c.send(ctx, "POST", base+"/worklogs", auth, "{}"); a.code {
	case 400:
		lines = append(lines, Line{manage, "Probably allowed (an empty test worklog was refused as invalid, nothing was saved).", ok(true)})
	case 401, 403:
		lines = append(lines, Line{manage, "The token can't create worklogs: give it the “Manage worklogs” permission (only needed for Send to Tempo).", ok(false)})
	case 0:
		lines = append(lines, Line{manage, a.unreachable, nil})
	default:
		lines = append(lines, Line{manage, "Couldn't tell from Tempo's answer.", nil})
	}
	return lines
}
