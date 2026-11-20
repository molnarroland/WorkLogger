package testsupport

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
)

// FakeIMAP is a scripted IMAP server over an in-memory pipe. AUTHENTICATE PLAIN accepts User and Password; any
// other command gets its Script lines (tag stripped from the key) then "<tag> OK", or "<tag> BAD" when unknown.
type FakeIMAP struct {
	User, Password string
	Script         map[string]string
	mu             sync.Mutex
	commands       []string
}

// Commands is what the server was sent, tags stripped.
func (f *FakeIMAP) Commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.commands...)
}

// Dial is an imap.Dialer.
func (f *FakeIMAP) Dial(context.Context, string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		r := bufio.NewReader(server)
		fmt.Fprint(server, "* OK ready\r\n")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			tag, cmd, _ := strings.Cut(strings.TrimRight(line, "\r\n"), " ")
			f.mu.Lock()
			f.commands = append(f.commands, cmd)
			f.mu.Unlock()
			if cmd == "AUTHENTICATE PLAIN" {
				fmt.Fprint(server, "+ \r\n")
				cred, _ := r.ReadString('\n')
				plain, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(cred))
				if string(plain) != "\x00"+f.User+"\x00"+f.Password {
					fmt.Fprintf(server, "%s NO [AUTHENTICATIONFAILED] Invalid credentials\r\n", tag)
				} else {
					fmt.Fprintf(server, "%s OK signed in\r\n", tag)
				}
				continue
			}
			reply, ok := f.Script[cmd]
			if !ok {
				fmt.Fprintf(server, "%s BAD unknown\r\n", tag)
				continue
			}
			fmt.Fprintf(server, "%s%s OK done\r\n", reply, tag)
		}
	}()
	return client, nil
}

// IMAPHeaders is a FETCH answer for a mail's Date, Subject and Message-ID headers.
func IMAPHeaders(seq, uid int, date, subject, id string) string {
	h := fmt.Sprintf("Date: %s\r\nSubject: %s\r\n", date, subject)
	if id != "" {
		h += "Message-ID: " + id + "\r\n"
	}
	h += "\r\n"
	return fmt.Sprintf("* %d FETCH (UID %d BODY[HEADER.FIELDS (DATE SUBJECT MESSAGE-ID)] {%d}\r\n%s)\r\n", seq, uid, len(h), h)
}
