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

// Load applies the fixture in a single multi-statement exec. mattn only walks the
// statement tail when the exec carries no args, so the fixture must stay argument-free.
func Load(db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM matches`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrSeeded
	}
	_, err := db.Exec(fixture)
	return err
}
