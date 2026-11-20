package imap

import (
	"strings"
	"testing"
	"time"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

func TestSentBetween(t *testing.T) {
	srv := &ts.FakeIMAP{User: "me@x.hu", Password: `p"ss`, Script: map[string]string{
		`LIST "" "*"`: "* LIST (\\HasNoChildren) \"/\" INBOX\r\n" +
			"* LIST (\\HasNoChildren \\Sent) \"/\" \"[Gmail]/Elk&APw-ld&APY-tt levelek\"\r\n",
		`EXAMINE "[Gmail]/Elk&APw-ld&APY-tt levelek"`:   "* 3 EXISTS\r\n",
		"UID SEARCH SINCE 2-Mar-2026 BEFORE 5-Mar-2026": "* SEARCH 7 8 9\r\n",
		"UID FETCH 7,8,9 (UID BODY.PEEK[HEADER.FIELDS (DATE SUBJECT MESSAGE-ID)])": ts.IMAPHeaders(1, 7, "Tue, 03 Mar 2026 09:15:00 +0100", "=?UTF-8?Q?PROJ-1_=C3=A1rlista?=", "<a@x>") +
			ts.IMAPHeaders(2, 8, "Tue, 03 Mar 2026 16:00:00 +0100", "Re: Plan", "") +
			ts.IMAPHeaders(3, 9, "Mon, 02 Mar 2026 23:00:00 +0100", "Yesterday", "<c@x>"),
		"LOGOUT": "* BYE\r\n",
	}}
	ctx := t.Context()
	if _, err := Login(ctx, srv.Dial, "imap.gmail.com", "me@x.hu", "wrong"); err == nil {
		t.Fatal("a wrong password signed in")
	}
	s, err := Login(ctx, srv.Dial, "imap.gmail.com", "me@x.hu", `p"ss`)
	if err != nil {
		t.Fatal(err)
	}
	from := ts.At("2026-03-03", "00:00")
	list, err := s.SentBetween(from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	must := func(ok bool, what string) {
		if !ok {
			t.Errorf("%s: %+v (commands %q)", what, list, srv.Commands())
		}
	}
	must(len(list) == 2, "the day's two mails, not yesterday's")
	must(len(list) > 0 && list[0].Subject == "PROJ-1 árlista" && list[0].ID == "<a@x>", "decoded subject")
	must(len(list) == 2 && list[1].ID == "<[Gmail]/Elk&APw-ld&APY-tt levelek 8>" && list[1].At.Equal(from.Add(16*time.Hour)), "no Message-ID")
	must(s.Close() == nil, "logout")
	for _, c := range srv.Commands() {
		if strings.Contains(c, "STORE") || strings.HasPrefix(c, "SELECT") {
			t.Errorf("changed the mailbox: %s", c)
		}
	}
}

func TestAddress(t *testing.T) {
	for in, want := range map[string]string{
		"imap.gmail.com": "imap.gmail.com:993", "imaps://mail.x.hu:1993/": "mail.x.hu:1993", " mail.x.hu ": "mail.x.hu:993",
	} {
		if got := Address(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
