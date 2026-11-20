package app

import (
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Collect reads the Sent folder into the store, the threads become drafts, and the Email page shows them; a
// wrong password is explained.
func TestEmail(t *testing.T) {
	s, _ := newService(t)
	s.Secrets.Unlock()
	srv := &ts.FakeIMAP{User: "me@x.hu", Password: "app-pass", Script: map[string]string{
		`LIST "" "*"`:          "* LIST (\\HasNoChildren) \"/\" \"Sent Items\"\r\n",
		`EXAMINE "Sent Items"`: "* 2 EXISTS\r\n",
		"UID SEARCH SINCE 2-Mar-2026 BEFORE 5-Mar-2026": "* SEARCH 1 2 3\r\n",
		"UID FETCH 1,2,3 (UID BODY.PEEK[HEADER.FIELDS (DATE SUBJECT MESSAGE-ID)])": ts.IMAPHeaders(1, 1, "Tue, 03 Mar 2026 09:00:00 +0100", "PROJ-1 prices", "<1@x>") +
			ts.IMAPHeaders(2, 2, "Tue, 03 Mar 2026 13:00:00 +0100", "Offsite", "<2@x>") +
			ts.IMAPHeaders(3, 3, "Tue, 03 Mar 2026 15:00:00 +0100", "RE: Offsite", "<3@x>"),
		"LOGOUT": "* BYE\r\n",
	}}
	s.DialIMAP = srv.Dial
	must(t, s.update(func(st *core.Settings) {
		st.Modules = []core.Module{core.ModuleEmail}
		st.Profiles = []core.Profile{{ID: "m1", Kind: core.ProfileEmail, Email: "me@x.hu", URL: "imap.x.hu"}}
	}))

	if lines, _ := s.CheckEmail("m1"); lines[0].OK == nil || *lines[0].OK || !strings.Contains(lines[0].Detail, "Enter the mail password") {
		t.Errorf("no password: %+v", lines)
	}
	must(t, s.SetToken("m1", "email", "wrong"))
	if lines, _ := s.CheckEmail("m1"); *lines[0].OK || !strings.Contains(lines[0].Detail, "app password") {
		t.Errorf("wrong password: %+v", lines)
	}
	must(t, s.SetToken("m1", "email", "app-pass"))
	if lines, _ := s.CheckEmail("m1"); !*lines[0].OK || !*lines[1].OK || !strings.Contains(lines[1].Detail, "Sent Items") {
		t.Errorf("check: %+v", lines)
	}

	n, failed := s.collectEmail(t.Context(), "2026-03-03", "2026-03-03")
	if n != 2 || len(failed) != 0 {
		t.Fatalf("collect: %d %v", n, failed)
	}
	must(t, s.rebuild(t.Context(), "2026-03-03", "2026-03-03"))
	feed, err := s.SentEmails("2026-03-03", "2026-03-03")
	must(t, err)
	if feed.Mails != 3 || feed.Threads != 2 || feed.Profiles != 1 || len(feed.Days) != 1 {
		t.Fatalf("feed: %+v", feed)
	}
	offsite, proj := feed.Days[0].Threads[0], feed.Days[0].Threads[1] // the latest first
	if offsite.Subject != "Offsite" || len(offsite.Times) != 2 || offsite.Subjects[1] != "RE: Offsite" || offsite.Entry == nil ||
		proj.IssueKey != "PROJ-1" || proj.Entry == nil || proj.Entry.Source != "email" {
		t.Errorf("threads: %+v / %+v", offsite, proj)
	}
	if _, err := s.RemoveProfile("m1"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Secrets.Get("email.password.m1"); v != "" {
		t.Error("the password stays behind")
	}
}
