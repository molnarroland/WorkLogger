package app

import "github.com/molnarroland/WorkLogger/internal/core"

// SourceCountsDTO is how much each source page has for a day or a period, keyed like the pages: repositories with
// commits, meetings, work sessions, chats (conversation-days), Jira ticket-days, Confluence page-days and mail thread-days. A module that is off counts 0.
type SourceCountsDTO struct {
	Repositories int `json:"repositories"`
	Meetings     int `json:"meetings"`
	Sessions     int `json:"sessions"`
	Chats        int `json:"chats"`
	Jira         int `json:"jira"`
	Confluence   int `json:"confluence"`
	Email        int `json:"email"`
}

// SourceCounts counts what is stored for the days (inclusive), for the bottom bar's badges. Local data only.
func (s *Service) SourceCounts(from, to string) (SourceCountsDTO, error) {
	var out SourceCountsDTO
	start, end, err := s.span(from, to)
	if err != nil {
		return out, err
	}
	from, to = s.Days.Key(start), s.Days.Shift(s.Days.Key(end), -1)
	st := s.current()
	if st.Has(core.ModuleGit) {
		byRepo, err := s.Store.CommitDays(start, end)
		if err != nil {
			return out, err
		}
		out.Repositories = len(byRepo)
	}
	if st.Has(core.ModuleCalendar) {
		list, err := s.Store.Meetings(start, end)
		if err != nil {
			return out, err
		}
		out.Meetings = len(list)
	}
	if st.Has(core.ModuleWorkTimes) {
		list, err := s.Store.SessionsRange(from, to)
		if err != nil {
			return out, err
		}
		for _, w := range list {
			if w.Kind == core.SessionWork {
				out.Sessions++
			}
		}
	}
	if st.Has(core.ModuleChats) {
		list, err := s.Store.ChatDays(start, end, s.chatGap())
		if err != nil {
			return out, err
		}
		out.Chats = len(list)
	}
	if st.Has(core.ModuleJira) {
		for _, day := range s.Days.Range(from, to) {
			tickets, err := s.Store.JiraTickets(day)
			if err != nil {
				return out, err
			}
			out.Jira += len(tickets)
		}
	}
	if st.Has(core.ModuleConfluence) {
		for _, day := range s.Days.Range(from, to) {
			pages, err := s.Store.ConfluencePages(day)
			if err != nil {
				return out, err
			}
			out.Confluence += len(pages)
		}
	}
	if st.Has(core.ModuleEmail) {
		for _, day := range s.Days.Range(from, to) {
			threads, err := s.Store.EmailThreads(day)
			if err != nil {
				return out, err
			}
			out.Email += len(threads)
		}
	}
	return out, nil
}
