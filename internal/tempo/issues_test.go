package tempo

import "testing"

func TestSearchJQL(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"", `created >= "2026-03-01" AND created < "2026-03-15" ORDER BY created DESC`},
		{` Login "page" OR x=1 `, `created >= "2026-03-01" AND created < "2026-03-15" AND summary ~ "login*" AND summary ~ "page*" AND summary ~ "or*" AND summary ~ "x*" AND summary ~ "1*" ORDER BY created DESC`},
		{"bejelentkezés", `created >= "2026-03-01" AND created < "2026-03-15" AND summary ~ "bejelentkezés*" ORDER BY created DESC`},
	} {
		if got := SearchJQL("2026-03-01", "2026-03-15", c.text); got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.text, got, c.want)
		}
	}
}

func TestIssueKeyLike(t *testing.T) {
	for s, want := range map[string]bool{"PROJ-12": true, "AB2-1": true, "PROJ": false, "PROJ-": false, "-12": false, "proj-12": false, "1A-2": false} {
		if issueKeyLike(s) != want {
			t.Errorf("%q: want %v", s, want)
		}
	}
}

func TestExcerpt(t *testing.T) {
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{
		{"  short\n\ttext ", 20, "short text"},
		{"exactly ten", 11, "exactly ten"},
		{"one two three four five", 12, "one two…"},
		{"averyveryverylongword", 8, "averyve…"},
		{"ünïcödé wörds hère", 10, "ünïcödé…"},
	} {
		if got := Excerpt(c.in, c.n); got != c.want {
			t.Errorf("Excerpt(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
