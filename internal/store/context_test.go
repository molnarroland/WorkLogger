package store

import (
	"context"
	"errors"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// RELEASE_REVIEW C8: a cancelled job stops its writes; deleting a missing entry says so.
func TestStoreContextAndDelete(t *testing.T) {
	s := open(t)
	id, err := s.InsertWorklog(ts.Worklog("2026-03-03", "09:00", 600, core.SourceManual, "", false))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a cancelled context stops queries and transactions", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := s.WithContext(ctx).Worklogs("2026-03-01", "2026-03-31"); !errors.Is(err, context.Canceled) {
			t.Errorf("query: %v", err)
		}
		err := s.WithContext(ctx).Tx(func(db *Store) error { return db.DeleteWorklog(id) })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("tx: %v", err)
		}
		if list, _ := s.Worklogs("2026-03-01", "2026-03-31"); len(list) != 1 {
			t.Error("a cancelled transaction changed the data")
		}
	})
	t.Run("deleting a missing entry is ErrNotFound", func(t *testing.T) {
		if err := s.DeleteWorklog(id + 100); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v", err)
		}
		if err := s.DeleteWorklog(id); err != nil {
			t.Error(err)
		}
	})
}
