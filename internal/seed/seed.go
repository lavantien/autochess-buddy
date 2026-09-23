// Package seed holds the shared fixture (readme:262): dev seeding, analytics goldens
// and the e2e fakes all read the same data.
package seed

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
)

//go:embed fixture.sql
var fixture string

var ErrSeeded = errors.New("database already has matches")

// Load applies the fixture atomically: the emptiness guard and the multi-statement
// exec share one tx, so a mid-script failure rolls the whole seed back. mattn only
// walks the statement tail when the exec carries no args, so the fixture must stay
// argument-free.
func Load(db *sql.DB) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM matches`).Scan(&n); err != nil {
		_ = tx.Rollback()
		return err
	}
	if n > 0 {
		_ = tx.Rollback()
		return ErrSeeded
	}
	if _, err := tx.Exec(fixture); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
