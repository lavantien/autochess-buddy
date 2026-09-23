package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/seed"
)

func seedTemp(t *testing.T) *Store {
	t.Helper()
	s := openTemp(t)
	if err := seed.Load(s.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func TestListMatches_FiltersPatchSourceState(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	cases := []struct {
		name   string
		filter domain.MatchFilter
		want   []int64 // match ids, newest played_at first
	}{
		{"all", domain.MatchFilter{}, []int64{6, 5, 4, 3, 2, 1}},
		{"patch 1", domain.MatchFilter{PatchID: 1}, []int64{2, 1}},
		{"patch 2", domain.MatchFilter{PatchID: 2}, []int64{6, 5, 4, 3}},
		{"source me", domain.MatchFilter{Source: "me"}, []int64{5}},
		{"source pro", domain.MatchFilter{Source: "pro"}, []int64{6, 4, 3, 2, 1}},
		{"state draft", domain.MatchFilter{State: "draft"}, []int64{6}},
		{"state final", domain.MatchFilter{State: "final"}, []int64{5, 4, 3, 2, 1}},
		{"pro final", domain.MatchFilter{Source: "pro", State: "final"}, []int64{4, 3, 2, 1}},
		{"patch 1 pro final", domain.MatchFilter{PatchID: 1, Source: "pro", State: "final"}, []int64{2, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := s.ListMatches(ctx, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(rows) != len(tc.want) {
				t.Fatalf("rows = %d, want %d: %+v", len(rows), len(tc.want), rows)
			}
			for i, r := range rows {
				if r.Match.ID != tc.want[i] {
					t.Fatalf("row %d = match %d, want %d", i, r.Match.ID, tc.want[i])
				}
			}
		})
	}

	// n/8 pip data and the patch column for the list page.
	rows, err := s.ListMatches(ctx, domain.MatchFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	wantCounts := map[int64]int{1: 8, 2: 8, 3: 8, 4: 8, 5: 1, 6: 3}
	wantPatches := map[int64]string{1: "7.4", 2: "7.4", 3: "7.5", 4: "7.5", 5: "7.5", 6: "7.5"}
	for _, r := range rows {
		if r.LineupCount != wantCounts[r.Match.ID] {
			t.Fatalf("match %d lineup count = %d, want %d", r.Match.ID, r.LineupCount, wantCounts[r.Match.ID])
		}
		if r.PatchVersion != wantPatches[r.Match.ID] {
			t.Fatalf("match %d patch = %q, want %q", r.Match.ID, r.PatchVersion, wantPatches[r.Match.ID])
		}
	}
}

func TestGetMatch_ReturnsSlotsItemsRelics(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	// The pro draft: 3 lineups, placements 1..3, the e2e edit substrate.
	m, lineups, err := s.GetMatch(ctx, 6)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m.Source != "pro" || m.PatchID != 2 || m.FinalizedAt != 0 {
		t.Fatalf("match = %+v", m)
	}
	if len(lineups) != 3 {
		t.Fatalf("lineups = %d, want 3", len(lineups))
	}
	for i, l := range lineups {
		if l.Placement != i+1 {
			t.Fatalf("lineup %d placement = %d, want %d", i, l.Placement, i+1)
		}
	}
	type wantSlot struct {
		hero  int64
		stars int
		items []int64
	}
	want := map[int64][]wantSlot{
		34: {
			{1, 3, []int64{1, 2}}, {2, 2, []int64{3}}, {3, 2, []int64{4}},
			{4, 2, []int64{5}}, {5, 1, []int64{6}}, {6, 1, []int64{7}},
		},
		35: {{7, 2, []int64{8}}, {8, 2, []int64{1}}},
		36: {{9, 2, []int64{2}}, {10, 3, []int64{3}}},
	}
	wantRelics := map[int64][]int64{34: {1, 2}, 35: {3}, 36: nil}
	for _, l := range lineups {
		ws := want[l.ID]
		if len(l.Slots) != len(ws) {
			t.Fatalf("lineup %d slots = %d, want %d", l.ID, len(l.Slots), len(ws))
		}
		for i, sl := range l.Slots {
			w := ws[i]
			if sl.SlotIndex != i {
				t.Fatalf("lineup %d slot %d index = %d, want %d", l.ID, i, sl.SlotIndex, i)
			}
			if sl.Hero.ID != w.hero || sl.Stars != w.stars {
				t.Fatalf("lineup %d slot %d = (hero %d, %d stars), want (%d, %d)", l.ID, i, sl.Hero.ID, sl.Stars, w.hero, w.stars)
			}
			if len(sl.Items) != len(w.items) {
				t.Fatalf("lineup %d slot %d items = %v, want %v", l.ID, i, sl.Items, w.items)
			}
			for j, it := range sl.Items {
				if it.ID != w.items[j] {
					t.Fatalf("lineup %d slot %d item %d = %d, want %d", l.ID, i, j, it.ID, w.items[j])
				}
			}
		}
		wr := wantRelics[l.ID]
		if len(l.Relics) != len(wr) {
			t.Fatalf("lineup %d relics = %+v, want %v", l.ID, l.Relics, wr)
		}
		for j, r := range l.Relics {
			if r.ID != wr[j] {
				t.Fatalf("lineup %d relic %d = %d, want %d", l.ID, j, r.ID, wr[j])
			}
		}
	}
	// slot heroes hydrate their codex face for rendering.
	if lineups[0].Slots[0].Hero.Name != "sky breaker" {
		t.Fatalf("slot hero name = %q, want sky breaker", lineups[0].Slots[0].Hero.Name)
	}

	// The me match: one lineup, no pro credit.
	_, mine, err := s.GetMatch(ctx, 5)
	if err != nil {
		t.Fatalf("get me match: %v", err)
	}
	if len(mine) != 1 || mine[0].ProID != nil || mine[0].Label != "my board" {
		t.Fatalf("me lineups = %+v", mine)
	}
	if len(mine[0].Slots) != 3 || mine[0].Slots[2].Hero.ID != 9 || mine[0].Slots[2].Stars != 3 {
		t.Fatalf("me slots = %+v", mine[0].Slots)
	}

	if _, _, err := s.GetMatch(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing match err = %v, want ErrNotFound", err)
	}
}
