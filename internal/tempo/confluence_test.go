package tempo

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

func TestConfluenceActivity(t *testing.T) {
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch p := r.URL.Path; {
		case strings.HasSuffix(p, "/user/current"):
			return 200, `{"accountId":"me"}`
		case strings.HasSuffix(p, "/rest/api/search") && r.URL.Query().Get("cursor") == "":
			if !strings.Contains(r.URL.Query().Get("cql"), `contributor = "me"`) {
				return 400, "bad cql"
			}
			return 200, `{"results":[{"content":{"id":"10","type":"page","title":"PM-1 Spec","space":{"name":"Product"}}}],
			  "_links":{"next":"/rest/api/search?cursor=c2"}}`
		case strings.HasSuffix(p, "/rest/api/search"):
			return 200, `{"results":[
			  {"content":{"id":"30","type":"comment","title":"Re: Roadmap","container":{"id":20,"type":"page","title":"Roadmap"},
			   "body":{"storage":{"value":"<p>Moved Q3 &amp; Q4</p>"}},
			   "version":{"when":"2026-03-03T13:00:00.000Z","by":{"accountId":"me"}}}},
			  {"content":{"id":"31","type":"comment","title":"Re: Roadmap","container":{"id":"20","type":"page","title":"Roadmap"},
			   "version":{"when":"2026-03-03T14:00:00.000Z","by":{"accountId":"other"}}}}],"_links":{}}`
		case strings.HasSuffix(p, "/api/v2/pages/10/versions"):
			return 200, `{"results":[
			  {"authorId":"me","createdAt":"2026-03-03T08:00:00.000Z"},
			  {"authorId":"other","createdAt":"2026-03-03T09:00:00.000Z"},
			  {"authorId":"me","createdAt":"2026-03-05T08:00:00.000Z"}],"_links":{}}`
		}
		return 404, r.URL.String()
	}}
	c := Confluence{Site: "https://x.atlassian.net", Email: "a@b.c", Token: "t", HTTP: h, Days: ts.Days}
	me, err := c.AccountID(t.Context())
	if err != nil || me != "me" {
		t.Fatal(me, err)
	}
	got, err := c.Activity(t.Context(), me, "2026-03-03", "2026-03-03")
	if err != nil {
		t.Fatal(err)
	}
	// Newest first: my comment on Roadmap, my version of PM-1 Spec; others' and out-of-range ones are left out.
	if len(got) != 2 || got[0].PageID != "20" || !got[0].Comment || got[0].Title != "Roadmap" || got[0].Text != "Moved Q3 & Q4" || got[0].Kind != "page" ||
		got[1].PageID != "10" || got[1].Space != "Product" || got[1].Comment || got[1].Kind != "page" {
		t.Errorf("%+v", got)
	}
	if u := h.Requests[len(h.Requests)-1].URL.Path; u != "/wiki/rest/api/search" {
		t.Errorf("the next link is read from /wiki: %s", u)
	}
}

func TestStorageLinesAndLineDiff(t *testing.T) {
	got := StorageLines(`<h1>Plan</h1><p>Ship &amp; <strong>test</strong></p><ul><li>one</li><li> </li></ul><p>a<br/>b</p>`)
	if strings.Join(got, "|") != "Plan|Ship & test|one|a|b" {
		t.Errorf("lines: %q", got)
	}
	added, removed := LineDiff([]string{"a", "b", "c", "d"}, []string{"a", "x", "c", "d", "e"})
	if strings.Join(added, "|") != "x|e" || strings.Join(removed, "|") != "b" {
		t.Errorf("diff: +%q -%q", added, removed)
	}
	big := make([]string, 2100)
	for i := range big {
		big[i] = strconv.Itoa(i)
	}
	added, removed = LineDiff(big, append(big[1:len(big):len(big)], "new"))
	if strings.Join(added, "|") != "new" || strings.Join(removed, "|") != "0" {
		t.Errorf("big diff: +%q -%q", added, removed)
	}
}

// My version 3 is compared with version 2 (someone else's), my version 1 with nothing; version 4 is out of range.
func TestConfluenceChanges(t *testing.T) {
	bodies := map[string]string{"1": "<p>Intro</p>", "2": "<p>Intro</p><p>Scope</p>", "3": "<p>Intro</p><p>Scope v2</p><p>Risks</p>"}
	h := &ts.FakeHTTP{Handle: func(r *http.Request, _ string) (int, string) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pages/10/versions"):
			return 200, `{"results":[
			  {"number":4,"authorId":"me","createdAt":"2026-03-05T08:00:00.000Z"},
			  {"number":3,"authorId":"me","createdAt":"2026-03-03T10:00:00.000Z","message":"Risks"},
			  {"number":2,"authorId":"other","createdAt":"2026-03-03T09:00:00.000Z"},
			  {"number":1,"authorId":"me","createdAt":"2026-03-03T08:00:00.000Z"}],"_links":{}}`
		case strings.HasSuffix(r.URL.Path, "/pages/10"):
			return 200, `{"body":{"storage":{"value":"` + bodies[r.URL.Query().Get("version")] + `"}}}`
		}
		return 404, r.URL.String()
	}}
	c := Confluence{Site: "https://x.atlassian.net", Email: "a@b.c", Token: "t", HTTP: h, Days: ts.Days}
	got, err := c.Changes(t.Context(), "me", "page", "10", "2026-03-03", "2026-03-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if first := got[0]; first.Number != 1 || first.AddedLines != 1 || first.Removed != 0 {
		t.Errorf("first: %+v", first)
	}
	if v3 := got[1]; v3.Number != 3 || v3.Message != "Risks" || v3.AddedLines != 2 || v3.Removed != 1 ||
		strings.Join(v3.AddedText, "|") != "Scope v2|Risks" || v3.RemovedText[0] != "Scope" {
		t.Errorf("v3: %+v", v3)
	}
	if _, err := c.Changes(t.Context(), "me", "../x", "10", "2026-03-03", "2026-03-03"); err == nil {
		t.Error("any kind accepted")
	}
}
