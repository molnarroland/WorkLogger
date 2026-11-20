package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// Intent-level settings edits: the frontend says what changes, the backend makes the ids, normalises the input and
// keeps the rules (RELEASE_REVIEW B3). Each returns the saved settings.

var errNoIssue = core.Err("issue.missing")

// AddPeriod adds a period covering the month, to be named and given its issue.
func (s *Service) AddPeriod(month string) (core.Settings, error) {
	from, to, ok := s.Days.Month(month)
	if !ok {
		return s.current(), fmt.Errorf("not a month: %q", month)
	}
	return s.edit(func(st *core.Settings) error {
		st.Periods = append(st.Periods, core.ProjectPeriod{ID: NewID(), From: from, To: to})
		return nil
	})
}

// UpdatePeriod saves a period's title, issue and days. The issue is normalised; an end before the start moves
// to the start (moving the start past the end drags the end along).
func (s *Service) UpdatePeriod(p core.ProjectPeriod) (core.Settings, error) {
	if _, ok := s.Days.Parse(p.From); !ok {
		return s.current(), fmt.Errorf("not a day: %q", p.From)
	}
	if _, ok := s.Days.Parse(p.To); !ok {
		return s.current(), fmt.Errorf("not a day: %q", p.To)
	}
	p.IssueKey = keyOrEmpty(p.IssueKey)
	p.To = max(p.To, p.From)
	return s.edit(func(st *core.Settings) error {
		i := slices.IndexFunc(st.Periods, func(x core.ProjectPeriod) bool { return x.ID == p.ID })
		if i < 0 {
			return fmt.Errorf("no period %q", p.ID)
		}
		st.Periods[i] = p
		return nil
	})
}

func (s *Service) RemovePeriod(id string) (core.Settings, error) {
	return s.edit(func(st *core.Settings) error {
		st.Periods = slices.DeleteFunc(st.Periods, func(x core.ProjectPeriod) bool { return x.ID == id })
		return nil
	})
}

// AddMeetingRule logs meetings whose title contains the text to the issue.
func (s *Service) AddMeetingRule(contains, issueKey string) (core.Settings, error) {
	return s.addRule(func(st *core.Settings) *[]core.MeetingRule { return &st.MeetingRules }, contains, issueKey)
}

func (s *Service) RemoveMeetingRule(id string) (core.Settings, error) {
	return s.removeRule(func(st *core.Settings) *[]core.MeetingRule { return &st.MeetingRules }, id)
}

// AddAbsenceRule logs leave whose HR label contains the text to the issue.
func (s *Service) AddAbsenceRule(contains, issueKey string) (core.Settings, error) {
	return s.addRule(func(st *core.Settings) *[]core.MeetingRule { return &st.AbsenceRules }, contains, issueKey)
}

func (s *Service) RemoveAbsenceRule(id string) (core.Settings, error) {
	return s.removeRule(func(st *core.Settings) *[]core.MeetingRule { return &st.AbsenceRules }, id)
}

func (s *Service) addRule(list func(*core.Settings) *[]core.MeetingRule, contains, issueKey string) (core.Settings, error) {
	contains = strings.TrimSpace(contains)
	key := core.NormalizeKeyString(issueKey)
	if contains == "" {
		return s.current(), core.Err("rule.needText")
	}
	if key == nil {
		return s.current(), errNoIssue
	}
	return s.edit(func(st *core.Settings) error {
		l := list(st)
		// The same text again points its rule at the new issue rather than adding one that never matches first.
		if i := slices.IndexFunc(*l, func(r core.MeetingRule) bool { return strings.EqualFold(r.Contains, contains) }); i >= 0 {
			(*l)[i].IssueKey = *key
			return nil
		}
		*l = append(*l, core.MeetingRule{ID: NewID(), Contains: contains, IssueKey: *key})
		return nil
	})
}

func (s *Service) removeRule(list func(*core.Settings) *[]core.MeetingRule, id string) (core.Settings, error) {
	return s.edit(func(st *core.Settings) error {
		l := list(st)
		*l = slices.DeleteFunc(*l, func(r core.MeetingRule) bool { return r.ID == id })
		return nil
	})
}

// SetChatRule always logs a conversation to the issue (Chats → "Always"); an empty issue removes the rule.
func (s *Service) SetChatRule(conversationID, name, issueKey string) (core.Settings, error) {
	if strings.TrimSpace(conversationID) == "" {
		return s.current(), errors.New("no conversation")
	}
	key := core.NormalizeKeyString(issueKey)
	return s.edit(func(st *core.Settings) error {
		st.ChatRules = slices.DeleteFunc(st.ChatRules, func(r core.ChatRule) bool { return r.ConversationID == conversationID })
		if key != nil {
			st.ChatRules = append(st.ChatRules, core.ChatRule{ConversationID: conversationID, Name: name, IssueKey: *key})
		}
		return nil
	})
}

// AddWorkspaceRoot adds a folder to scan for repositories (once).
func (s *Service) AddWorkspaceRoot(path string) (core.Settings, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return s.current(), core.Err("folder.missing")
	}
	return s.edit(func(st *core.Settings) error {
		if !slices.Contains(st.WorkspaceRoots, path) {
			st.WorkspaceRoots = append(st.WorkspaceRoots, path)
		}
		return nil
	})
}

// RemoveWorkspaceRoot takes a folder off the list; its repositories stay until removed, nothing on disk changes.
func (s *Service) RemoveWorkspaceRoot(path string) (core.Settings, error) {
	return s.edit(func(st *core.Settings) error {
		st.WorkspaceRoots = slices.DeleteFunc(st.WorkspaceRoots, func(r string) bool { return r == path })
		return nil
	})
}

// edit applies change to a copy of the settings and saves it; on an error nothing changes.
func (s *Service) edit(change func(*core.Settings) error) (core.Settings, error) {
	err := s.updateOr(change)
	return s.current(), err
}

func keyOrEmpty(k string) string {
	if p := core.NormalizeKeyString(k); p != nil {
		return *p
	}
	return ""
}
