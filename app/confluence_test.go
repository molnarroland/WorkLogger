package app

import (
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/tempo"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// One day: "PROJ-1 Spec" edited twice in the morning, "Roadmap" commented after lunch. The Spec page-day became a
// draft; the untagged commit around 10:00 is offered PROJ-1, and nothing is offered to the page without a ticket.
func TestBuildConfluenceFeed(t *testing.T) {
	edit := func(hm, id, title string, comment bool) profileEdit {
		return profileEdit{tempo.ConfluenceEdit{At: ts.At("2026-03-03", hm), PageID: id, Title: title, Space: "Product", Kind: "page", Comment: comment, Text: "Looks good"}, "c1", ""}
	}
	edits := []profileEdit{edit("14:00", "8", "Roadmap", true), edit("09:30", "7", "PROJ-1 Spec", false), edit("10:30", "7", "PROJ-1 Spec", false)}
	log := func(id int64, hm string, ref string) core.Worklog {
		return core.Worklog{
			ID: id, Day: "2026-03-03", Start: ts.At("2026-03-03", hm), Duration: 1800,
			Source: core.SourceGit, Status: core.StatusDraft, SourceRef: ref,
		}
	}
	logs := []core.Worklog{log(1, "10:00", "abc"), log(2, "14:00", "def"), log(3, "09:00", "confluence:7:2026-03-03")}
	got := buildConfluenceFeed(ConfluenceActivityDTO{Days: []ConfluenceDayDTO{}}, edits, logs, map[string]bool{"PROJ": true}, ts.Days)
	if got.Entries != 3 || got.Pages != 2 || len(got.Days) != 1 || len(got.Days[0].Pages) != 2 {
		t.Fatalf("feed: %+v", got)
	}
	road, spec := got.Days[0].Pages[0], got.Days[0].Pages[1] // the latest activity first
	if spec.IssueKey != "PROJ-1" || spec.First != "09:30" || spec.Last != "10:30" || len(spec.Edits) != 2 ||
		spec.Entry == nil || spec.Entry.ID != 3 || len(spec.Suggested) != 2 || spec.Suggested[0].ID != 1 {
		t.Errorf("spec: %+v", spec)
	}
	if road.IssueKey != "" || len(road.Comments) != 1 || road.Comments[0] != (JiraNoteDTO{"14:00", "Looks good"}) || road.Entry != nil || len(road.Suggested) != 0 {
		t.Errorf("roadmap: %+v", road)
	}
}

// Only the profile's own site opens, and only for a page id.
func TestOpenConfluencePage(t *testing.T) {
	s, _ := newService(t)
	var opened []string
	s.Shell.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	must(t, s.update(func(st *core.Settings) {
		st.Profiles = []core.Profile{{ID: "c1", Kind: core.ProfileConfluence, URL: "https://acme.atlassian.net/"}}
	}))
	must(t, s.OpenConfluencePage("c1", "123"))
	for _, c := range [][2]string{{"c1", "12/../evil"}, {"x", "123"}} {
		if s.OpenConfluencePage(c[0], c[1]) == nil {
			t.Errorf("%v: opened", c)
		}
	}
	if len(opened) != 1 || opened[0] != "https://acme.atlassian.net/wiki/pages/viewpage.action?pageId=123" {
		t.Errorf("opened %v", opened)
	}
}
