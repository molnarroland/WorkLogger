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
