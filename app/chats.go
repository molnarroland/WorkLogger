package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/store"
)

// ── Chats ───────────────────────────────────────────────────────────────────

func (s *Service) chatGap() time.Duration {
	if g := s.current().ChatGapMinutes; g > 0 {
		return time.Duration(g) * time.Minute
	}
	return core.DefaultChatGap
}

func chatMinutes(c core.ChatDay) int { return (c.ActiveSeconds() + 59) / 60 }

// Chats is the conversation-days of the range under their days, each day with its active time.
func (s *Service) Chats(from, to string) (ChatsDTO, error) {
	list, err := s.chatList(from, to)
	return chatsDTO(list), err
}

func (s *Service) chatList(from, to string) ([]ChatDayDTO, error) {
	out := []ChatDayDTO{}
	start, end, err := s.span(from, to)
	if err != nil {
		return out, err
	}
	days, err := s.Store.ChatDays(start, end, s.chatGap())
	if err != nil {
		return out, err
	}
	rules := map[string]string{}
	for _, r := range s.current().ChatRules {
		rules[r.ConversationID] = r.IssueKey
	}
	for _, c := range days {
		d := ChatDayDTO{
			ID: c.ID(), Day: c.Day, ConversationID: c.ConversationID, Name: c.Name, Type: "space",
			Count: len(c.Times), ActiveSeconds: c.ActiveSeconds(), First: rfc(c.Times[0]), Last: rfc(c.Times[len(c.Times)-1]),
		}
		if c.Type == "dm" {
			d.Type = "dm"
		}
		if k := rules[c.ConversationID]; k != "" {
			d.RuleIssue = &k
		}
		if w, err := s.Store.WorklogBySourceRef(core.SourceChat, c.Ref()); err == nil && w != nil {
			d.IssueKey, d.Exported = w.IssueKey, w.Status == core.StatusExported
		} else {
			d.IssueKey = d.RuleIssue
		}
		out = append(out, d)
	}
	return out, nil
}

// chatDay finds a conversation-day by its id ("conversation|day").
func (s *Service) chatDay(id string) (core.ChatDay, error) {
	i := strings.LastIndex(id, "|")
	if i < 0 {
		return core.ChatDay{}, fmt.Errorf("not a chat id: %q", id)
	}
	start, end := s.Days.Bounds(id[i+1:])
	days, err := s.Store.ChatDays(start, end, s.chatGap())
	for _, c := range days {
		if c.ID() == id {
			return c, err
		}
	}
	return core.ChatDay{}, fmt.Errorf("no chat %q", id)
}

// ChatMessages is read on demand; message text is never logged.
func (s *Service) ChatMessages(id string) ([]ChatMessageDTO, error) {
	c, err := s.chatDay(id)
	if err != nil {
		return nil, err
	}
	lines, err := s.Store.ChatLines(c.ConversationID, c.Day)
	out := []ChatMessageDTO{}
	for _, l := range lines {
		files := l.Files
		if files == nil {
			files = []string{}
		}
		out = append(out, ChatMessageDTO{rfc(l.Time), l.Sender, l.Me, l.Text, files})
	}
	return out, err
}

// writeChatWorklog logs a conversation-day's active time to an issue (or re-targets its entry); locked so
// rebuilds keep it.
func (s *Service) writeChatWorklog(db *store.Store, c core.ChatDay, key string) error {
	w, err := db.WorklogBySourceRef(core.SourceChat, c.Ref())
	if err != nil {
		return err
	}
	if w != nil {
		if w.Status == core.StatusExported {
			return nil
		}
		w.IssueKey, w.Locked = &key, true // a chat draft from a rebuild becomes a kept entry
		return db.UpdateWorklog(*w)
	}
	_, err = db.InsertWorklog(core.Worklog{
		Day: c.Day, Start: c.Times[0], Duration: chatMinutes(c) * 60, IssueKey: &key,
		Description: "Chat: " + c.Name, Source: core.SourceChat, SourceRef: c.Ref(), Estimated: c.ActiveSeconds(),
		Status: core.StatusDraft, Locked: true,
	})
	return err
}

func (s *Service) LogChat(id, issueKey string) error {
	key := core.NormalizeKeyString(issueKey)
	if key == nil {
		return errNoIssue
	}
	c, err := s.chatDay(id)
	if err != nil {
		return err
	}
	if c.ActiveSeconds() <= core.MinLoggableChat {
		return core.Err("chat.tooShort")
	}
	return s.writeChatWorklog(s.Store, c, *key)
}

// applyChatRules logs every conversation-day in the range that has an "Always" rule and isn't logged yet.
func (s *Service) applyChatRules(start, end time.Time) error {
	rules := map[string]string{}
	for _, r := range s.current().ChatRules {
		if k := core.NormalizeKeyString(r.IssueKey); k != nil {
			if _, ok := rules[r.ConversationID]; !ok {
				rules[r.ConversationID] = *k
			}
		}
	}
	if len(rules) == 0 {
		return nil
	}
	days, err := s.Store.ChatDays(start, end, s.chatGap())
	if err != nil {
		return err
	}
	return s.Store.Tx(func(db *store.Store) error {
		for _, c := range days {
			key, ok := rules[c.ConversationID]
			if !ok || c.ActiveSeconds() <= core.MinLoggableChat {
				continue
			}
			if w, err := db.WorklogBySourceRef(core.SourceChat, c.Ref()); err != nil || w != nil {
				continue
			}
			if err := s.writeChatWorklog(db, c, key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ClearChats(from, to string) error {
	start, end, err := s.span(from, to)
	if err != nil {
		return err
	}
	return s.Store.DeleteChats(start, end)
}

// ImportChats takes a pasted export (base64 zlib JSON or plain JSON) and returns how many messages came in.
func (s *Service) ImportChats(text string) (int, error) {
	e, err := core.ParseChatExport(text)
	if err != nil {
		return 0, err
	}
	return s.importChats(e)
}

func (s *Service) importChats(e core.ChatExport) (int, error) {
	added, err := s.Store.ImportChats(e)
	if err != nil {
		return 0, err
	}
	var first, last time.Time
	for _, c := range e.Conversations {
		for _, m := range c.Messages {
			if first.IsZero() || m.Time.Before(first) {
				first = m.Time
			}
			if m.Time.After(last) {
				last = m.Time
			}
		}
	}
	if !first.IsZero() {
		s.logFailure("chats", "Applying the chat rules failed", s.applyChatRules(s.Days.StartOfDay(first), s.Days.DayAfter(last)))
	}
	return added, nil
}
