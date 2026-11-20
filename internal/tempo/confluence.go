package tempo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// ConfluenceScopes is what a Confluence API token with scopes needs: finding the pages you worked on (CQL search),
// your account, and the pages' and blog posts' versions. A classic token works with your own permissions.
const ConfluenceScopes = "search:confluence, read:confluence-user, read:page:confluence"

// Confluence reads one Confluence Cloud site (https://yourcompany.atlassian.net, its /wiki).
type Confluence struct {
	Site, Email, Token string
	HTTP               core.HTTPDoer
	Days               core.Days
}

// ConfluenceEdit is one moment you worked on a page or blog post: saved a version of it, or commented on it.
type ConfluenceEdit struct {
	At                   time.Time
	PageID, Title, Space string
	Kind                 string // "page" or "blogpost"
	Comment              bool
	Text                 string // a comment's text
}

func (c Confluence) get(ctx context.Context, path, what string, out any) error {
	cl := &Client{HTTP: c.HTTP}
	data, err := siteSend(ctx, c.HTTP, cl.send, c.Site, "confluence", "GET", path, basicAuth(c.Email, c.Token), nil, what)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// AccountID is your Atlassian account id, as Confluence knows it.
func (c Confluence) AccountID(ctx context.Context) (string, error) {
	var me struct {
		AccountID string `json:"accountId"`
	}
	err := c.get(ctx, "/wiki/rest/api/user/current", "Reading your Confluence account", &me)
	if err == nil && me.AccountID == "" {
		err = fmt.Errorf("Confluence didn't say who you are")
	}
	return me.AccountID, err
}

// wikiPath makes a _links.next link a path from the site: v1 gives it from /wiki, v2 from the site.
func wikiPath(next string) string {
	if strings.HasPrefix(next, "/wiki/") {
		return next
	}
	return "/wiki" + next
}

// Activity is what you did in Confluence over the days (inclusive): the pages, blog posts and comments CQL finds
// you contributed to and that changed since the first day, then each page's versions you saved in the range, and
// each of your comments by when it was last saved.
// Known limit: a comment counts at its latest version only (an older edit of it is not seen).
func (c Confluence) Activity(ctx context.Context, me, fromDay, toDay string) ([]ConfluenceEdit, error) {
	start, _ := c.Days.Bounds(fromDay)
	_, end := c.Days.Bounds(toDay)
	in := func(t time.Time) bool { return !t.Before(start) && t.Before(end) }
	// CQL compares in the user's own zone; a day early keeps the range's first hours in, the versions trim it.
	cql := fmt.Sprintf(`contributor = "%s" AND lastmodified >= "%s" AND type in (page, blogpost, comment)`,
		strings.ReplaceAll(me, `"`, ""), c.Days.Shift(fromDay, -1))
	next := "/wiki/rest/api/search?" + url.Values{
		"cql": {cql}, "limit": {"50"}, "expand": {"content.version,content.container,content.space,content.body.storage"},
	}.Encode()
	type ref struct {
		ID    flexID `json:"id"` // a string, but a comment's container has it as a number
		Type  string `json:"type"`
		Title string `json:"title"`
	}
	var out []ConfluenceEdit
	for next != "" {
		var page struct {
			Results []struct {
				Content *struct {
					ref
					Space   *struct{ Name string } `json:"space"`
					Version *struct {
						When string `json:"when"`
						By   *struct {
							AccountID string `json:"accountId"`
						} `json:"by"`
					} `json:"version"`
					Container *ref `json:"container"`
					Body      *struct {
						Storage struct{ Value string } `json:"storage"`
					} `json:"body"`
				} `json:"content"`
			} `json:"results"`
			Links struct {
				Next string `json:"next"`
			} `json:"_links"`
		}
		if err := c.get(ctx, next, "Searching your Confluence pages", &page); err != nil {
			return nil, err
		}
		for _, r := range page.Results {
			ct := r.Content
			if ct == nil {
				continue
			}
			space := ""
			if ct.Space != nil {
				space = ct.Space.Name
			}
			switch ct.Type {
			case "page", "blogpost":
				list, err := c.versions(ctx, ct.Type, ct.ID.String(), me)
				if err != nil {
					return nil, err
				}
				for _, v := range list {
					if in(v.At) {
						out = append(out, ConfluenceEdit{At: v.At, PageID: ct.ID.String(), Title: ct.Title, Space: space, Kind: ct.Type})
					}
				}
			case "comment":
				if ct.Version == nil || ct.Version.By == nil || ct.Version.By.AccountID != me || ct.Container == nil || ct.Container.ID == 0 {
					continue
				}
				if t, err := time.Parse(time.RFC3339Nano, ct.Version.When); err == nil && in(t) {
					e := ConfluenceEdit{
						At: t, PageID: ct.Container.ID.String(), Title: ct.Container.Title, Space: space,
						Kind: ct.Container.Type, Comment: true,
					}
					if ct.Body != nil {
						e.Text = strings.Join(StorageLines(ct.Body.Storage.Value), "\n")
					}
					out = append(out, e)
				}
			}
		}
		next = ""
		if page.Links.Next != "" {
			next = wikiPath(page.Links.Next)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

// version is one saved version of a page or blog post.
type version struct {
	Number  int
	At      time.Time
	Message string
}

// versions is the versions you saved of a page or blog post (Confluence API v2, all pages of them).
func (c Confluence) versions(ctx context.Context, kind, id, me string) ([]version, error) {
	next := fmt.Sprintf("/wiki/api/v2/%ss/%s/versions?limit=250", kind, url.PathEscape(id))
	var out []version
	for next != "" {
		var page struct {
			Results []struct {
				Number    int    `json:"number"`
				AuthorID  string `json:"authorId"`
				CreatedAt string `json:"createdAt"`
				Message   string `json:"message"`
			} `json:"results"`
			Links struct {
				Next string `json:"next"`
			} `json:"_links"`
		}
		if err := c.get(ctx, next, "Reading the versions of Confluence page "+id, &page); err != nil {
			return nil, err
		}
		for _, v := range page.Results {
			if t, err := time.Parse(time.RFC3339Nano, v.CreatedAt); err == nil && v.AuthorID == me {
				out = append(out, version{v.Number, t, v.Message})
			}
		}
		next = ""
		if page.Links.Next != "" {
			next = wikiPath(page.Links.Next)
		}
	}
	return out, nil
}

// ConfluenceChange is one version you saved: how many lines of text it added and removed against the version
// before it, and the lines themselves (Added and Removed: at most changeSample of each).
type ConfluenceChange struct {
	At                     time.Time
	Number                 int
	Message                string
	AddedLines, Removed    int
	AddedText, RemovedText []string
}

const changeSample = 5

// Changes is what you changed on a page or blog post over the days (inclusive): each version you saved there,
// compared with the version before it (the first version adds the whole page). Two reads per version, so it is
// asked for one page at a time.
func (c Confluence) Changes(ctx context.Context, me, kind, id, fromDay, toDay string) ([]ConfluenceChange, error) {
	if kind != "page" && kind != "blogpost" {
		return nil, fmt.Errorf("not a page or blog post: %q", kind)
	}
	start, _ := c.Days.Bounds(fromDay)
	_, end := c.Days.Bounds(toDay)
	list, err := c.versions(ctx, kind, id, me)
	if err != nil {
		return nil, err
	}
	bodies := map[int][]string{0: nil}
	body := func(n int) ([]string, error) {
		if b, ok := bodies[n]; ok {
			return b, nil
		}
		var v struct {
			Body struct {
				Storage struct{ Value string } `json:"storage"`
			} `json:"body"`
		}
		path := fmt.Sprintf("/wiki/api/v2/%ss/%s?version=%d&body-format=storage", kind, url.PathEscape(id), n)
		if err := c.get(ctx, path, fmt.Sprintf("Reading version %d of Confluence page %s", n, id), &v); err != nil {
			return nil, err
		}
		bodies[n] = StorageLines(v.Body.Storage.Value)
		return bodies[n], nil
	}
	var out []ConfluenceChange
	for _, v := range list {
		if v.At.Before(start) || !v.At.Before(end) {
			continue
		}
		before, err := body(v.Number - 1)
		if err != nil {
			return nil, err
		}
		after, err := body(v.Number)
		if err != nil {
			return nil, err
		}
		added, removed := LineDiff(before, after)
		out = append(out, ConfluenceChange{
			At: v.At, Number: v.Number, Message: v.Message, AddedLines: len(added), Removed: len(removed),
			AddedText: added[:min(len(added), changeSample)], RemovedText: removed[:min(len(removed), changeSample)],
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

var (
	// Block ends become line breaks; every other tag goes.
	blockEnd = regexp.MustCompile(`(?i)<br\s*/?>|</(p|li|h[1-6]|tr|td|th|div|pre|blockquote)>`)
	anyTag   = regexp.MustCompile(`<[^>]*>`)
)

// StorageLines is the text of Confluence's storage format (XHTML), one non-empty line per paragraph, list item,
// heading or table cell. Macros' parameters are tags too, so only their visible text remains.
func StorageLines(storage string) []string {
	text := html.UnescapeString(anyTag.ReplaceAllString(blockEnd.ReplaceAllString(storage, "\n"), ""))
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// LineDiff is the lines b adds to a and the lines it removes from it, in order (a longest common subsequence).
// ponytail: O(len(a)·len(b)); past maxDiffCells it only compares which lines exist, not their order.
func LineDiff(a, b []string) (added, removed []string) {
	added, removed = []string{}, []string{}
	if len(a)*len(b) > maxDiffCells {
		in := func(list []string) map[string]int {
			m := map[string]int{}
			for _, l := range list {
				m[l]++
			}
			return m
		}
		ina, inb := in(a), in(b)
		for _, l := range b {
			if ina[l] > 0 {
				ina[l]--
			} else {
				added = append(added, l)
			}
		}
		for _, l := range a {
			if inb[l] > 0 {
				inb[l]--
			} else {
				removed = append(removed, l)
			}
		}
		return added, removed
	}
	// lcs[i][j]: the longest common subsequence of a[i:] and b[j:].
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			removed = append(removed, a[i])
			i++
		default:
			added = append(added, b[j])
			j++
		}
	}
	return append(added, b[j:]...), append(removed, a[i:]...)
}

const maxDiffCells = 4_000_000

// Check tests the site, email and token (your account, read:confluence-user) and that pages can be searched
// (search:confluence) with harmless reads.
func (c Confluence) Check(ctx context.Context) []Line {
	signIn, search := "Site, email and API token", "Search pages (search:confluence)"
	if u, err := url.Parse(c.Site); err != nil || u.Host == "" {
		return []Line{{signIn, "Link a Jira profile, or enter your site, e.g. https://yourcompany.atlassian.net.", ok(false)}, {search, "", nil}}
	}
	if c.Email == "" || c.Token == "" {
		return []Line{{signIn, "Enter your email and an API token, or link a Jira profile that has them.", ok(false)}, {search, "", nil}}
	}
	explain := func(err error, scope string) string {
		var he HTTPError
		if !errors.As(err, &he) {
			return "Can't reach Confluence: " + err.Error()
		}
		switch he.Code {
		case 401:
			return "Confluence rejected the email or token. A Jira token with scopes doesn't work for Confluence: create one for Confluence, or a classic token."
		case 403:
			return "The token is missing " + scope + "."
		case 404:
			return "No Confluence at this address."
		}
		return httpDetail("Confluence", he.Code)
	}
	me, err := c.AccountID(ctx)
	if err != nil {
		return []Line{{signIn, explain(err, "read:confluence-user"), ok(false)}, {search, "", nil}}
	}
	lines := []Line{{signIn, "Signed in (" + me + ").", ok(true)}}
	var page struct{}
	if err := c.get(ctx, "/wiki/rest/api/search?limit=1&cql="+url.QueryEscape("type = page"), "Searching Confluence", &page); err != nil {
		return append(lines, Line{search, explain(err, "search:confluence"), ok(false)})
	}
	return append(lines, Line{search, "Pages can be searched. Reading their versions also needs read:page:confluence.", ok(true)})
}
