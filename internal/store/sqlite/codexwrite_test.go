package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

func mustOK(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func wantSentinel(t *testing.T, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func wantErr(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: want error, got nil", what)
	}
}

func TestCodexCreate_DuplicateNaturalKeyFails(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	cases := []struct {
		name string
		dup  func() error
	}{
		{"race", func() error { _, err := s.CreateRace(ctx, domain.Race{Name: "twin"}, nil); return err }},
		{"class", func() error { _, err := s.CreateClass(ctx, domain.Class{Name: "twin"}, nil); return err }},
		{"hero", func() error { _, err := s.CreateHero(ctx, domain.Hero{Name: "twin", Cost: 2}); return err }},
		{"item", func() error { _, err := s.CreateItem(ctx, domain.Item{Name: "twin", Tier: 1}, nil); return err }},
		{"relic", func() error { _, err := s.CreateRelic(ctx, domain.Relic{Name: "twin"}); return err }},
		{"patch", func() error { _, err := s.CreatePatch(ctx, domain.Patch{Version: "9.0", ReleasedAt: "2026-01-01"}); return err }},
		{"pro", func() error { _, err := s.CreatePro(ctx, domain.Pro{Name: "twin"}); return err }},
	}
	for _, tc := range cases {
		mustOK(t, tc.dup(), tc.name+" first create")
		wantErr(t, tc.dup(), tc.name+" duplicate create")
	}
}

func TestCodexUpdate_DuplicateNaturalKeyFails(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	raceA, err := s.CreateRace(ctx, domain.Race{Name: "alpha race"}, nil)
	mustOK(t, err, "race a")
	_, err = s.CreateRace(ctx, domain.Race{Name: "beta race"}, nil)
	mustOK(t, err, "race b")
	classA, err := s.CreateClass(ctx, domain.Class{Name: "alpha class"}, nil)
	mustOK(t, err, "class a")
	_, err = s.CreateClass(ctx, domain.Class{Name: "beta class"}, nil)
	mustOK(t, err, "class b")
	heroA, err := s.CreateHero(ctx, domain.Hero{Name: "alpha hero", Cost: 1})
	mustOK(t, err, "hero a")
	_, err = s.CreateHero(ctx, domain.Hero{Name: "beta hero", Cost: 1})
	mustOK(t, err, "hero b")
	itemA, err := s.CreateItem(ctx, domain.Item{Name: "alpha item", Tier: 1}, nil)
	mustOK(t, err, "item a")
	_, err = s.CreateItem(ctx, domain.Item{Name: "beta item", Tier: 1}, nil)
	mustOK(t, err, "item b")
	relicA, err := s.CreateRelic(ctx, domain.Relic{Name: "alpha relic"})
	mustOK(t, err, "relic a")
	_, err = s.CreateRelic(ctx, domain.Relic{Name: "beta relic"})
	mustOK(t, err, "relic b")
	patchA, err := s.CreatePatch(ctx, domain.Patch{Version: "8.1", ReleasedAt: "2026-02-01"})
	mustOK(t, err, "patch a")
	_, err = s.CreatePatch(ctx, domain.Patch{Version: "8.2", ReleasedAt: "2026-02-02"})
	mustOK(t, err, "patch b")
	proA, err := s.CreatePro(ctx, domain.Pro{Name: "alpha pro"})
	mustOK(t, err, "pro a")
	_, err = s.CreatePro(ctx, domain.Pro{Name: "beta pro"})
	mustOK(t, err, "pro b")
	wantErr(t, s.UpdateRace(ctx, domain.Race{ID: raceA, Name: "beta race"}, nil), "race rename onto sibling")
	wantErr(t, s.UpdateClass(ctx, domain.Class{ID: classA, Name: "beta class"}, nil), "class rename onto sibling")
	wantErr(t, s.UpdateHero(ctx, domain.Hero{ID: heroA, Name: "beta hero", Cost: 1}), "hero rename onto sibling")
	wantErr(t, s.UpdateItem(ctx, domain.Item{ID: itemA, Name: "beta item"}, nil), "item rename onto sibling")
	wantErr(t, s.UpdateRelic(ctx, domain.Relic{ID: relicA, Name: "beta relic"}), "relic rename onto sibling")
	wantErr(t, s.UpdatePatch(ctx, domain.Patch{ID: patchA, Version: "8.2", ReleasedAt: "2026-02-02"}), "patch version onto sibling")
	wantErr(t, s.UpdatePro(ctx, domain.Pro{ID: proA, Name: "beta pro"}), "pro rename onto sibling")
}

func TestCodexUpdate_StaleIdsMapToNotFound(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	wantSentinel(t, s.UpdateClass(ctx, domain.Class{ID: 99, Name: "ghost"}, nil), domain.ErrNotFound)
	wantSentinel(t, s.UpdateHero(ctx, domain.Hero{ID: 99, Name: "ghost", Cost: 3}), domain.ErrNotFound)
	wantSentinel(t, s.UpdateItem(ctx, domain.Item{ID: 99, Name: "ghost"}, nil), domain.ErrNotFound)
	wantSentinel(t, s.UpdateRelic(ctx, domain.Relic{ID: 99, Name: "ghost"}), domain.ErrNotFound)
	wantSentinel(t, s.UpdatePatch(ctx, domain.Patch{ID: 99, Version: "9.9", ReleasedAt: "2026-01-01"}), domain.ErrNotFound)
}

func TestCreateHero_MissingLineageRefused(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	_, err := s.CreateHero(ctx, domain.Hero{Name: "shade", Cost: 2, Races: []domain.Race{{ID: 999}}})
	if !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("missing race lineage err = %v, want ErrInUse", err)
	}
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM heroes WHERE name = 'shade'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("hero rows = %d, err %v, want 0 after rollback", n, err)
	}
}

func TestCreateRace_DuplicateTierCountRefused(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	_, err := s.CreateRace(ctx, domain.Race{Name: "beast"},
		[]domain.Tier{{Count: 2, Effect: "one"}, {Count: 2, Effect: "two"}})
	wantErr(t, err, "duplicate tier count")
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM races WHERE name = 'beast'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("race rows = %d, err %v, want 0 after rollback", n, err)
	}
}
