package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	_ "github.com/duckdb/duckdb-go/v2"
)

// errFirstRun marks the INSTALL/LOAD path: the sqlite extension downloads from the
// internet on first use and fails without network access (readme:243).
var errFirstRun = errors.New("duckdb: the sqlite extension failed to load, it downloads on first run and needs network access")

// engine answers the views by querying the sqlite database through duckdb. The duckdb
// side is in-memory; the sqlite file attaches read-only as ac. Every batch holds the
// store's write mutex for the whole query with rows fully scanned inside the lock
// (readme:290), so a write tx can never interleave with an analytics read.
type engine struct {
	db *sql.DB
	mu *sync.Mutex
}

// New attaches the sqlite file at dbPath read-only. mu is the store's write mutex.
// The return widens to Service once the catalogue methods land on the engine.
func New(dbPath string, mu *sync.Mutex) (*engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	for _, stmt := range []string{`INSTALL sqlite`, `LOAD sqlite`} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, errors.Join(errFirstRun, err)
		}
	}
	attach := fmt.Sprintf(`ATTACH '%s' AS ac (TYPE SQLITE, READ_ONLY)`, filepath.ToSlash(dbPath))
	if _, err := db.Exec(attach); err != nil {
		db.Close()
		return nil, fmt.Errorf("attach %s: %w", dbPath, err)
	}
	// ATTACH lives on the connection that ran it, so the pool stays at one conn and
	// every batch sees ac.
	db.SetMaxOpenConns(1)
	return &engine{db: db, mu: mu}, nil
}

// Close releases the duckdb handle and with it the read lock on the sqlite file.
func (e *engine) Close() error {
	return e.db.Close()
}

// batch runs one catalogue query under the shared mutex, holding it across the query
// and the full row scan.
func (e *engine) batch(ctx context.Context, query string, args []any, scan func(*sql.Rows) error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	if err := scan(rows); err != nil {
		return err
	}
	return rows.Err()
}
