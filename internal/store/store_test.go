package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:", ts.Days)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func addRepo(t *testing.T, s *Store, name string) core.Repo {
	t.Helper()
	must(t, s.AddRepo("/tmp/"+name, name))
	repos, _ := s.Repos()
	for _, r := range repos {
		if r.Name == name {
			return r
		}
	}
	t.Fatal("repo not added")
	return core.Repo{}
}

func count(t *testing.T, s *Store, table string) int {
	var n int
	must(t, s.db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&n))
	return n
}

func p[T any](v T) *T { return &v }

func TestSchema(t *testing.T) {
	s := open(t)
	var v int
	must(t, s.db.QueryRow("PRAGMA user_version").Scan(&v))
	if v != SchemaVersion || SchemaVersion != 10 {
		t.Errorf("migratesToTheLatestVersion: %d", v)
	}

	// upgradesAnOlderDatabaseAndKeepsItsData: every intermediate version comes up to date.
	for from := 0; from < SchemaVersion; from++ {
		path := filepath.Join(t.TempDir(), "old.sqlite")
		raw, err := sql.Open("sqlite3", path)
		must(t, err)
		old := &Store{db: raw, q: raw}
		must(t, old.migrateTo(from))
		if from > 0 {
			_, err = raw.Exec("INSERT INTO repos(path, name) VALUES('/w/billing', 'billing')")
			must(t, err)
		}
		raw.Close()
		up, err := Open(path, ts.Days)
		must(t, err)
		repos, _ := up.Repos()
		if n, err := up.ImportChats(core.ChatExport{Source: "t"}); err != nil || n != 0 || (from > 0 && len(repos) != 1) {
			t.Errorf("upgrade from %d: %v %v", from, err, repos)
		}
		up.Close()
	}

	boom := errors.New("boom")
	if err := s.Tx(func(tx *Store) error { must(t, tx.AddRepo("/tmp/x", "x")); return boom }); err != boom {
		t.Error(err)
	}
	if repos, _ := s.Repos(); len(repos) != 0 {
		t.Error("failedTransactionRollsBack")
	}
}

func TestSettingsStore(t *testing.T) {
	s := open(t)
	id := func() string { return "x" }
	want := core.DefaultSettings()
	want.KnownModules = core.AllModules
	if got := s.LoadSettings(id); !reflect.DeepEqual(got, want) {
		t.Errorf("settingsDefault: %+v", got)
	}
	st := want
	st.FixedDailyHours = 6
	must(t, s.SaveSettings(st))
	a := core.NewJiraAccount("a")
	a.BaseURL, a.Email = "https://acme.atlassian.net", "me@x.hu"
	st.JiraAccounts = []core.JiraAccount{a}
	// Tempo is a profile linking the account (without one, the account falls back to Jira's own worklogs).
	st.Profiles = []core.Profile{{ID: "t", Kind: core.ProfileTempo, JiraAccountID: "a", URL: a.TempoBaseURL}}
	must(t, s.SaveSettings(st)) // upsert
	if got := s.LoadSettings(id); !reflect.DeepEqual(got, st) {
		t.Errorf("roundTrip: %+v", got)
	}
}

func TestReposAndProjects(t *testing.T) {
	s := open(t)
	must(t, s.AddRepo("/tmp/billing", "billing"))
	r := addRepo(t, s, "billing")
	repos, _ := s.Repos()
	if len(repos) != 1 {
		t.Error("reposAreUniqueByPath")
	}
	r.IssueKey, r.Enabled = p("PROJ-1"), false
	must(t, s.UpdateRepo(r))
	if repos, _ := s.Repos(); !reflect.DeepEqual(repos, []core.Repo{r}) {
		t.Errorf("updatable: %+v", repos)
	}
	must(t, s.SaveCommits([]core.StoredCommit{ts.Commit("2026-03-03", "10:00", ts.RepoID(r.ID))}))
	must(t, s.DeleteRepo(r.ID))
	if count(t, s, "commits") != 0 {
		t.Error("deletingARepoDeletesItsCommits")
	}

	must(t, s.UpsertProject(core.ProjectMapping{Key: "PROJ", DefaultIssue: p("PROJ-1")}))
	must(t, s.UpsertProject(core.ProjectMapping{Key: "PROJ", DefaultIssue: p("PROJ-2")}))
	if ps, _ := s.Projects(); len(ps) != 1 || *ps[0].DefaultIssue != "PROJ-2" {
		t.Error("projectsUpsert")
	}
	must(t, s.DeleteProject("PROJ"))
	if ps, _ := s.Projects(); len(ps) != 0 {
		t.Error("projectsDelete")
	}
}

func TestCommits(t *testing.T) {
	s := open(t)
	off, on := addRepo(t, s, "off"), addRepo(t, s, "on")
	off.Enabled = false
	must(t, s.UpdateRepo(off))
	must(t, s.SaveCommits([]core.StoredCommit{
		ts.Commit("2026-03-03", "10:00", ts.Hash("a"), ts.RepoID(on.ID)),
		ts.Commit("2026-03-03", "11:00", ts.Hash("b"), ts.RepoID(off.ID)),
		ts.Commit("2026-03-04", "00:00", ts.Hash("c"), ts.RepoID(on.ID)),
	}))
	if cs, _ := s.Commits("2026-03-03"); len(cs) != 1 || cs[0].Hash != "a" {
		t.Errorf("commitsOfADayFromEnabledReposOnly: %+v", cs)
	}
	c := ts.Commit("2026-03-03", "10:00", ts.Hash("a"), ts.RepoID(on.ID))
	must(t, s.SaveCommits([]core.StoredCommit{c}))
	c.Insertions, c.Ref = 99, "refs/heads/feature"
	must(t, s.SaveCommits([]core.StoredCommit{c}))
	if cs, _ := s.Commits("2026-03-03"); len(cs) != 1 || cs[0].Insertions != 99 || cs[0].Ref != c.Ref || !cs[0].Date.Equal(c.Date) {
		t.Errorf("savingACommitAgainUpdatesItsStats: %+v", cs)
	}
}

func TestSessionsAndRecords(t *testing.T) {
	s := open(t)
	day := "2026-03-03"
	must(t, s.ImportTimesheet(core.ParsedTimesheet{Sessions: []core.WorkSession{
		ts.Session(day, "08:00", "12:00", core.SessionWork), ts.Session("2026-03-04", "08:00", "16:00", core.SessionWork),
	}}))
	record := core.DayRecord{Day: day, Status: core.DayLeave, Label: "Szabadság", ReportedWorked: p(8 * 3600)}
	must(t, s.ImportTimesheet(core.ParsedTimesheet{Days: []core.DayRecord{record}})) // leave: clears the day
	a, _ := s.Sessions(day)
	b, _ := s.Sessions("2026-03-04")
	recs, _ := s.DayRecords(day, day)
	if len(a) != 0 || len(b) != 1 || !reflect.DeepEqual(recs, []core.DayRecord{record}) {
		t.Errorf("importReplacesTheImportedDays: %v %v %+v", a, b, recs)
	}
	must(t, s.DeleteSessions(day, "2026-03-04"))
	b, _ = s.Sessions("2026-03-04")
	recs, _ = s.DayRecords(day, day)
	if len(b) != 0 || len(recs) != 0 {
		t.Error("clearingSessionsClearsDayRecords")
	}
}

func TestSyncMeetings(t *testing.T) {
	s := open(t)
	day := "2026-03-03"
	from, to := ts.Days.Bounds(day)
	m := func(id, a, b, cal string) core.Meeting {
		x := ts.Meeting(day, a, b, "Meeting")
		x.ID, x.CalendarID = id, cal
		return x
	}
	kept := m("kept", "09:00", "10:00", "work")
	all := map[string]bool{"work": true, "google": true, "old": true}
	must(t, s.SyncMeetings(from, to, all, all, []core.Meeting{
		kept, m("stale", "11:00", "12:00", "work"),
		m("other", "13:00", "14:00", "google"), m("inactive", "15:00", "16:00", "old"),
	}))
	must(t, s.SetMeetingSkipped("kept", true))
	renamed := kept
	renamed.Title = "Renamed"
	must(t, s.SyncMeetings(from, to, map[string]bool{"work": true}, map[string]bool{"work": true, "google": true}, []core.Meeting{renamed}))
	got, _ := s.MeetingsOn(day)
	if len(got) != 2 || got[0].ID != "kept" || got[1].ID != "other" || got[0].Title != "Renamed" || !got[0].Skipped {
		t.Errorf("syncKeepsSkippedAndRemovesStaleOrInactiveEvents: %+v", got)
	}
}

func TestWorklogs(t *testing.T) {
	s := open(t)
	w := ts.Worklog("2026-03-03", "09:00", 5400, core.SourceGit, "a,b", false)
	w.IssueKey = nil
	id, err := s.InsertWorklog(w)
	must(t, err)
	w.ID = id
	if got, _ := s.WorklogsOn("2026-03-03"); len(got) != 1 || !sameWorklog(got[0], w) {
		t.Errorf("worklogRoundTrip: %+v", got)
	}
	w.IssueKey, w.Status, w.Locked, w.TempoID, w.Day = p("PROJ-1"), core.StatusExported, true, p(int64(42)), "2026-03-04"
	must(t, s.UpdateWorklog(w))
	if got, _ := s.Worklogs("2026-03-01", "2026-03-31"); len(got) != 1 || !sameWorklog(got[0], w) {
		t.Errorf("update: %+v", got)
	}
	must(t, s.DeleteWorklog(w.ID))
	if count(t, s, "worklogs") != 0 {
		t.Error("delete")
	}

	day := "2026-03-03"
	entries := map[string]core.Worklog{
		"git draft": ts.Worklog(day, "09:00", 60, core.SourceGit, "", false), "meeting draft": ts.Worklog(day, "09:00", 60, core.SourceMeeting, "", false),
		"locked": ts.Worklog(day, "09:00", 60, core.SourceGit, "", true), "manual": ts.Worklog(day, "09:00", 60, core.SourceManual, "", false),
		"outside": ts.Worklog("2026-03-10", "09:00", 60, core.SourceGit, "", false),
	}
	reviewed := ts.Worklog(day, "09:00", 60, core.SourceGit, "", false)
	reviewed.Status = core.StatusReviewed
	entries["reviewed"] = reviewed
	for label, e := range entries {
		e.Description = label
		_, err := s.InsertWorklog(e)
		must(t, err)
	}
	must(t, s.DeleteRegeneratable("2026-03-02", "2026-03-06"))
	got, _ := s.Worklogs("2026-03-01", "2026-03-31")
	var left []string
	for _, w := range got {
		left = append(left, w.Description)
	}
	sort.Strings(left)
	if !reflect.DeepEqual(left, []string{"locked", "manual", "outside", "reviewed"}) {
		t.Errorf("onlyUnlockedGeneratedDraftsAreRegeneratable: %v", left)
	}
}

func sameWorklog(a, b core.Worklog) bool {
	return a.ID == b.ID && a.Day == b.Day && a.Start.Equal(b.Start) && a.Duration == b.Duration &&
		reflect.DeepEqual(a.IssueKey, b.IssueKey) && a.Description == b.Description && a.Source == b.Source &&
		a.SourceRef == b.SourceRef && reflect.DeepEqual(a.RepoID, b.RepoID) && a.Estimated == b.Estimated &&
		a.Status == b.Status && a.Locked == b.Locked && reflect.DeepEqual(a.TempoID, b.TempoID)
}

func TestIssueCache(t *testing.T) {
	s := open(t)
	a, b := "https://a.atlassian.net", "https://b.atlassian.net"
	if _, ok := s.CachedIssueID("PROJ-1", a); ok {
		t.Error("empty cache hit")
	}
	must(t, s.CacheIssueID("PROJ-1", 10001, a))
	must(t, s.CacheIssueID("PROJ-1", 10002, a))
	must(t, s.CacheIssueID("PROJ-1", 7, b)) // the same key on another site is another issue
	ia, _ := s.CachedIssueID("PROJ-1", a)
	ib, _ := s.CachedIssueID("PROJ-1", b)
	ka, _ := s.CachedIssueKey(10002, a)
	_, kb := s.CachedIssueKey(10002, b)
	if ia != 10002 || ib != 7 || ka != "PROJ-1" || kb {
		t.Error("issueIdCache")
	}
}

func TestChats(t *testing.T) {
	s := open(t)
	e, err := core.ParseChatExport(`{"source":"google-chat","conversations":[
      {"id":"space/A","type":"space","name":null,"messages":[
        {"time":"2026-03-03T08:00:00.000Z","sender":"me@x","me":true,"text":"Ön: hi","extra":"PROJ | Project | , | a.png | b c.xlsx"},
        {"time":"2026-03-03T08:05:00.000Z","sender":null,"me":false,"text":"yo","extra":"PROJ | Project | ,"},
        {"time":"2026-03-03T09:00:00.000Z","sender":"me@x","me":true,"text":"later"}]},
      {"id":"dm/B","type":"dm","name":"Bea","messages":[
        {"time":"2026-03-04T10:00:00Z","sender":"me@x","me":true,"text":"ping","extra":"Busy"}]}]}`)
	must(t, err)
	if n, _ := s.ImportChats(e); n != 4 {
		t.Errorf("first import: %d", n)
	}
	if n, _ := s.ImportChats(e); n != 0 {
		t.Errorf("second import: %d", n)
	}
	from := ts.At("2026-03-01", "00:00")
	days, _ := s.ChatDays(from, from.AddDate(0, 0, 31), core.DefaultChatGap)
	if len(days) != 2 || days[0].Name != "PROJ" || days[1].Name != "Bea" || len(days[0].Times) != 3 || len(days[1].Times) != 1 {
		t.Fatalf("%+v", days)
	}
	lines, _ := s.ChatLines("space/A", days[0].Day)
	if !reflect.DeepEqual(lines[0].Files, []string{"a.png", "b c.xlsx"}) || lines[0].Text != "hi" {
		t.Errorf("%+v", lines[0])
	}
}

// A conversation Chat gave no name for shows as "Unknown #<id>", and an unknown name never replaces a known one.
func TestUnknownChatName(t *testing.T) {
	s := open(t)
	imp := func(name string) {
		e, err := core.ParseChatExport(`{"source":"google-chat","conversations":[{"id":"space/AbC1","type":"space","name":` + name +
			`,"messages":[{"time":"2026-03-03T08:00:00Z","sender":"me@x","me":true,"text":"hi"}]}]}`)
		must(t, err)
		_, err = s.ImportChats(e)
		must(t, err)
	}
	from := ts.At("2026-03-01", "00:00")
	name := func() string {
		days, _ := s.ChatDays(from, from.AddDate(0, 0, 31), core.DefaultChatGap)
		return days[0].Name
	}
	if imp("null"); name() != "Unknown #AbC1" {
		t.Errorf("unknown: %q", name())
	}
	if imp(`"PROJ"`); name() != "PROJ" {
		t.Errorf("named later: %q", name())
	}
	if imp("null"); name() != "PROJ" {
		t.Errorf("kept the known name: %q", name())
	}
}

func TestRebuildDrafts(t *testing.T) {
	setup := func(t *testing.T) (*Store, core.Repo) {
		s := open(t)
		return s, addRepo(t, s, "billing")
	}
	commit := func(t *testing.T, s *Store, r core.Repo, hash, day, subject string) {
		must(t, s.SaveCommits([]core.StoredCommit{ts.Commit(day, "10:00", ts.Hash(hash), ts.Subject(subject), ts.RepoID(r.ID))}))
	}
	rebuild := func(t *testing.T, s *Store, st core.Settings) []core.Worklog {
		repos, _ := s.Repos()
		projects, _ := s.Projects()
		must(t, s.RebuildDrafts(ts.Builder(st, repos, projects), "2026-03-02", "2026-03-06"))
		logs, _ := s.Worklogs("2026-03-01", "2026-03-31")
		return logs
	}
	def := core.DefaultSettings()

	t.Run("commitsBecomeADayOfDrafts", func(t *testing.T) {
		s, r := setup(t)
		commit(t, s, r, "a", "2026-03-03", "PROJ-1 Fix")
		logs := rebuild(t, s, def)
		if len(logs) != 1 || logs[0].Day != "2026-03-03" || *logs[0].IssueKey != "PROJ-1" || logs[0].Duration != 8*3600 || logs[0].SourceRef != "a" {
			t.Errorf("%+v", logs)
		}
	})
	t.Run("rebuildingTwiceGivesTheSameDrafts", func(t *testing.T) {
		s, r := setup(t)
		commit(t, s, r, "a", "2026-03-03", "PROJ-1 Fix")
		commit(t, s, r, "b", "2026-03-05", "PROJ-2 Other")
		key := func(logs []core.Worklog) (out [][3]string) {
			for _, w := range logs {
				out = append(out, [3]string{w.Day, *w.IssueKey, w.SourceRef})
			}
			return
		}
		first, second := key(rebuild(t, s, def)), key(rebuild(t, s, def))
		if !reflect.DeepEqual(first, second) || len(first) != 2 {
			t.Errorf("%v %v", first, second)
		}
	})
	t.Run("saturdayCommitJustOutsideTheRangeLandsOnFriday", func(t *testing.T) {
		s, r := setup(t)
		commit(t, s, r, "sat", "2026-03-07", "PROJ-1 Fix")
		if logs := rebuild(t, s, def); len(logs) != 1 || logs[0].Day != "2026-03-06" || logs[0].SourceRef != "sat" {
			t.Errorf("%+v", logs)
		}
	})
	t.Run("lockedEntriesSurviveAndKeepTheirCommits", func(t *testing.T) {
		s, r := setup(t)
		commit(t, s, r, "a", "2026-03-03", "PROJ-1 Fix")
		entry := rebuild(t, s, def)[0]
		entry.Locked, entry.Description = true, "Edited by hand"
		must(t, s.UpdateWorklog(entry))
		if logs := rebuild(t, s, def); len(logs) != 1 || !sameWorklog(logs[0], entry) {
			t.Errorf("%+v", logs)
		}
	})
	t.Run("leaveDayGetsAbsenceAndItsCommitsMoveToTheDayBefore", func(t *testing.T) {
		s, r := setup(t)
		st := def
		st.AbsenceIssueKey = "HR-1"
		must(t, s.ImportTimesheet(core.ParsedTimesheet{Days: []core.DayRecord{{Day: "2026-03-04", Status: core.DayLeave, Label: "Szabadság"}}}))
		commit(t, s, r, "a", "2026-03-04", "PROJ-1 Fix")
		logs := rebuild(t, s, st)
		if len(logs) != 2 || logs[0].Day != "2026-03-03" || logs[1].Day != "2026-03-04" ||
			logs[0].Source != core.SourceGit || logs[1].Source != core.SourceAbsence || *logs[1].IssueKey != "HR-1" {
			t.Errorf("%+v", logs)
		}
	})
	t.Run("disabledRepositoriesAreIgnored", func(t *testing.T) {
		s, r := setup(t)
		commit(t, s, r, "a", "2026-03-03", "PROJ-1 Fix")
		r.Enabled = false
		must(t, s.UpdateRepo(r))
		if logs := rebuild(t, s, def); len(logs) != 0 {
			t.Errorf("%+v", logs)
		}
	})
	t.Run("entriesOutsideTheRangeAreLeftAlone", func(t *testing.T) {
		s, _ := setup(t)
		_, err := s.InsertWorklog(ts.Worklog("2026-03-10", "09:00", 3600, core.SourceGit, "", false))
		must(t, err)
		if logs := rebuild(t, s, def); len(logs) != 1 || logs[0].Day != "2026-03-10" {
			t.Errorf("%+v", logs)
		}
	})
}

func TestSentEmails(t *testing.T) {
	s := open(t)
	day := "2026-03-03"
	start, end := ts.Days.Bounds(day)
	must(t, s.SyncSentEmails(start, end, []SentEmail{
		{ID: "<1>", Subject: "Offsite", At: ts.At(day, "09:00")},
		{ID: "<2>", Subject: "PROJ-1 prices", At: ts.At(day, "10:00")},
		{ID: "<3>", Subject: "RE: offsite", At: ts.At(day, "11:00")},
	}, true))
	// Fetched again: one mail is gone (deleted from Sent), one is new; replace keeps only these.
	must(t, s.SyncSentEmails(start, end, []SentEmail{
		{ID: "<1>", Subject: "Offsite", At: ts.At(day, "09:00")},
		{ID: "<3>", Subject: "RE: offsite", At: ts.At(day, "11:00")},
		{ID: "<4>", Subject: "Fwd: Offsite", At: ts.At(day, "15:00")},
	}, true))
	got, err := s.EmailThreads(day)
	must(t, err)
	if len(got) != 1 || got[0].Subject != "Offsite" || got[0].Count != 3 ||
		!got[0].First.Equal(ts.At(day, "09:00")) || !got[0].Last.Equal(ts.At(day, "15:00")) {
		t.Errorf("%+v", got)
	}
}
