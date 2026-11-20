package app

import (
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/view"
)

// settledDays are the days of [from, to] in weeks Tempo already has in full: the whole week is past, Tempo's
// read-back covers every one of its days, and each day with expected time has at least that much in Tempo.
// Collect skips them unless forced, and CleanUpSettled may drop their source data. A week without the read-back
// (its month not read from Tempo yet) is never settled.
func (s *Service) settledDays(from, to string) (map[string]bool, error) {
	out := map[string]bool{}
	today := s.today()
	for monday := view.WeekOf(s.Days, from)[0]; monday <= to; monday = s.Days.Shift(monday, 7) {
		week := view.WeekOf(s.Days, monday)
		if week[6] >= today {
			break
		}
		ok, err := s.weekSettled(week)
		if err != nil {
			return nil, err
		}
		if ok {
			for _, d := range week {
				if d >= from && d <= to {
					out[d] = true
				}
			}
		}
	}
	return out, nil
}

func (s *Service) weekSettled(week []string) (bool, error) {
	r, err := s.loadRange(week[0], week[6])
	if err != nil {
		return false, err
	}
	s.tempoMu.Lock()
	defer s.tempoMu.Unlock()
	expected := 0
	for _, d := range r.info {
		logs, read := s.tempoCache[d.Day]
		if !read {
			return false, nil
		}
		seconds := 0
		for _, w := range logs {
			seconds += w.Seconds
		}
		if seconds < d.Expected {
			return false, nil
		}
		expected += d.Expected
	}
	return expected > 0, nil // a week with nothing to log (a holiday week) proves nothing
}

// unsettled trims [from, to] to its first and last day outside settled weeks; ok is false when all of it is
// settled. Settled weeks between two unsettled ones stay in (Collect reads one span).
func unsettled(days []string, settled map[string]bool) (from, to string, ok bool) {
	for _, d := range days {
		if !settled[d] {
			if from == "" {
				from = d
			}
			to = d
		}
	}
	return from, to, from != ""
}

// CleanupDTO is what CleanUpSettled removed: the days it covered and the rows deleted.
type CleanupDTO struct {
	Days int `json:"days"`
	Rows int `json:"rows"`
}

// CleanUpSettled deletes the local source data (commits, meetings, chat messages) of the settled days in
// [from, to], then compacts the database. Worklogs, HRmaster sessions and day types stay: they are small and the
// views and the expected hours are built from them. Collect brings the data back when forced.
func (s *Service) CleanUpSettled(from, to string) (CleanupDTO, error) {
	if _, _, err := s.span(from, to); err != nil {
		return CleanupDTO{}, err
	}
	settled, err := s.settledDays(min(from, to), max(from, to))
	if err != nil || len(settled) == 0 {
		return CleanupDTO{}, err
	}
	out := CleanupDTO{Days: len(settled)}
	err = s.Store.Tx(func(db *store.Store) error {
		for d := range settled {
			n, err := db.DeleteSources(d)
			if err != nil {
				return err
			}
			out.Rows += n
		}
		return nil
	})
	if err != nil {
		return CleanupDTO{}, err
	}
	s.logFailure("store", "Compacting the database failed", s.Store.Vacuum())
	return out, nil
}
