package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store is the sqlite pool over one app.db file.
type Store struct {
	DB      *sql.DB
	WriteMu sync.Mutex // serializes every write tx against duckdb read batches
}

// Open opens the pool with per-conn pragmas and runs the embedded migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=TRUNCATE&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		db.Close()
		return nil, err
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, db, sub)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err := p.Up(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return &Store{DB: db}, nil
}

// Close closes the pool.
func (s *Store) Close() error {
	return s.DB.Close()
}

// WithTx runs fn in one tx, holding WriteMu for the whole tx.
func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return tx.Commit()
}
