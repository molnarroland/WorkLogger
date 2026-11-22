package app

import (
	"fmt"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/view"
)

// ── Tempo plans ─────────────────────────────────────────────────────────────

// planned is what the last RefreshTempo read for the days, as the views take it: Tempo Planner's plans, and the
// worklogs Tempo has that WorkLogger didn't send (logged by Jira or Tempo themselves, or by hand). Never goes to
// the network.
func (s *Service) planned(from, to string) []view.Planned {
	sent := map[int64]bool{}
	if local, err := s.Store.Worklogs(from, to); err == nil {
		for _, w := range local {
			if w.TempoID != nil {
				sent[*w.TempoID] = true
			}
		}
	}
	s.tempoMu.Lock()
	defer s.tempoMu.Unlock()
	var out []view.Planned
	for _, day := range s.Days.Range(from, to) {
		for _, p := range s.planCache[day] {
			out = append(out, view.Planned{Kind: view.PlanKindPlan, Key: p.IssueKey, Day: p.Day, Seconds: p.Seconds, Description: p.Description})
		}
		for _, w := range s.tempoCache[day] {
			if !sent[w.ID] {
				out = append(out, view.Planned{Kind: view.PlanKindTempo, Key: w.IssueKey, Day: w.Day, Seconds: w.Seconds, Description: w.Description})
			}
		}
	}
	return out
}

// readBack: RefreshTempo has read the day (it may have found nothing).
func (s *Service) readBack(day string) bool {
	s.tempoMu.Lock()
	defer s.tempoMu.Unlock()
	_, ok := s.tempoCache[day]
	return ok
}

// PlanEntryDTO is one of WorkLogger's entries next to a plan.
type PlanEntryDTO struct {
	ID          int64       `json:"id"`
	Source      string      `json:"source"`
	Description string      `json:"description"`
	Seconds     int         `json:"seconds"`
	Status      string      `json:"status"`
	Focus       *view.Focus `json:"focus"`
}

// PlanItemDTO is a ticket-day Tempo has: planned ("plan"), or a worklog WorkLogger didn't send ("tempo").
type PlanItemDTO struct {
	Kind        string         `json:"kind"`
	Key         string         `json:"key"`
	Day         string         `json:"day"`
	Seconds     int            `json:"seconds"`
	Description string         `json:"description"`
	Logged      int            `json:"logged"`
	Entries     []PlanEntryDTO `json:"entries"`
}

type PlanDayDTO struct {
	Day     string        `json:"day"`
	Planned int           `json:"planned"`
	Items   []PlanItemDTO `json:"items"`
	// Untagged is the day's entries without a ticket; Suggest the planned ticket for them.
	Untagged []PlanEntryDTO `json:"untagged"`
	Suggest  *string        `json:"suggest"`
}

type PlansDTO struct {
	Days    []PlanDayDTO `json:"days"`
	Planned int          `json:"planned"`
	// Read: Tempo has been read back for the month (else there is nothing to show yet).
	Read bool `json:"read"`
}

func planEntries(logs []core.Worklog) []PlanEntryDTO {
	out := []PlanEntryDTO{}
	for _, w := range logs {
		out = append(out, PlanEntryDTO{
			ID: w.ID, Source: string(w.Source), Description: w.TempoText(), Seconds: w.Duration,
			Status: string(w.Status), Focus: view.SourceFocus(w),
		})
	}
	return out
}

// Plans is the month's Tempo plans and outside worklogs by day, each next to WorkLogger's own entries on the same
// ticket and day, from the last RefreshTempo. An empty month is the current one.
func (s *Service) Plans(month string) (PlansDTO, error) {
	out := PlansDTO{Days: []PlanDayDTO{}}
	if month == "" {
		month = s.today()[:7]
	}
	from, to, ok := s.Days.Month(month)
	if !ok {
		return out, fmt.Errorf("not a month: %q", month)
	}
	out.Read = s.readBack(from)
	if !out.Read {
		return out, nil // nothing to show next to until Tempo is read
	}
	logs, err := s.Store.Worklogs(from, to)
	if err != nil {
		return out, err
	}
	for _, d := range view.BuildPlans(s.planned(from, to), logs) {
		dd := PlanDayDTO{Day: d.Day, Planned: d.Planned, Items: []PlanItemDTO{}, Untagged: planEntries(d.Untagged), Suggest: d.Suggest}
		for _, it := range d.Items {
			dd.Items = append(dd.Items, PlanItemDTO{
				Kind: string(it.Kind), Key: it.Key, Day: it.Day, Seconds: it.Seconds, Description: it.Description,
				Logged: it.Logged, Entries: planEntries(it.Entries),
			})
		}
		out.Planned += d.Planned
		out.Days = append(out.Days, dd)
	}
	return out, nil
}
