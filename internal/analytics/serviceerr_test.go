package analytics

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

func TestRateZeroDenominator(t *testing.T) {
	cases := []struct {
		name  string
		k, n  int
		ratio float64
	}{
		{"empty", 0, 0, 0},
		{"no picks", 3, 0, 0},
		{"half", 1, 2, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rate(tc.k, tc.n); got != tc.ratio {
				t.Errorf("rate(%d, %d) = %v, want %v", tc.k, tc.n, got, tc.ratio)
			}
		})
	}
}

// New must fail the ATTACH arm with a nil engine when the dbPath cannot build
// a valid ATTACH statement, after sql.Open and the extension load succeeded.
// The path is interpolated into the ATTACH string literal unescaped, so a
// single quote in it breaks the parse at Exec time; a directory or a non-sqlite
// file attach fine because duckdb opens the sqlite side lazily.
func TestNewAttachFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "it's.db")

	e, err := New(path, &sync.Mutex{})
	if e != nil {
		t.Errorf("New(%q): engine non-nil, want nil", path)
		_ = e.Close()
	}
	if err == nil {
		t.Fatal("New: error nil, want attach failure")
	}
	if !strings.Contains(err.Error(), "attach") {
		t.Errorf("New: error %q, want the attach wrap, not another arm", err)
	}
}

// Broken SQL must hit the query arm of batch and never reach the scan func.
func TestBatchQueryError(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	scanned := false
	err := e.batch(ctx, "SELEC", nil, func(*sql.Rows) error {
		scanned = true
		return nil
	})
	if err == nil {
		t.Fatal("batch(SELEC): error nil, want query failure")
	}
	if !strings.Contains(err.Error(), "query:") {
		t.Errorf("batch(SELEC): error %q, want the query wrap", err)
	}
	if scanned {
		t.Error("batch(SELEC): scan ran on a failed query")
	}
}

// Valid SQL with a failing scan func must propagate that scan error.
func TestBatchScanError(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	sentinel := errors.New("scan boom")
	ran := false
	err := e.batch(ctx, "SELECT 1", nil, func(*sql.Rows) error {
		ran = true
		return sentinel
	})
	if !ran {
		t.Fatal("batch: scan func never ran, the arm was not reached")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("batch scan error = %v, want the scan sentinel propagated", err)
	}
}

// Every Service wrapper must surface the batch error from an already-canceled
// context instead of returning rows.
func TestFiltersClauseNarrowQuery(t *testing.T) {
	_, e := openAnalytics(t)
	f := domain.Filter{PatchID: 1, Source: "pro"}
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"heroes", func() error { _, err := e.HeroPerformance(context.Background(), f); return err }},
		{"lineups in view", func() error { _, err := e.LineupsInView(context.Background(), f); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err != nil {
				t.Fatalf("%s with patch+source filter: %v", tc.name, err)
			}
		})
	}
}

func TestWrappersCanceledContext(t *testing.T) {
	_, e := openAnalytics(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := []struct {
		name string
		call func() error
	}{
		{"HeroPerformance", func() error { _, err := e.HeroPerformance(ctx, domain.Filter{}); return err }},
		{"SynergyPerformance", func() error { _, err := e.SynergyPerformance(ctx, domain.Filter{}); return err }},
		{"ItemPerformance", func() error { _, err := e.ItemPerformance(ctx, domain.Filter{}); return err }},
		{"RelicPerformance", func() error { _, err := e.RelicPerformance(ctx, domain.Filter{}); return err }},
		{"NetworthByPlacement", func() error { _, err := e.NetworthByPlacement(ctx, domain.Filter{}); return err }},
		{"LineupsInView", func() error { _, err := e.LineupsInView(ctx, domain.Filter{}); return err }},
	}
	for _, c := range calls {
		err := c.call()
		if err == nil {
			t.Errorf("%s(canceled ctx): error nil, want context canceled", c.name)
			continue
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s(canceled ctx): error %v, want context.Canceled in the chain", c.name, err)
		}
	}
}
