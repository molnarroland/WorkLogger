package core

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// ChatExport is a chat history export (e.g. Google Chat), pasted as base64 of zlib-compressed JSON (PHP
// `gzcompress`) or as plain JSON.
type ChatExport struct {
	Source        string             `json:"source"`
	Conversations []ChatConversation `json:"conversations"`
}

type ChatConversation struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Name     *string       `json:"name"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Time   time.Time `json:"time"`
	Sender *string   `json:"sender"` // null when the exporter couldn't tell who wrote it
	Me     bool      `json:"me"`
	Text   string    `json:"text"`
	Extra  *string   `json:"extra"`
}

// DisplayName: DMs carry the other person's name; spaces have none, but each message's `extra` starts with it.
func (c ChatConversation) DisplayName() string {
	if c.Name != nil && *c.Name != "" {
		return *c.Name
	}
	if c.Type == "space" && len(c.Messages) > 0 && c.Messages[0].Extra != nil {
		if first := strings.Split(*c.Messages[0].Extra, " | ")[0]; first != "" {
			return first
		}
	}
	return "" // unknown: shown as UnknownName
}

// UnknownName labels a conversation whose name Chat didn't give: "Unknown #<id>".
func UnknownName(id string) string {
	return "Unknown #" + id[strings.LastIndex(id, "/")+1:]
}

// Files are the attached file names: in `extra` ("Space | Projektterület | , | a.png | b.png") everything after ",".
func (m ChatMessage) Files() []string {
	if m.Extra == nil {
		return nil
	}
	parts := strings.Split(*m.Extra, " | ")
	for i, p := range parts {
		if strings.TrimSpace(p) == "," {
			var out []string
			for _, f := range parts[i+1:] {
				if f = strings.TrimSpace(f); f != "" {
					out = append(out, f)
				}
			}
			return out
		}
	}
	return nil
}

// Body is the text without the export's "Ön:" ("You:") prefix on own messages.
func (m ChatMessage) Body() string {
	if i := strings.Index(m.Text, ":"); m.Me && i >= 0 && m.Text[:i] == "Ön" {
		return strings.TrimSpace(m.Text[i+1:])
	}
	return m.Text
}

var ErrChatNotRecognized = Err("chat.notRecognized")

func ParseChatExport(pasted string) (ChatExport, error) {
	text := strings.TrimSpace(pasted)
	data := []byte(text)
	if !strings.HasPrefix(text, "{") {
		clean := strings.Map(func(r rune) rune {
			if r == '+' || r == '/' || r == '=' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return -1 // like Foundation's .ignoreUnknownCharacters
		}, text)
		raw, err := base64.StdEncoding.DecodeString(clean)
		if err != nil || len(raw) <= 2 {
			return ChatExport{}, ErrChatNotRecognized
		}
		// PHP's gzcompress output may lack the adler32 trailer, so read raw DEFLATE after the 2-byte zlib header.
		if raw[0] == 0x78 {
			raw = raw[2:]
		}
		if data, err = io.ReadAll(flate.NewReader(bytes.NewReader(raw))); err != nil {
			return ChatExport{}, ErrChatNotRecognized
		}
	}
	var e ChatExport
	if err := json.Unmarshal(data, &e); err != nil {
		return ChatExport{}, err
	}
	return e, nil
}

type ChatLine struct {
	Time   time.Time
	Sender string
	Me     bool
	Text   string
	Files  []string
}

// ChatDay is one conversation's messages on one day, as listed on the Chats page.
type ChatDay struct {
	ConversationID, Name, Type, Day string
	Times                           []time.Time // sorted
	Gap                             time.Duration
}

// MinLoggableChat: shorter chats are not worth a worklog (seconds).
const MinLoggableChat = 15

const DefaultChatGap = 10 * time.Minute

func (c ChatDay) ID() string  { return c.ConversationID + "|" + c.Day }
func (c ChatDay) Ref() string { return "chat:" + c.ID() }

// ActiveSeconds: messages less than Gap apart form one burst; a burst lasts from its first to its last message,
// at least a minute.
func (c ChatDay) ActiveSeconds() int {
	if len(c.Times) == 0 {
		return 0
	}
	burst := func(a, b time.Time) time.Duration { return max(b.Sub(a), time.Minute) }
	start, last := c.Times[0], c.Times[0]
	var total time.Duration
	for _, t := range c.Times[1:] {
		if t.Sub(last) > c.Gap {
			total += burst(start, last)
			start = t
		}
		last = t
	}
	total += burst(start, last)
	return int(total.Seconds())
}
