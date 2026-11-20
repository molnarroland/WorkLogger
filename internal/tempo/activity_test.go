package tempo

import (
	"testing"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// One issue you created and moved, someone else's change, a quiet Rank change, your comment, and a change
// outside the range.
const activityPage = `{"issues":[{"key":"PROJ-7","fields":{"summary":"Fix rounding",
  "created":"2026-03-03T09:00:00.000+0100","creator":{"accountId":"me"},
  "comment":{"comments":[
    {"author":{"accountId":"me"},"created":"2026-03-03T11:00:00.000+0100",
     "body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Done, see PR"}]}]}},
    {"author":{"accountId":"other"},"created":"2026-03-03T12:00:00.000+0100","body":null}]}},
  "changelog":{"histories":[
    {"author":{"accountId":"me"},"created":"2026-03-03T10:00:00.000+0100","items":[
      {"field":"status","fromString":"To Do","toString":"In Progress"},{"field":"Rank","fromString":null,"toString":"Ranked higher"}]},
    {"author":{"accountId":"other"},"created":"2026-03-03T10:30:00.000+0100","items":[{"field":"assignee","toString":"Me"}]},
    {"author":{"accountId":"me"},"created":"2026-03-05T10:00:00.000+0100","items":[{"field":"status","toString":"Done"}]}]}}],
 "nextPageToken":"p2"}`

func TestParseActivity(t *testing.T) {
	start, _ := ts.Days.Bounds("2026-03-03")
	_, end := ts.Days.Bounds("2026-03-04")
	got, next, err := parseActivity([]byte(activityPage), "me", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if next != "p2" || len(got) != 3 {
		t.Fatalf("next %q, %d entries: %+v", next, len(got), got)
	}
	if a := got[0]; a.Kind != "created" || a.IssueKey != "PROJ-7" || a.Summary != "Fix rounding" {
		t.Errorf("created: %+v", a)
	}
	if a := got[1]; a.Kind != "change" || a.Field != "status" || a.From != "To Do" || a.To != "In Progress" {
		t.Errorf("change: %+v", a)
	}
	if a := got[2]; a.Kind != "comment" || a.Text != "Done, see PR" || !a.At.Equal(ts.At("2026-03-03", "11:00")) {
		t.Errorf("comment: %+v", a)
	}
}
