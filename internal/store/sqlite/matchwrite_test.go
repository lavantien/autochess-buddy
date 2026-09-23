package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

func TestAddLineup_PartialFailureRollsBackAll(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	// Relic 99 does not exist: the lineup insert succeeds, the relic insert fires the
	// foreign key mid-tx, and the whole tx rolls back.
	_, err := s.AddLineup(ctx, domain.AddLineupCmd{
		MatchID: 6, Label: "doomed", Placement: 4, RelicIDs: []int64{99},
	})
	if err == nil {
		t.Fatal("add lineup with dangling relic: want error, got nil")
	}
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM lineups WHERE label = 'doomed'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("doomed lineup rows = %d, err %v, want 0", n, err)
	}
}

func TestAddLineup_DuplicatePlacementMapsToConflictError(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	_, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 6, Label: "clash", Placement: 1})
	var pc *domain.PlacementConflictError
	if !errors.As(err, &pc) {
		t.Fatalf("duplicate placement err = %v, want PlacementConflictError", err)
	}
	if pc.Placement != 1 {
		t.Fatalf("conflict placement = %d, want 1", pc.Placement)
	}
}

func TestDeleteLineup_CascadesSlotsAndItems(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	if err := s.DeleteLineup(ctx, 34); err != nil {
		t.Fatalf("delete: %v", err)
	}
	counts := map[string]int{
		`SELECT count(*) FROM lineup_slots WHERE lineup_id = 34`:            0,
		`SELECT count(*) FROM slot_items WHERE slot_id BETWEEN 100 AND 105`: 0,
		`SELECT count(*) FROM lineup_relics WHERE lineup_id = 34`:           0,
		`SELECT count(*) FROM lineups WHERE match_id = 6`:                   2,
		`SELECT count(*) FROM lineup_slots WHERE lineup_id = 35`:            2,
	}
	for q, want := range counts {
		var n int
		if err := s.DB.QueryRowContext(ctx, q).Scan(&n); err != nil || n != want {
			t.Fatalf("%s = %d, err %v, want %d", q, n, err, want)
		}
	}
}

func TestCopyLineup_CopiesBoardResetsScalars(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	newID, err := s.CopyLineup(ctx, 34)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	_, lineups, err := s.GetMatch(ctx, 6)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var copy *domain.Lineup
	for i := range lineups {
		if lineups[i].ID == newID {
			copy = &lineups[i]
		}
	}
	if copy == nil {
		t.Fatalf("copy %d not in match 6", newID)
	}
	if copy.Label != "draft board a" {
		t.Fatalf("label = %q, want draft board a", copy.Label)
	}
	if copy.Placement != 4 {
		t.Fatalf("placement = %d, want 4 (first free)", copy.Placement)
	}
	if copy.ProID != nil || copy.Wins != 0 || copy.Draws != 0 || copy.Losses != 0 || copy.Networth != 0 {
		t.Fatalf("scalars not reset: %+v", copy)
	}
	if len(copy.Slots) != 6 {
		t.Fatalf("slots = %d, want 6", len(copy.Slots))
	}
	wantHeroes := []int64{1, 2, 3, 4, 5, 6}
	wantStars := []int{3, 2, 2, 2, 1, 1}
	wantItems := [][]int64{{1, 2}, {3}, {4}, {5}, {6}, {7}}
	for i, sl := range copy.Slots {
		if sl.SlotIndex != i || sl.Hero.ID != wantHeroes[i] || sl.Stars != wantStars[i] {
			t.Fatalf("slot %d = (idx %d, hero %d, %d stars), want (%d, %d, %d)",
				i, sl.SlotIndex, sl.Hero.ID, sl.Stars, i, wantHeroes[i], wantStars[i])
		}
		if len(sl.Items) != len(wantItems[i]) {
			t.Fatalf("slot %d items = %v, want %v", i, sl.Items, wantItems[i])
		}
		for j, it := range sl.Items {
			if it.ID != wantItems[i][j] {
				t.Fatalf("slot %d item %d = %d, want %d", i, j, it.ID, wantItems[i][j])
			}
		}
	}
	if len(copy.Relics) != 2 || copy.Relics[0].ID != 1 || copy.Relics[1].ID != 2 {
		t.Fatalf("relics = %+v, want 1 and 2", copy.Relics)
	}
}

func TestAddSlot_FillsLowestFreeCell(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	// Punch a hole at board index 2 of draft board a (slot 102, hero 3), then add:
	// the new cell must take index 2, not append after the tail.
	if err := s.DeleteSlot(ctx, 102); err != nil {
		t.Fatalf("punch hole: %v", err)
	}
	id, err := s.AddSlot(ctx, 34, 12, 2)
	if err != nil {
		t.Fatalf("add slot: %v", err)
	}
	var idx, stars int
	var hero int64
	if err := s.DB.QueryRowContext(ctx,
		`SELECT slot_index, stars, hero_id FROM lineup_slots WHERE id = ?`, id).
		Scan(&idx, &stars, &hero); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if idx != 2 || stars != 2 || hero != 12 {
		t.Fatalf("new slot = (idx %d, %d stars, hero %d), want (2, 2, 12)", idx, stars, hero)
	}
}

func TestFinalizeMatch_SetsFinalizedAt(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	if err := s.FinalizeMatch(ctx, 6); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	m, _, err := s.GetMatch(ctx, 6)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m.FinalizedAt <= 0 {
		t.Fatalf("finalized_at = %d, want > 0", m.FinalizedAt)
	}
	if err := s.FinalizeMatch(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing match err = %v, want ErrNotFound", err)
	}
}
