package tempo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Plan is one day of time planned for you on a ticket in Tempo Planner: what Jira or Tempo plans when a ticket is
// assigned to you or changes status. A plan over several days becomes one Plan per day.
type Plan struct {
	ID          int64
	IssueID     int64
	IssueKey    string // filled from the Jira issue id; "" when it can't be looked up
	Day         string
	Seconds     int
	Description string
}

// flexID reads an id Tempo sends as a number or as a string.
type flexID int64

func (f *flexID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil // not a numeric id: leave it zero
	}
	*f = flexID(n)
	return nil
}

func (f flexID) String() string { return strconv.FormatInt(int64(f), 10) }

// Plans is your Tempo plans on issues for the days (inclusive): GET /plans/user/{accountId}?from&to, following
// metadata.next. Plans on a whole project, and accounts that log in Jira, have none.
func (c *Client) Plans(ctx context.Context, accountID, fromDay, toDay string) ([]Plan, error) {
	if c.Provider != core.ProviderTempo {
		return nil, nil
	}
	q := url.Values{"from": {fromDay}, "to": {toDay}, "limit": {"1000"}}
	next := c.TempoBaseURL + "/plans/user/" + url.PathEscape(accountID) + "?" + q.Encode()
	if _, err := url.Parse(next); err != nil || !strings.HasPrefix(next, "http") {
		return nil, missing("tempoUrl")
	}
	out := []Plan{}
	for next != "" {
		data, err := c.send(ctx, "GET", next, "Bearer "+c.TempoToken, nil, "Reading your Tempo plans")
		if err != nil {
			return nil, err
		}
		page, err := parsePlans(data, fromDay, toDay)
		if err != nil {
			return nil, err
		}
		out = append(out, page.plans...)
		next = page.next
	}
	return out, nil
}

type planPage struct {
	plans []Plan
	next  string
}

func parsePlans(data []byte, fromDay, toDay string) (planPage, error) {
	var page struct {
		Metadata *struct {
			Next *string `json:"next"`
		} `json:"metadata"`
		Results []struct {
			ID       flexID `json:"id"`
			PlanItem struct {
				ID   flexID `json:"id"`
				Type string `json:"type"`
			} `json:"planItem"`
			Description *string `json:"description"`
			Dates       struct {
				Values []struct {
					Date              string `json:"date"`
					TimePlannedSecond int    `json:"timePlannedSeconds"`
				} `json:"values"`
			} `json:"dates"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return planPage{}, err
	}
	var out planPage
	for _, r := range page.Results {
		if !strings.EqualFold(r.PlanItem.Type, "ISSUE") || r.PlanItem.ID == 0 {
			continue
		}
		desc := ""
		if r.Description != nil {
			desc = strings.TrimSpace(*r.Description)
		}
		for _, d := range r.Dates.Values {
			if d.TimePlannedSecond <= 0 || d.Date < fromDay || d.Date > toDay {
				continue
			}
			out.plans = append(out.plans, Plan{
				ID: int64(r.ID), IssueID: int64(r.PlanItem.ID), Day: d.Date, Seconds: d.TimePlannedSecond, Description: desc,
			})
		}
	}
	if page.Metadata != nil && page.Metadata.Next != nil {
		out.next = *page.Metadata.Next
	}
	return out, nil
}

// ReadPlans is your plans for a range with their issues named (looked up once and kept, like Read).
func ReadPlans(ctx context.Context, cache IssueCache, c *Client, accountID, fromDay, toDay string) ([]Plan, error) {
	plans, err := c.Plans(ctx, accountID, fromDay, toDay)
	if err != nil {
		return nil, err
	}
	keys := map[int64]string{}
	for i := range plans {
		plans[i].IssueKey = issueKeyOf(ctx, cache, c, plans[i].IssueID, keys)
	}
	return plans, nil
}

// issueKeyOf names an issue id from the memo, the cache, or Jira ("" when it can't).
func issueKeyOf(ctx context.Context, cache IssueCache, c Service, id int64, memo map[int64]string) string {
	if k, ok := memo[id]; ok {
		return k
	}
	k, ok := cache.CachedIssueKey(id, c.Site())
	if !ok {
		var err error
		if k, err = c.IssueKey(ctx, id); err == nil {
			_ = cache.CacheIssueID(k, id, c.Site())
		} else {
			k = ""
		}
	}
	memo[id] = k
	return k
}
