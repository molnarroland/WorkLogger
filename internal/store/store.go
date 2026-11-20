// Package store is the SQLite database.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver" // pure Go (SQLite translated from Wasm), no cgo
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum"
)

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store is the database, or one transaction of it (inside Tx).
type Store struct {
	db   *sql.DB
	q    queryer
	ctx  context.Context // what its queries run under; nil means context.Background
	Path string
	Days core.Days
}

// WithContext is the store with its queries (and transactions begun from it) running under ctx, so cancelling a
// job stops its writes at the next statement (RELEASE_REVIEW C8).
func (s *Store) WithContext(ctx context.Context) *Store {
	c := *s
	c.ctx = ctx
	return &c
}

func (s *Store) context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

// ErrNotFound: the row to change or delete doesn't exist.
var ErrNotFound = errors.New("not found")

// Open opens (or creates) an unencrypted database at path; ":memory:" gives a private one for tests.
func Open(path string, days core.Days) (*Store, error) {
	return openDSN(path, path, "", days)
}

func openDSN(path, dsn, hexKey string, days core.Days) (*Store, error) {
	db, err := driver.Open(dsn, func(c *sqlite3.Conn) error {
		if hexKey == "" {
			return nil
		}
		// The key goes in by PRAGMA, first thing on each connection, not in the file name other code can read.
		return c.Exec("PRAGMA hexkey='" + hexKey + "'; PRAGMA temp_store=memory;")
	})
	if err != nil {
		return nil, err
	}
	// Known limit: one connection serialises all access; enough for one user, and :memory: stays one database.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, q: db, Path: path, Days: days}
	// A wrong key shows here: the file reads as "not a database".
	for _, p := range []string{"PRAGMA journal_mode=WAL;", "PRAGMA foreign_keys=ON;"} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Tx runs fn in one transaction (BEGIN IMMEDIATE semantics: the single connection is held throughout).
// Use only the Store passed to fn inside it.
func (s *Store) Tx(fn func(*Store) error) error {
	if _, ok := s.q.(*sql.Tx); ok {
		return fn(s) // already inside one
	}
	tx, err := s.db.BeginTx(s.context(), nil)
	if err != nil {
		return err
	}
	if err := fn(&Store{db: s.db, q: tx, ctx: s.ctx, Path: s.Path, Days: s.Days}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.q.ExecContext(s.context(), query, args...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.q.QueryContext(s.context(), query, args...)
}

func unix(t time.Time) int64 { return t.Round(time.Second).Unix() }

func fromUnix(n int64) time.Time { return time.Unix(n, 0) }

func nullString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

func intPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

// Migrations, oldest first: step N brings the database to user_version N. Append new steps; never edit old ones.
var migrations = []string{
	// 1: repos, projects, commits, sessions, meetings, worklogs, issue cache, settings
	`CREATE TABLE IF NOT EXISTS repos(
        id INTEGER PRIMARY KEY,
        path TEXT UNIQUE NOT NULL,
        name TEXT NOT NULL,
        issue_key TEXT,
        project_key TEXT,
        enabled INTEGER NOT NULL DEFAULT 1
    );
    CREATE TABLE IF NOT EXISTS projects(
        key TEXT PRIMARY KEY,
        default_issue TEXT
    );
    CREATE TABLE IF NOT EXISTS commits(
        repo_id INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
        hash TEXT NOT NULL,
        ts INTEGER NOT NULL,
        author TEXT,
        email TEXT,
        subject TEXT NOT NULL DEFAULT '',
        body TEXT NOT NULL DEFAULT '',
        ref TEXT NOT NULL DEFAULT '',
        files INTEGER NOT NULL DEFAULT 0,
        insertions INTEGER NOT NULL DEFAULT 0,
        deletions INTEGER NOT NULL DEFAULT 0,
        PRIMARY KEY(repo_id, hash)
    );
    CREATE INDEX IF NOT EXISTS idx_commits_ts ON commits(ts);
    CREATE TABLE IF NOT EXISTS sessions(
        id INTEGER PRIMARY KEY,
        day TEXT NOT NULL,
        start_ts INTEGER NOT NULL,
        end_ts INTEGER NOT NULL,
        kind TEXT NOT NULL,
        label TEXT NOT NULL DEFAULT ''
    );
    CREATE INDEX IF NOT EXISTS idx_sessions_day ON sessions(day);
    CREATE TABLE IF NOT EXISTS meetings(
        event_id TEXT PRIMARY KEY,
        calendar_id TEXT NOT NULL,
        title TEXT NOT NULL DEFAULT '',
        details TEXT NOT NULL DEFAULT '',
        start_ts INTEGER NOT NULL,
        end_ts INTEGER NOT NULL,
        skipped INTEGER NOT NULL DEFAULT 0
    );
    CREATE INDEX IF NOT EXISTS idx_meetings_start ON meetings(start_ts);
    CREATE TABLE IF NOT EXISTS worklogs(
        id INTEGER PRIMARY KEY,
        day TEXT NOT NULL,
        start_ts INTEGER NOT NULL,
        duration INTEGER NOT NULL,
        issue_key TEXT,
        description TEXT NOT NULL DEFAULT '',
        source TEXT NOT NULL,
        source_ref TEXT NOT NULL DEFAULT '',
        repo_id INTEGER,
        estimated INTEGER NOT NULL DEFAULT 0,
        status TEXT NOT NULL DEFAULT 'draft',
        locked INTEGER NOT NULL DEFAULT 0,
        tempo_id INTEGER
    );
    CREATE INDEX IF NOT EXISTS idx_worklogs_day ON worklogs(day);
    CREATE TABLE IF NOT EXISTS issue_cache(
        key TEXT PRIMARY KEY,
        issue_id INTEGER NOT NULL
    );
    CREATE TABLE IF NOT EXISTS settings(
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL
    );`,
	// 2: HRmaster day types and labels
	`CREATE TABLE IF NOT EXISTS day_info(
        day TEXT PRIMARY KEY,
        status TEXT NOT NULL,
        label TEXT NOT NULL DEFAULT '',
        reported_worked INTEGER,
        reported_breaks INTEGER
    );`,
	// 3: chat histories
	`CREATE TABLE IF NOT EXISTS chat_conversations(
        id TEXT PRIMARY KEY,
        source TEXT NOT NULL,
        type TEXT NOT NULL DEFAULT '',
        name TEXT NOT NULL DEFAULT ''
    );
    CREATE TABLE IF NOT EXISTS chat_messages(
        conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
        ts_ms INTEGER NOT NULL,
        sender TEXT NOT NULL,
        me INTEGER NOT NULL DEFAULT 0,
        text TEXT NOT NULL DEFAULT '',
        PRIMARY KEY(conversation_id, ts_ms, sender)
    );
    CREATE INDEX IF NOT EXISTS idx_chat_messages_ts ON chat_messages(ts_ms);`,
	// 4: files sent in chats
	`ALTER TABLE chat_messages ADD COLUMN files TEXT NOT NULL DEFAULT '';`,
	// 5: issue ids per Jira site (several accounts); the old cache is only a cache, so it starts over
	`DROP TABLE issue_cache;
    CREATE TABLE issue_cache(
        site TEXT NOT NULL,
        key TEXT NOT NULL,
        issue_id INTEGER NOT NULL,
        PRIMARY KEY(site, key)
    );`,
	// 6: the comment that goes to Tempo (Review); blank until written
	`ALTER TABLE worklogs ADD COLUMN comment TEXT NOT NULL DEFAULT '';`,
	// 7: your Jira activity (the Jira module): one row per thing you did on a ticket
	`CREATE TABLE IF NOT EXISTS jira_activity(
        issue_key TEXT NOT NULL,
        ts INTEGER NOT NULL,
        summary TEXT NOT NULL DEFAULT '',
        PRIMARY KEY(issue_key, ts)
    );
    CREATE INDEX IF NOT EXISTS idx_jira_activity_ts ON jira_activity(ts);`,
	// 8: your Confluence work (the Confluence module): one row per version you saved or comment you wrote
	`CREATE TABLE IF NOT EXISTS confluence_activity(
        page_id TEXT NOT NULL,
        ts INTEGER NOT NULL,
        title TEXT NOT NULL DEFAULT '',
        space TEXT NOT NULL DEFAULT '',
        PRIMARY KEY(page_id, ts)
    );
    CREATE INDEX IF NOT EXISTS idx_confluence_activity_ts ON confluence_activity(ts);`,
	// 9: the mail you sent (the Email module): its subject and time, by Message-ID; never its body
	`CREATE TABLE IF NOT EXISTS sent_emails(
        id TEXT PRIMARY KEY,
        ts INTEGER NOT NULL,
        subject TEXT NOT NULL DEFAULT ''
    );
    CREATE INDEX IF NOT EXISTS idx_sent_emails_ts ON sent_emails(ts);`,
}

// SchemaVersion is the user_version a fully migrated database has.
var SchemaVersion = len(migrations)

func (s *Store) migrate() error { return s.migrateTo(len(migrations)) }

func (s *Store) migrateTo(target int) error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < target; i++ {
		if _, err := s.db.Exec(migrations[i] + fmt.Sprintf("\nPRAGMA user_version = %d;", i+1)); err != nil {
			return fmt.Errorf("migrating to schema %d: %w", i+1, err)
		}
	}
	return nil
}
