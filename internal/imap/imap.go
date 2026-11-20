// Package imap reads the subjects and times of the mail you sent, from an IMAP server over TLS. It speaks just
// enough IMAP4rev1 for that (AUTHENTICATE PLAIN, LIST, EXAMINE, UID SEARCH, UID FETCH of two headers), so the app
// needs no IMAP library, and it never changes the mailbox (EXAMINE is read-only, BODY.PEEK leaves \Seen alone).
package imap

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Dialer opens the connection to "host:port". The real one is TLS (DialTLS); tests pass a pipe.
type Dialer func(ctx context.Context, addr string) (net.Conn, error)

// DialTLS connects with TLS from the first byte (IMAPS, port 993). STARTTLS on 143 is not offered: the password
// would wait on a plain connection until the upgrade.
func DialTLS(ctx context.Context, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	d := tls.Dialer{Config: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}}
	return d.DialContext(ctx, "tcp", addr)
}

// Address is the server as "host:port", 993 when no port is given.
func Address(server string) string {
	server = strings.TrimSpace(server)
	server = strings.TrimPrefix(strings.TrimPrefix(server, "imaps://"), "imap://")
	server = strings.TrimRight(server, "/")
	if _, _, err := net.SplitHostPort(server); err != nil {
		return net.JoinHostPort(server, "993")
	}
	return server
}

// Sent is one mail you sent.
type Sent struct {
	ID      string // its Message-ID, else "<folder uid>"
	At      time.Time
	Subject string
}

// Session is one signed-in connection.
type Session struct {
	conn   net.Conn
	r      *bufio.Reader
	tag    int
	Folder string // the Sent folder, once found
}

// Login connects and signs in (AUTHENTICATE PLAIN, which carries any password unquoted).
func Login(ctx context.Context, dial Dialer, server, user, password string) (*Session, error) {
	conn, err := dial(ctx, Address(server))
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	s := &Session{conn: conn, r: bufio.NewReader(conn)}
	if greeting, _, err := s.read(); err != nil {
		conn.Close()
		return nil, err
	} else if !strings.HasPrefix(greeting, "* OK") && !strings.HasPrefix(greeting, "* PREAUTH") {
		conn.Close()
		return nil, fmt.Errorf("the server didn't greet: %s", greeting)
	}
	tag := s.next()
	if _, err := fmt.Fprintf(conn, "%s AUTHENTICATE PLAIN\r\n", tag); err != nil {
		conn.Close()
		return nil, err
	}
	if line, _, err := s.read(); err != nil || !strings.HasPrefix(line, "+") {
		conn.Close()
		return nil, fmt.Errorf("the server refused PLAIN sign-in: %s %v", line, err)
	}
	cred := base64.StdEncoding.EncodeToString([]byte("\x00" + user + "\x00" + password))
	if _, err := io.WriteString(conn, cred+"\r\n"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := s.until(tag); err != nil {
		conn.Close()
		return nil, ErrSignIn{err.Error()}
	}
	return s, nil
}

// ErrSignIn is the server refusing the user name or password.
type ErrSignIn struct{ Reason string }

func (e ErrSignIn) Error() string { return "sign-in refused: " + e.Reason }

// Close logs out.
func (s *Session) Close() error {
	_, _ = s.cmd("LOGOUT")
	return s.conn.Close()
}

var (
	listLine = regexp.MustCompile(`(?i)^\* LIST \(([^)]*)\) (?:"(?:[^"\\]|\\.)*"|NIL) (.+)$`)
	// Common names of the Sent folder on servers without the \Sent attribute (RFC 6154).
	sentNames = []string{"sent", "sent items", "sent messages", "sent mail", "inbox.sent", "inbox/sent", "elküldött elemek", "elküldött levelek"}
)

// FindSent finds the Sent folder: the one marked \Sent, else one with a usual name.
func (s *Session) FindSent() (string, error) {
	lines, err := s.cmd(`LIST "" "*"`)
	if err != nil {
		return "", err
	}
	var byName string
	for _, l := range lines {
		m := listLine.FindStringSubmatch(l.text)
		if m == nil {
			continue
		}
		name := unquote(m[2], l.literals)
		if strings.Contains(strings.ToLower(m[1]), `\sent`) {
			s.Folder = name
			return name, nil
		}
		if byName == "" {
			for _, n := range sentNames {
				if strings.EqualFold(name, n) || strings.HasSuffix(strings.ToLower(name), "/"+n) {
					byName = name
				}
			}
		}
	}
	if byName == "" {
		return "", errors.New("no Sent folder")
	}
	s.Folder = byName
	return byName, nil
}

var (
	searchLine = regexp.MustCompile(`(?i)^\* SEARCH\b(.*)$`)
	uidField   = regexp.MustCompile(`(?i)\bUID (\d+)`)
)

// SentBetween is the mail in the Sent folder dated in [from, to). IMAP searches by whole days in the server's
// zone, so a day on each side is searched and the Date headers trim it.
func (s *Session) SentBetween(from, to time.Time) ([]Sent, error) {
	if s.Folder == "" {
		if _, err := s.FindSent(); err != nil {
			return nil, err
		}
	}
	if _, err := s.cmd("EXAMINE " + quote(s.Folder)); err != nil {
		return nil, err
	}
	day := func(t time.Time) string { return t.Format("2-Jan-2006") }
	lines, err := s.cmd(fmt.Sprintf("UID SEARCH SINCE %s BEFORE %s", day(from.AddDate(0, 0, -1)), day(to.AddDate(0, 0, 1))))
	if err != nil {
		return nil, err
	}
	var uids []string
	for _, l := range lines {
		if m := searchLine.FindStringSubmatch(l.text); m != nil {
			uids = append(uids, strings.Fields(m[1])...)
		}
	}
	var out []Sent
	for len(uids) > 0 {
		n := min(len(uids), 200)
		batch := uids[:n]
		uids = uids[n:]
		lines, err := s.cmd("UID FETCH " + strings.Join(batch, ",") + " (UID BODY.PEEK[HEADER.FIELDS (DATE SUBJECT MESSAGE-ID)])")
		if err != nil {
			return nil, err
		}
		for _, l := range lines {
			if len(l.literals) == 0 || !strings.Contains(strings.ToUpper(l.text), "FETCH") {
				continue
			}
			msg, err := mail.ReadMessage(bytes.NewReader(append(l.literals[0], "\r\n\r\n"...)))
			if err != nil {
				continue
			}
			at, err := msg.Header.Date()
			if err != nil || at.Before(from) || !at.Before(to) {
				continue
			}
			id := strings.TrimSpace(msg.Header.Get("Message-Id"))
			if id == "" {
				if m := uidField.FindStringSubmatch(l.text); m != nil {
					id = "<" + s.Folder + " " + m[1] + ">"
				}
			}
			out = append(out, Sent{ID: id, At: at, Subject: decode(msg.Header.Get("Subject"))})
		}
	}
	return out, nil
}

// decode turns RFC 2047 words (=?UTF-8?Q?…?=) into text; a charset Go doesn't know stays as it came.
func decode(s string) string {
	if d, err := new(mime.WordDecoder).DecodeHeader(s); err == nil {
		return strings.TrimSpace(d)
	}
	return strings.TrimSpace(s)
}

// ── Protocol ────────────────────────────────────────────────────────────────

// response is one response line, with the literals ({n} and the n bytes after it) taken out: the text keeps a
// "\x00<i>" mark where literal i was.
type response struct {
	text     string
	literals [][]byte
}

func (s *Session) next() string {
	s.tag++
	return "a" + strconv.Itoa(s.tag)
}

var literalEnd = regexp.MustCompile(`\{(\d+)\+?\}$`)

// read reads one response line, following its literals.
func (s *Session) read() (string, [][]byte, error) {
	var text strings.Builder
	var lits [][]byte
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return "", nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		m := literalEnd.FindStringSubmatchIndex(line)
		if m == nil {
			text.WriteString(line)
			return text.String(), lits, nil
		}
		n, _ := strconv.Atoi(line[m[2]:m[3]])
		if n > 1<<20 {
			return "", nil, fmt.Errorf("a %d-byte literal: too large", n)
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(s.r, buf); err != nil {
			return "", nil, err
		}
		text.WriteString(line[:m[0]] + "\x00" + strconv.Itoa(len(lits)))
		lits = append(lits, buf)
	}
}

// until reads to the tagged answer: the untagged lines before it, or the server's refusal.
func (s *Session) until(tag string) ([]response, error) {
	var out []response
	for {
		text, lits, err := s.read()
		if err != nil {
			return nil, err
		}
		if rest, ok := strings.CutPrefix(text, tag+" "); ok {
			if strings.HasPrefix(strings.ToUpper(rest), "OK") {
				return out, nil
			}
			return nil, errors.New(rest)
		}
		out = append(out, response{text, lits})
	}
}

func (s *Session) cmd(command string) ([]response, error) {
	tag := s.next()
	if _, err := fmt.Fprintf(s.conn, "%s %s\r\n", tag, command); err != nil {
		return nil, err
	}
	out, err := s.until(tag)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Fields(command)[0], err)
	}
	return out, nil
}

// quote is an IMAP quoted string. Folder names are 7-bit (modified UTF-7), so a quoted string always fits.
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// unquote reads a mailbox name: a quoted string, a literal mark, or an atom.
func unquote(s string, lits [][]byte) string {
	if strings.HasPrefix(s, "\x00") {
		if i, err := strconv.Atoi(s[1:]); err == nil && i < len(lits) {
			return string(lits[i])
		}
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(s[1 : len(s)-1])
	}
	return s
}
