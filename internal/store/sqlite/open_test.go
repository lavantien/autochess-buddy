package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpen_MigratesFreshFile(t *testing.T) {
	store := openTemp(t)
	want := []string{
		"patches", "races", "race_tiers", "classes", "class_tiers",
		"heroes", "hero_races", "hero_classes", "items", "item_recipes",
		"relics", "pros", "matches", "lineups", "lineup_slots",
		"slot_items", "lineup_relics",
	}
	for _, table := range want {
		var n int
		err := store.DB.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("sqlite_master query for %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %s missing after migration", table)
		}
	}
	var n int
	if err := store.DB.QueryRow(
		`SELECT count(*) FROM pragma_table_info('matches') WHERE name = 'finalized_at'`,
	).Scan(&n); err != nil {
		t.Fatalf("pragma_table_info matches: %v", err)
	}
	if n != 1 {
		t.Error("matches.finalized_at missing, deviation 1 not applied")
	}
}

func TestOpen_SecondOpenIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	var afterFirst int
	if err := first.DB.QueryRow(`SELECT count(*) FROM goose_db_version`).Scan(&afterFirst); err != nil {
		t.Fatalf("version count after first Open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = second.Close() }()
	var afterSecond int
	if err := second.DB.QueryRow(`SELECT count(*) FROM goose_db_version`).Scan(&afterSecond); err != nil {
		t.Fatalf("version count after second Open: %v", err)
	}
	if afterFirst != afterSecond {
		t.Errorf("goose_db_version rows changed on reopen: %d -> %d", afterFirst, afterSecond)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	store := openTemp(t)
	boom := errors.New("boom")
	err := store.WithTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO patches (version, released_at) VALUES ('7.4', '2026-01-15')`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx error = %v, want boom", err)
	}
	var n int
	if err := store.DB.QueryRow(`SELECT count(*) FROM patches`).Scan(&n); err != nil {
		t.Fatalf("count patches: %v", err)
	}
	if n != 0 {
		t.Errorf("patches count = %d after rollback, want 0", n)
	}
}

func TestWithTx_HoldsWriteMu(t *testing.T) {
	store := openTemp(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := store.WithTx(context.Background(), func(tx *sql.Tx) error {
			if store.WriteMu.TryLock() {
				store.WriteMu.Unlock()
				t.Error("WriteMu not held inside WithTx")
			}
			return nil
		})
		if err != nil {
			t.Errorf("WithTx: %v", err)
		}
	}()
	<-done
	if !store.WriteMu.TryLock() {
		t.Fatal("WriteMu still held after WithTx returned")
	}
	store.WriteMu.Unlock()
}
