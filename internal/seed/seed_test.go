package seed

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func openSeeded(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := Load(s.DB); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func TestSeed_LoadsFixture(t *testing.T) {
	s := openSeeded(t)
	for table, want := range map[string]int{
		"patches":       2,
		"races":         3,
		"race_tiers":    9,
		"classes":       3,
		"class_tiers":   9,
		"heroes":        12,
		"hero_races":    13,
		"hero_classes":  13,
		"items":         8,
		"item_recipes":  4,
		"relics":        4,
		"pros":          4,
		"matches":       6,
		"lineups":       36,
		"lineup_slots":  109,
		"slot_items":    110,
		"lineup_relics": 36,
	} {
		var n int
		if err := s.DB.QueryRowContext(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != want {
			t.Errorf("%s: %d rows, want %d", table, n, want)
		}
	}
}

func TestSeed_ProInvariants(t *testing.T) {
	s := openSeeded(t)
	var n int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT count(*) FROM lineups l JOIN matches m ON m.id = l.match_id
		 WHERE m.source = 'pro' AND m.finalized_at > 0`).Scan(&n)
	if err != nil {
		t.Fatalf("count finalized pro lineups: %v", err)
	}
	if n == 0 || n%8 != 0 {
		t.Errorf("%d finalized pro lineups, want a count divisible by 8", n)
	}
	var avg float64
	err = s.DB.QueryRowContext(context.Background(),
		`SELECT avg(l.placement) FROM lineups l JOIN matches m ON m.id = l.match_id
		 WHERE m.source = 'pro' AND m.finalized_at > 0`).Scan(&avg)
	if err != nil {
		t.Fatalf("avg placement: %v", err)
	}
	if avg != 4.5 {
		t.Errorf("field avg placement = %g, want exactly 4.5", avg)
	}
}

func TestSeed_RefusesNonEmpty(t *testing.T) {
	s := openSeeded(t)
	if err := Load(s.DB); !errors.Is(err, ErrSeeded) {
		t.Errorf("second Load: err = %v, want ErrSeeded", err)
	}
}

func TestSeed_GoldenSpot(t *testing.T) {
	s := openSeeded(t)
	// grim jaw: the fixture comments hand-derive 8 picks with placements 1..8
	// once each, so top4 is 4 and the average is exactly 4.5.
	var picks, top4 int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT count(*),
		        sum(CASE WHEN l.placement <= 4 THEN 1 ELSE 0 END)
		 FROM lineup_slots ls
		 JOIN lineups l ON l.id = ls.lineup_id
		 JOIN matches m ON m.id = l.match_id
		 JOIN heroes h ON h.id = ls.hero_id
		 WHERE m.source = 'pro' AND m.finalized_at > 0 AND h.name = 'grim jaw'`).Scan(&picks, &top4)
	if err != nil {
		t.Fatalf("golden spot: %v", err)
	}
	if picks != 8 || top4 != 4 {
		t.Errorf("grim jaw: picks %d top4 %d, want 8 and 4", picks, top4)
	}
	var avg float64
	err = s.DB.QueryRowContext(context.Background(),
		`SELECT avg(l.placement)
		 FROM lineup_slots ls
		 JOIN lineups l ON l.id = ls.lineup_id
		 JOIN matches m ON m.id = l.match_id
		 JOIN heroes h ON h.id = ls.hero_id
		 WHERE m.source = 'pro' AND m.finalized_at > 0 AND h.name = 'grim jaw'`).Scan(&avg)
	if err != nil {
		t.Fatalf("golden spot avg: %v", err)
	}
	if avg != 4.5 {
		t.Errorf("grim jaw avg placement = %g, want exactly 4.5", avg)
	}
}

func TestSeed_BeginTxError(t *testing.T) {
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := Load(s.DB); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Load on closed db: err = %v, want closed-database error", err)
	}
}

func TestSeed_ExecErrorRollsBack(t *testing.T) {
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	const abortMsg = "seed-test blocked heroes insert"
	if _, err := s.DB.Exec(`CREATE TRIGGER block_heroes BEFORE INSERT ON heroes BEGIN SELECT RAISE(ABORT, '` + abortMsg + `'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	err = Load(s.DB)
	if err == nil {
		t.Fatal("Load with blocked heroes insert: nil error, want failure")
	}
	if errors.Is(err, ErrSeeded) {
		t.Errorf("Load failure misread as ErrSeeded: %v", err)
	}
	if !strings.Contains(err.Error(), abortMsg) {
		t.Errorf("Load error = %v, want trigger abort %q", err, abortMsg)
	}
	var n int
	if err := s.DB.QueryRowContext(context.Background(), `SELECT count(*) FROM matches`).Scan(&n); err != nil {
		t.Fatalf("count matches: %v", err)
	}
	if n != 0 {
		t.Errorf("matches count after failed Load = %d, want 0 (rolled back)", n)
	}
}
