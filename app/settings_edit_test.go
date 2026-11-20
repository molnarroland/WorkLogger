package app

import "testing"

func TestSettingsEdits(t *testing.T) {
	s, _ := newService(t)

	t.Run("adds a period over the month with its own id, and keeps its end after its start", func(t *testing.T) {
		st, err := s.AddPeriod("2026-02")
		must(t, err)
		p := st.Periods[len(st.Periods)-1]
		if p.ID == "" || p.From != "2026-02-01" || p.To != "2026-02-28" {
			t.Fatalf("period %+v", p)
		}
		p.From, p.To, p.IssueKey = "2026-02-20", "2026-02-10", " proj-9 "
		st, err = s.UpdatePeriod(p)
		must(t, err)
		got := st.Periods[len(st.Periods)-1]
		if got.To != "2026-02-20" || got.IssueKey != "PROJ-9" {
			t.Errorf("updated %+v", got)
		}
		st, err = s.RemovePeriod(p.ID)
		must(t, err)
		for _, x := range st.Periods {
			if x.ID == p.ID {
				t.Error("not removed")
			}
		}
	})
	t.Run("refuses a rule without text or issue, and changes nothing then", func(t *testing.T) {
		before := len(s.Settings().MeetingRules)
		if _, err := s.AddMeetingRule("Standup", " "); err == nil {
			t.Error("no issue accepted")
		}
		if _, err := s.AddAbsenceRule("  ", "HR-1"); err == nil {
			t.Error("no text accepted")
		}
		if len(s.Settings().MeetingRules) != before {
			t.Error("a refused edit was saved")
		}
		st, err := s.AddMeetingRule(" Standup ", "int-12")
		must(t, err)
		r := st.MeetingRules[len(st.MeetingRules)-1]
		if r.Contains != "Standup" || r.IssueKey != "INT-12" || r.ID == "" {
			t.Errorf("rule %+v", r)
		}
		if st, _ = s.AddMeetingRule("standup", "INT-13"); len(st.MeetingRules) != before+1 || st.MeetingRules[len(st.MeetingRules)-1].IssueKey != "INT-13" {
			t.Errorf("the same text repoints its rule: %+v", st.MeetingRules)
		}
		st, err = s.RemoveMeetingRule(r.ID)
		must(t, err)
		if len(st.MeetingRules) != before {
			t.Error("not removed")
		}
	})
	t.Run("sets, replaces and clears a chat rule", func(t *testing.T) {
		_, err := s.SetChatRule("space/PROJ", "PROJ", "proj-1")
		must(t, err)
		st, err := s.SetChatRule("space/PROJ", "PROJ", "PROJ-2")
		must(t, err)
		if len(st.ChatRules) != 1 || st.ChatRules[0].IssueKey != "PROJ-2" {
			t.Errorf("rules %+v", st.ChatRules)
		}
		st, err = s.SetChatRule("space/PROJ", "PROJ", "")
		must(t, err)
		if len(st.ChatRules) != 0 {
			t.Error("not cleared")
		}
	})
	t.Run("adds a workspace folder once and removes it", func(t *testing.T) {
		n := len(s.Settings().WorkspaceRoots)
		_, err := s.AddWorkspaceRoot("~/Code")
		must(t, err)
		st, err := s.AddWorkspaceRoot("~/Code")
		must(t, err)
		if len(st.WorkspaceRoots) != n+1 {
			t.Errorf("roots %v", st.WorkspaceRoots)
		}
		st, err = s.RemoveWorkspaceRoot("~/Code")
		must(t, err)
		if len(st.WorkspaceRoots) != n {
			t.Errorf("roots %v", st.WorkspaceRoots)
		}
	})
}
