package analytics

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/seed"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func openAnalytics(t *testing.T) (*sqlite.Store, *engine) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := seed.Load(s.DB); err != nil {
		_ = s.Close()
		t.Fatalf("seed: %v", err)
	}
	e, err := New(path, &s.WriteMu)
	if err != nil {
		_ = s.Close()
		t.Fatalf("open analytics: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() }) // closes before s.Close on the LIFO, releases the file on Windows
	t.Cleanup(func() { _ = s.Close() })
	return s, e
}

func countLineups(t *testing.T, e *engine) int {
	t.Helper()
	var n int
	err := e.batch(context.Background(), `SELECT count(*) FROM ac.lineups`, nil, func(rows *sql.Rows) error {
		for rows.Next() {
			if err := rows.Scan(&n); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("count ac.lineups: %v", err)
	}
	return n
}

func TestEngine_AttachesFixture(t *testing.T) {
	_, e := openAnalytics(t)
	if got := countLineups(t, e); got != 36 {
		t.Errorf("ac.lineups = %d rows, want 36", got)
	}
}

// A single quote in the db path must survive the ATTACH string literal via SQL
// doubling, else duckdb parses the statement apart. The row count also proves
// the attach resolved the right file and the lazy sqlite open answers queries.
func TestNew_QuoteInDbPathAttaches(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "qu'ote.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := seed.Load(st.DB); err != nil {
		_ = st.Close()
		t.Fatalf("seed: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	e, err := New(dbPath, &sync.Mutex{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if got := countLineups(t, e); got != 36 {
		t.Errorf("ac.lineups = %d rows through the quoted path, want 36", got)
	}
}

func TestWriteThenRead_DuckdbSeesCommittedSqlite(t *testing.T) {
	s, e := openAnalytics(t)
	err := s.WithTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO lineups (id, match_id, pro_id, label, placement, wins, draws, losses, networth, created_at)
			VALUES (37, 5, NULL, 'late board', 8, 0, 0, 0, 900, 1787210000)`)
		return err
	})
	if err != nil {
		t.Fatalf("insert lineup: %v", err)
	}
	if got := countLineups(t, e); got != 37 {
		t.Errorf("ac.lineups = %d rows after write, want 37", got)
	}
}

func TestEngine_BatchHoldsSharedMutex(t *testing.T) {
	_, e := openAnalytics(t)
	held := true
	err := e.batch(context.Background(), `SELECT id FROM ac.lineups ORDER BY id`, nil, func(rows *sql.Rows) error {
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if e.mu.TryLock() {
				e.mu.Unlock()
				held = false
			}
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if !held {
		t.Error("the batch let the shared mutex go mid-query")
	}
}
