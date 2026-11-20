package tempo

import (
	"net/http"
	"strings"
	"testing"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

const issueWithSchema = `{"key":"%s","names":{"customfield_10026":"Account"},` +
	`"schema":{"customfield_10026":{"custom":"io.tempo.jira__account"}},` +
	`"fields":{"project":{"id":"%s","key":"%s"},"customfield_10026":%s}}`

func issueJSON(key, projectID, projectKey, account string) string {
	out := issueWithSchema
	for _, v := range []string{key, projectID, projectKey, account} {
		out = strings.Replace(out, "%s", v, 1)
	}
	return out
}

// The worklog is booked (_Account_) on the issue's Account, else on its project's default account; a bare
// account id is turned into its key with Tempo's account list.
func TestCreateBooksTheIssueAccount(t *testing.T) {
	var posted []string
	listReads := 0
	h := &ts.FakeHTTP{Handle: func(r *http.Request, body string) (int, string) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/10001"):
			if r.URL.Query().Get("expand") != "names,schema" {
				t.Errorf("issue query %s", r.URL.RawQuery)
			}
			return 200, issueJSON("PROJ-1", "100", "PROJ", `{"id":12,"value":"Acme Support"}`)
		case strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/10002"):
			return 200, issueJSON("PROJ-2", "100", "PROJ", `null`)
		case strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/10004"):
			// The Forge Account field carries the key: no Tempo lookup (which needs accounts:view).
			return 200, `{"key":"PROJSTART-1","names":{"customfield_10055":"Account"},` +
				`"schema":{"customfield_10055":{"custom":"ari:cloud:ecosystem::extension/fa75/ea7f/static/account"}},` +
				`"fields":{"customfield_10055":{"id":227,"value":"PROJ GENERAL SUP 2026","optionProperties":{"id":227,"key":"PROJGENERALSUP2026"}}}}`
		case strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/10003"):
			return 200, issueJSON("INT-1", "200", "INT", `null`)
		case strings.HasSuffix(r.URL.Path, "/account-links/project/100"):
			return 200, `{"results":[{"default":false,"account":{"self":"https://api.tempo.io/4/accounts/OTHER"}},` +
				`{"default":true,"account":{"self":"https://api.tempo.io/4/accounts/PROJ-DEF"}}]}`
		case strings.HasSuffix(r.URL.Path, "/account-links/project/200"):
			return 200, `{"results":[]}`
		case strings.HasSuffix(r.URL.Path, "/accounts"):
			listReads++
			return 200, `{"metadata":{},"results":[{"id":12,"key":"ACME-SUP"},{"id":13,"key":"X"}]}`
		case r.Method == "POST":
			posted = append(posted, body)
			if !strings.Contains(body, "_Account_") {
				return 400, `{"errors":[{"message":"Work attribute _Account_ is required"}]}`
			}
			return 200, `{"tempoWorklogId":77}`
		}
		return 500, "unexpected " + r.URL.String()
	}}
	a := account()
	a.TempoBaseURL = "https://api.tempo.io/4"
	c := client(t, h, a)
	create := func(id int64) error {
		_, err := c.Create(ctx, Payload{
			IssueKey: "X", IssueID: p(id), AuthorAccountID: p("acc"),
			StartDate: "2026-03-03", StartTime: "09:00:00", TimeSpentSeconds: 60,
		})
		return err
	}
	for _, id := range []int64{10001, 10001, 10002} {
		if err := create(id); err != nil {
			t.Fatal(err)
		}
	}
	if want := `"attributes":[{"key":"_Account_","value":"ACME-SUP"}]`; !strings.Contains(posted[0], want) || !strings.Contains(posted[1], want) {
		t.Errorf("issueAccount: %v", posted)
	}
	if !strings.Contains(posted[2], `"value":"PROJ-DEF"`) {
		t.Errorf("projectDefault: %s", posted[2])
	}
	if err := create(10004); err != nil || !strings.Contains(posted[3], `"value":"PROJGENERALSUP2026"`) {
		t.Errorf("forgeField: %v %s", err, posted[3])
	}
	if listReads != 1 {
		t.Errorf("account list read %d times", listReads)
	}
	if err := create(10003); err == nil || !strings.Contains(err.Error(), "INT-1 has no Account set, and project INT has no default") {
		t.Errorf("noAccountIsExplained: %v", err)
	}
}

// A lookup that fails doesn't stop a site without required accounts; when Tempo wants one, the failure is named.
func TestAccountLookupFailureOnlyExplains(t *testing.T) {
	required := false
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		if r.Method == "POST" {
			if required {
				return 400, `{"errors":[{"message":"Work attribute _Account_ is required"}]}`
			}
			return 200, `{"tempoWorklogId":5}`
		}
		return 403, `{"errorMessages":["scope does not match"]}`
	}}
	c := client(t, h, account())
	pl := Payload{IssueKey: "PROJ-1", IssueID: p(int64(1)), AuthorAccountID: p("acc")}
	if id, err := c.Create(ctx, pl); err != nil || id != 5 {
		t.Errorf("notRequired: %v", err)
	}
	required = true
	if _, err := c.Create(ctx, pl); err == nil || !strings.Contains(err.Error(), "account couldn't be read") ||
		!strings.Contains(err.Error(), "scope does not match") {
		t.Errorf("required: %v", err)
	}
}
