package app

import (
	"fmt"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/view"
)

// ── Work sessions ───────────────────────────────────────────────────────────

// Sessions is the Work sessions page: each day's sessions next to HR's record, with net and reported time.
func (s *Service) Sessions(month string) (SessionsDTO, error) {
	out := SessionsDTO{Days: []SessionDayDTO{}}
	from, to, ok := s.Days.Month(month)
	if !ok {
		return out, fmt.Errorf("not a month: %q", month)
	}
	if !s.current().Has(core.ModuleWorkTimes) {
		return out, nil
	}
	list, err := s.Store.SessionsRange(from, to)
	if err != nil {
		return out, err
	}
	records, err := s.Store.DayRecords(from, to)
	if err != nil {
		return out, err
	}
	return sessionsDTO(view.SessionDays(list, sessionRecords(records, s.Days)), s.Days), nil
}

func sessionRecords(records []core.DayRecord, d core.Days) []view.SessionRecord {
	var out []view.SessionRecord
	for _, r := range records {
		worked := 0
		if r.ReportedWorked != nil {
			worked = *r.ReportedWorked
		}
		out = append(out, view.SessionRecord{Day: r.Day, Status: view.RecordStatus(r.Status, r.Day, d), Label: r.Label, Reported: worked})
	}
	return out
}

func (s *Service) ClearSessions(from, to string) error { return s.Store.DeleteSessions(from, to) }
