package tempo

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/store"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

var ctx = context.Background()

func p[T any](v T) *T { return &v }

func account() core.JiraAccount {
	a := core.NewJiraAccount("A1")
	a.BaseURL, a.Email = "https://acme.atlassian.net", "me@example.com"
	return a
}

func secrets() *ts.InMemorySecrets {
	a := account()
	return ts.NewSecrets(map[string]string{a.JiraTokenKey(): "jira-token", a.TempoTokenKey(): "tempo-token"})
}

func exportLogs() []core.Worklog {
	quoted := ts.Worklog("2026-03-03", "09:00", 5400, core.SourceGit, "", false)
	quoted.Description = "Fix \"quoted\"\nthing"
	noIssue := ts.Worklog("2026-03-03", "11:00", 1800, core.SourceGit, "", false)
	noIssue.IssueKey = nil
	zero := ts.Worklog("2026-03-03", "12:00", 0, core.SourceGit, "", false)
	zero.IssueKey = p("PROJ-2")
	return []core.Worklog{quoted, noIssue, zero}
}

func TestExport(t *testing.T) {
	logs := exportLogs()
	if e := Exportable(logs); len(e) != 1 || *e[0].IssueKey != "PROJ-1" {
		t.Error("onlyEntriesWithIssueAndTimeAreExported")
	}
	// 09:00 in Budapest is 08:00 UTC.
	if got := CSV(logs); got != "Issue Key,Date Started,Time Spent,Comment\nPROJ-1,2026-03-03 08:00:00Z,1h 30m,\"Fix \"\"quoted\"\" thing\"\n" {
		t.Errorf("csvUsesUTCAndQuotes: %q", got)
	}
	pl := Payloads(logs, "acc-1", ts.Days)[0]
	if pl.IssueKey != "PROJ-1" || pl.IssueID != nil || *pl.AuthorAccountID != "acc-1" || pl.StartDate != "2026-03-03" ||
		pl.StartTime != "09:00:00" || pl.TimeSpentSeconds != 5400 {
		t.Errorf("payloadsUseLocalDateAndTime: %+v", pl)
	}
	if Payloads(logs, "", ts.Days)[0].AuthorAccountID != nil {
		t.Error("missingAccountIdIsLeftOut")
	}
	data, _ := JSON(logs, "acc-1", ts.Days)
	var decoded []Payload
	if json.Unmarshal(data, &decoded) != nil || len(decoded) != 1 || decoded[0].StartTime != "09:00:00" {
		t.Errorf("jsonDecodesToThePayloads: %s", data)
	}
	w := ts.Worklog("2026-03-03", "09:00", 5400, core.SourceGit, "", false)
	w.Description = "Fix | pipe"
	noIssue := w
	noIssue.IssueKey = nil
	md := Markdown([]core.Worklog{w, noIssue}, "2026-03-01", "2026-03-31", "https://acme.atlassian.net/", ts.Days)
	if !strings.Contains(md, "https://acme.atlassian.net/plugins/servlet/ac/io.tempo.jira/tempo-app") ||
		!strings.Contains(md, `| 1 | PROJ-1 | 09:00 | 1h 30m | Fix \| pipe |`) || !strings.Contains(md, "1 worklogs") {
		t.Errorf("markdownExportListsEntriesPerDay:\n%s", md)
	}
}

func client(t *testing.T, h *ts.FakeHTTP, a core.JiraAccount) *Client {
	t.Helper()
	c, err := NewClient(a, secrets(), h, ts.Days)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func openStore(t *testing.T) *store.Store {
	s, err := store.Open(":memory:", ts.Days)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestReading(t *testing.T) {
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		u := r.URL.String()
		switch {
		case strings.Contains(u, "/rest/api/3/issue/10001"):
			return 200, `{"id":"10001","key":"PROJ-1"}`
		case strings.Contains(u, "offset=1"):
			return 200, `{"metadata":{},"results":[{"tempoWorklogId":2,"issue":{"id":10001},"timeSpentSeconds":1800,"startDate":"2026-03-04","startTime":"10:00:00"}]}`
		}
		return 200, `{"metadata":{"next":"https://api.tempo.io/4/worklogs/user/acc?offset=1"},"results":[{"tempoWorklogId":1,"issue":{"id":10001},"timeSpentSeconds":3600,"startDate":"2026-03-03","startTime":"09:00:00","description":"Fix"}]}`
	}}
	db := openStore(t)
	logs, err := Read(ctx, db, client(t, h, account()), "acc", "2026-03-01", "2026-03-31")
	if err != nil || len(logs) != 2 || logs[0].ID != 1 || logs[1].ID != 2 || logs[0].IssueKey != "PROJ-1" || logs[1].IssueKey != "PROJ-1" {
		t.Fatalf("worklogsFollowThePages: %v %+v", err, logs)
	}
	if u := h.Requests[0].URL.String(); u != "https://api.tempo.io/4/worklogs/user/acc?from=2026-03-01&limit=1000&to=2026-03-31" {
		t.Errorf("first url %s", u)
	}
	if h.Requests[0].Header.Get("Authorization") != "Bearer tempo-token" {
		t.Error("tempo auth")
	}
	lookups := 0
	for _, r := range h.Requests {
		if strings.Contains(r.URL.Path, "/issue/") {
			lookups++
		}
	}
	if k, _ := db.CachedIssueKey(10001, "https://acme.atlassian.net"); lookups != 1 || k != "PROJ-1" {
		t.Errorf("keysAreLookedUpOnce: %d %q", lookups, k)
	}

	local := []core.Worklog{
		ts.Worklog("2026-03-03", "09:00", 3600, core.SourceGit, "", false),
		ts.Worklog("2026-03-03", "11:00", 1800, core.SourceGit, "", false),
		ts.Worklog("2026-03-04", "09:00", 3600, core.SourceGit, "", false),
	}
	local[1].IssueKey, local[2].IssueKey = p("INT-12"), nil
	covered := Coverage(local, []Worklog{
		{ID: 1, IssueID: 1, IssueKey: "PROJ-1", Day: "2026-03-03", Seconds: 3540},
		{ID: 2, IssueID: 2, IssueKey: "INT-12", Day: "2026-03-03", Seconds: 900},
	})
	if !reflect.DeepEqual(covered, map[string]bool{"PROJ-1|2026-03-03": true}) {
		t.Errorf("coverageNeedsAtLeastTheLocalTimePerTicketDay: %v", covered)
	}
}

func oks(lines []Line) (out []any) {
	for _, l := range lines {
		if l.OK == nil {
			out = append(out, nil)
		} else {
			out = append(out, *l.OK)
		}
	}
	return
}

func check(h *ts.FakeHTTP, a core.JiraAccount, s *ts.InMemorySecrets) Check {
	return NewCheck(a, s, h, "2026-03-04")
}

func TestConnectionCheck(t *testing.T) {
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		u := r.URL.String()
		switch {
		case strings.HasSuffix(u, "/myself"):
			return 200, `{"accountId":"acc-1","displayName":"Jane"}`
		case strings.Contains(u, "project/search"):
			return 200, "{}"
		case r.Method == "POST":
			return 400, `{"errors":[]}`
		}
		return 200, `{"results":[]}`
	}}
	c := check(h, account(), secrets())
	lines, id := c.Jira(ctx)
	if id != "acc-1" || !reflect.DeepEqual(oks(lines), []any{true, true}) || !strings.Contains(lines[0].Detail, "Jane") {
		t.Errorf("everythingWorks jira: %+v", lines)
	}
	if got := oks(c.Tempo(ctx, id)); !reflect.DeepEqual(got, []any{true, true}) {
		t.Errorf("everythingWorks tempo: %v", got)
	}
	for _, r := range h.Requests {
		if strings.Contains(r.URL.Path, "/worklogs/user/acc-1") &&
			(!strings.Contains(r.URL.RawQuery, "from=2026-03-04&to=2026-03-04") || r.Header.Get("Authorization") != "Bearer tempo-token") {
			t.Errorf("view request %s", r.URL)
		}
	}

	lines, id = check(&ts.FakeHTTP{Handle: func(*http.Request, string) (int, string) { return 401, "" }}, account(), secrets()).Jira(ctx)
	if id != "" || !reflect.DeepEqual(oks(lines), []any{false, nil}) || !strings.Contains(lines[0].Detail, "rejected") {
		t.Errorf("rejectedJiraTokenStopsBeforeIssues: %+v", lines)
	}
	lines, _ = check(&ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			return 200, `{"accountId":"a"}`
		}
		return 403, ""
	}}, account(), secrets()).Jira(ctx)
	if !reflect.DeepEqual(oks(lines), []any{true, false}) || !strings.Contains(lines[1].Detail, "read:jira-work") {
		t.Errorf("missingReadWorkScopeIsNamed: %+v", lines)
	}
	wrong := check(&ts.FakeHTTP{Handle: func(*http.Request, string) (int, string) { return 404, "" }}, account(), secrets()).Tempo(ctx, "a")
	if !reflect.DeepEqual(oks(wrong), []any{false, nil}) || !strings.Contains(wrong[0].Detail, "api.tempo.io") {
		t.Errorf("tempoAddress: %+v", wrong)
	}
	viewOnly := check(&ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		if r.Method == "POST" {
			return 403, ""
		}
		return 200, "{}"
	}}, account(), secrets()).Tempo(ctx, "a")
	if !reflect.DeepEqual(oks(viewOnly), []any{true, false}) || !strings.Contains(viewOnly[1].Detail, "Manage worklogs") {
		t.Errorf("tempoPermissions: %+v", viewOnly)
	}
	empty := account()
	empty.BaseURL = ""
	none := &ts.FakeHTTP{}
	lines, _ = check(none, empty, ts.NewSecrets(nil)).Jira(ctx)
	tl := check(none, account(), ts.NewSecrets(nil)).Tempo(ctx, "a")
	if !reflect.DeepEqual(oks(lines), []any{false, nil}) || *tl[0].OK || len(none.Requests) != 0 {
		t.Error("missingFieldsAreNamedWithoutCalls")
	}
}

func TestClient(t *testing.T) {
	if _, err := NewClient(account(), ts.NewSecrets(nil), &ts.FakeHTTP{}, ts.Days); err == nil {
		t.Error("missingConfigurationIsReported")
	}

	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case strings.Contains(r.URL.RawQuery, "startAt=0"):
			return 200, `{"values":[{"key":"PROJ"},{"key":"INT"}],"isLast":false}`
		case strings.Contains(r.URL.RawQuery, "startAt=2"):
			return 200, `{"values":[{"key":"HR"}],"isLast":true}`
		case strings.HasSuffix(r.URL.Path, "/myself"):
			return 200, `{"accountId":"acc-1"}`
		case strings.Contains(r.URL.Path, "/issue/PROJ-1"):
			return 200, `{"id":"10001"}`
		case strings.Contains(r.URL.Path, "/issue/NOPE-1"):
			return 404, `{"errorMessages":["Issue does not exist"]}`
		case r.Method == "POST":
			return 200, `{"tempoWorklogId":77}`
		}
		return 500, "unexpected"
	}}
	c := client(t, h, account())
	if keys, _ := c.ProjectKeys(ctx); !reflect.DeepEqual(keys, []string{"PROJ", "INT", "HR"}) {
		t.Errorf("projectKeysFollowThePages: %v", keys)
	}
	if id, _ := c.AccountID(ctx); id != "acc-1" || !strings.HasPrefix(h.Requests[len(h.Requests)-1].Header.Get("Authorization"), "Basic ") {
		t.Error("accountIdUsesJiraBasicAuth")
	}
	if _, err := c.IssueKey(ctx, 1); err == nil || !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "unexpected") {
		t.Errorf("httpErrorsCarryStatusAndBody: %v", err)
	}

	db := openStore(t)
	ok := ts.Worklog("2026-03-03", "09:00", 3600, core.SourceGit, "", false)
	ok.ID = 1
	bad := ok
	bad.ID, bad.IssueKey = 2, p("NOPE-1")
	var exported []int64
	n, failures := Push(ctx, db, c, []core.Worklog{ok, bad}, "acc-1", ts.Days, func(w core.Worklog, id int64) error {
		if id != 77 {
			t.Errorf("tempo id %d", id)
		}
		exported = append(exported, w.ID)
		return nil
	}, nil)
	if n != 1 || len(failures) != 1 || !strings.Contains(failures[0], "NOPE-1 2026-03-03") || !reflect.DeepEqual(exported, []int64{1}) {
		t.Errorf("successfulEntriesAreMarkedExportedAndFailuresAreReported: %d %v", n, failures)
	}
	if id, _ := db.CachedIssueID("PROJ-1", c.Site()); id != 10001 {
		t.Error("cachesNewIssueIds")
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(h.Bodies[len(h.Bodies)-2]), &body)
	for _, b := range h.Bodies {
		if strings.Contains(b, "issueId") {
			_ = json.Unmarshal([]byte(b), &body)
		}
	}
	if body["issueId"] != float64(10001) || body["authorAccountId"] != "acc-1" || body["startTime"] != "09:00:00" {
		t.Errorf("createPostsTheWorklogWithTheTempoToken: %v", body)
	}

	ja := account()
	ja.Provider = core.ProviderJira
	jh := &ts.FakeHTTP{Handle: func(*http.Request, string) (int, string) { return 201, `{"id":"5005"}` }}
	id, err := client(t, jh, ja).Create(ctx, Payload{IssueKey: "PROJ-1", StartDate: "2026-03-03", StartTime: "09:00:00", TimeSpentSeconds: 1800, Description: "Fix"})
	if err != nil || id != 5005 || jh.Requests[0].URL.Path != "/rest/api/3/issue/PROJ-1/worklog" ||
		!strings.Contains(jh.Bodies[0], `"started":"2026-03-03T09:00:00.000+0100"`) || !strings.Contains(jh.Bodies[0], `"text":"Fix"`) {
		t.Errorf("jiraProviderCreatesTheWorklogOnTheIssue: %v %d %s", err, id, jh.Bodies)
	}
}

func TestParsePlans(t *testing.T) {
	data := []byte(`{"metadata":{"next":"https://api.tempo.io/4/plans?offset=50"},"results":[` +
		`{"id":"3","planItem":{"id":10001,"type":"ISSUE"},"description":" Assigned ",` +
		`"dates":{"values":[{"date":"2026-02-28","timePlannedSeconds":3600},{"date":"2026-03-02","timePlannedSeconds":1800},{"date":"2026-03-03","timePlannedSeconds":0}]}},` +
		`{"id":4,"planItem":{"id":"20","type":"PROJECT"},"dates":{"values":[{"date":"2026-03-02","timePlannedSeconds":3600}]}}]}`)
	page, err := parsePlans(data, "2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.plans) != 1 || page.next == "" {
		t.Fatalf("%+v", page)
	}
	p := page.plans[0]
	if p.ID != 3 || p.IssueID != 10001 || p.Day != "2026-03-02" || p.Seconds != 1800 || p.Description != "Assigned" {
		t.Errorf("%+v", p)
	}
}
