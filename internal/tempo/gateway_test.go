package tempo

import (
	"net/http"
	"strings"
	"testing"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// A token with scopes: the site answers 401, the gateway (found through tenant_info) signs in.
func TestScopedTokenGoesThroughTheGateway(t *testing.T) {
	a := account()
	a.BaseURL = "https://scoped.atlassian.net" // its own site: the gateway is remembered per site
	scoped := func(r *http.Request, _ string) (int, string) {
		switch {
		case r.URL.Host == "scoped.atlassian.net" && r.URL.Path == "/_edge/tenant_info":
			return 200, `{"cloudId":"cloud-1"}`
		case r.URL.Host == "scoped.atlassian.net":
			return 401, ""
		case strings.HasPrefix(r.URL.Path, "/ex/jira/cloud-1/rest/api/3/myself"):
			return 200, `{"accountId":"acc-1"}`
		case strings.HasPrefix(r.URL.Path, "/ex/jira/cloud-1/rest/api/3/project/search"):
			return 200, `{}`
		}
		return 404, ""
	}

	lines, id := check(&ts.FakeHTTP{Handle: scoped}, a, secrets()).Jira(ctx)
	if id != "acc-1" || !*lines[1].OK || !strings.Contains(lines[0].Detail, "api.atlassian.com") {
		t.Fatalf("check: %q %+v", id, lines)
	}

	h := &ts.FakeHTTP{Handle: scoped}
	if got, err := client(t, h, a).AccountID(ctx); err != nil || got != "acc-1" {
		t.Fatalf("client: %q %v", got, err)
	}
	if n := len(h.Requests); n != 1 || h.Requests[0].URL.Host != "api.atlassian.com" {
		t.Errorf("the remembered gateway is used straight away: %d requests, first to %s", n, h.Requests[0].URL)
	}
}

// On an issue the site answers a scoped token with 404 ("does not exist or you do not have permission"), not 401.
func TestScopedTokenIssueNotFoundTriesTheGateway(t *testing.T) {
	a := account()
	a.BaseURL = "https://scoped404.atlassian.net"
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case r.URL.Path == "/_edge/tenant_info":
			return 200, `{"cloudId":"cloud-2"}`
		case r.URL.Host == "scoped404.atlassian.net":
			return 404, `{"errorMessages":["Issue does not exist or you do not have permission to see it."]}`
		case strings.HasSuffix(r.URL.Path, "/ex/jira/cloud-2/rest/api/3/issue/PROJ-1"):
			return 200, `{"id":"65648"}`
		}
		return 500, ""
	}}
	if id, err := client(t, h, a).IssueID(ctx, "PROJ-1"); err != nil || id != 65648 {
		t.Fatalf("got %d %v", id, err)
	}
}

// Without a check first (a fresh start): the site answers a scoped token's project list with an empty page, not
// 401, so the first call asks the site who you are to find the gateway.
func TestScopedTokenFindsTheGatewayWithoutACheck(t *testing.T) {
	a := account()
	a.BaseURL = "https://scopedfresh.atlassian.net"
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case r.URL.Path == "/_edge/tenant_info":
			return 200, `{"cloudId":"cloud-3"}`
		case r.URL.Host == "scopedfresh.atlassian.net" && r.URL.Path == "/rest/api/3/myself":
			return 401, ""
		case r.URL.Host == "scopedfresh.atlassian.net":
			return 200, `{"values":[],"isLast":true}`
		case strings.HasSuffix(r.URL.Path, "/ex/jira/cloud-3/rest/api/3/project/search"):
			return 200, `{"values":[{"key":"PROJ"}],"isLast":true}`
		}
		return 500, ""
	}}
	c := client(t, h, a)
	if keys, err := c.ProjectKeys(ctx); err != nil || len(keys) != 1 || keys[0] != "PROJ" {
		t.Fatalf("got %v %v", keys, err)
	}
	before := len(h.Requests)
	if _, err := c.ProjectKeys(ctx); err != nil || len(h.Requests) != before+1 {
		t.Errorf("the route is decided once: %d more requests, %v", len(h.Requests)-before, err)
	}
}

// A classic token: the site says who you are, and every call stays on the site.
func TestClassicTokenStaysOnTheSite(t *testing.T) {
	a := account()
	a.BaseURL = "https://classic.atlassian.net"
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case r.URL.Host != "classic.atlassian.net":
			return 500, ""
		case r.URL.Path == "/rest/api/3/myself":
			return 200, `{"accountId":"acc-1"}`
		}
		return 200, `{"values":[{"key":"PROJ"}],"isLast":true}`
	}}
	c := client(t, h, a)
	for range 2 {
		if keys, err := c.ProjectKeys(ctx); err != nil || len(keys) != 1 {
			t.Fatalf("got %v %v", keys, err)
		}
	}
	if n := len(h.Requests); n != 3 {
		t.Errorf("one who-am-I, then the calls: %d requests", n)
	}
}
