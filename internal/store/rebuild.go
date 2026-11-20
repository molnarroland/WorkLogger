package store

import (
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// PaddingDays is how far outside the range commits are looked for: leave and idle rest days hand their commits to
// the closest workday, which may lie just outside the range.
const PaddingDays = 31

// RebuildDrafts replaces the unlocked drafts of fromDay..toDay with fresh ones built from stored commits, meetings
// and sessions, in one transaction.
func (s *Store) RebuildDrafts(b core.Builder, fromDay, toDay string) error {
	on := b.Settings
	return s.Tx(func(db *Store) error {
		if err := db.DeleteRegeneratable(fromDay, toDay); err != nil {
			return err
		}
		pad := PaddingDays
		// Pads from the exclusive end (the day after toDay).
		padFrom, padTo := s.Days.Shift(fromDay, -pad), s.Days.Shift(toDay, pad+1)
		// Twice the padding, so the closest-workday search from a padded leave day sees real day types.
		records := map[string]*core.DayRecord{}
		worked := map[string]bool{}
		if on.Has(core.ModuleWorkTimes) {
			var err error
			if records, err = db.DayRecordMap(s.Days.Shift(fromDay, -2*pad), s.Days.Shift(toDay, 2*pad+1)); err != nil {
				return err
			}
			sessions, err := db.SessionsRange(padFrom, padTo)
			if err != nil {
				return err
			}
			for _, w := range sessions {
				if w.Kind == core.SessionWork {
					worked[w.Day] = true
				}
			}
		}
		info := func(d string) *core.DayRecord { return records[d] }
		rangeDays := s.Days.Range(fromDay, toDay)
		inRange := map[string]bool{}
		for _, d := range rangeDays {
			inRange[d] = true
		}
		commitsByDay := map[string][]core.StoredCommit{}
		if on.Has(core.ModuleGit) {
			for _, day := range s.Days.Range(padFrom, padTo) {
				target := b.CommitDay(day, info, func(d string) bool { return worked[d] })
				if !inRange[day] && target == day {
					continue
				}
				list, err := db.Commits(day)
				if err != nil {
					return err
				}
				commitsByDay[target] = append(commitsByDay[target], list...)
			}
		}
		// Entries that survived the delete (locked, moved, reviewed) own their commits wherever they now sit.
		surviving, err := db.Worklogs(padFrom, padTo)
		if err != nil {
			return err
		}
		claimed := map[string]bool{}
		for _, w := range surviving {
			for _, r := range w.Refs() {
				claimed[r] = true
			}
		}
		for day, list := range commitsByDay {
			var keep []core.StoredCommit
			for _, c := range list {
				if !claimed[c.Hash] {
					keep = append(keep, c)
				}
			}
			commitsByDay[day] = keep
		}
		chatsByDay := map[string][]core.ChatDay{}
		if on.Has(core.ModuleChats) {
			gap := core.DefaultChatGap
			if g := on.ChatGapMinutes; g > 0 {
				gap = time.Duration(g) * time.Minute
			}
			start, _ := s.Days.Bounds(fromDay)
			_, end := s.Days.Bounds(toDay)
			chats, err := db.ChatDays(start, end, gap)
			if err != nil {
				return err
			}
			for _, c := range chats {
				chatsByDay[c.Day] = append(chatsByDay[c.Day], c)
			}
		}
		for _, day := range rangeDays {
			in := core.DayInput{Day: day, Commits: commitsByDay[day], Info: records[day], Chats: chatsByDay[day]}
			if on.Has(core.ModuleWorkTimes) {
				if in.Sessions, err = db.Sessions(day); err != nil {
					return err
				}
			}
			if on.Has(core.ModuleCalendar) {
				meetings, err := db.MeetingsOn(day)
				if err != nil {
					return err
				}
				for _, m := range meetings {
					if !claimed[m.ID] {
						in.Meetings = append(in.Meetings, m)
					}
				}
			}
			if on.Has(core.ModuleJira) {
				tickets, err := db.JiraTickets(day)
				if err != nil {
					return err
				}
				for _, t := range tickets {
					if !claimed[t.Ref()] {
						in.Jira = append(in.Jira, t)
					}
				}
			}
			if on.Has(core.ModuleConfluence) {
				pages, err := db.ConfluencePages(day)
				if err != nil {
					return err
				}
				for _, p := range pages {
					if !claimed[p.Ref()] {
						in.Confluence = append(in.Confluence, p)
					}
				}
			}
			if on.Has(core.ModuleEmail) {
				threads, err := db.EmailThreads(day)
				if err != nil {
					return err
				}
				for _, t := range threads {
					if !claimed[t.Ref()] {
						in.Email = append(in.Email, t)
					}
				}
			}
			if in.Kept, err = db.WorklogsOn(day); err != nil {
				return err
			}
			for _, w := range b.Build(in) {
				if _, err := db.InsertWorklog(w); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
