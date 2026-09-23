package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	sqlite3 "github.com/mattn/go-sqlite3"
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

func TestAddLineup_PersistsSlotItems(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	// Slot items ride the initial lineup write: hero 1 carries items 1 and 2.
	id, err := s.AddLineup(ctx, domain.AddLineupCmd{
		MatchID: 6, Label: "loaded", Placement: 4,
		Slots: []domain.Slot{
			{SlotIndex: 0, Hero: domain.Hero{ID: 1}, Stars: 2, Items: []domain.Item{{ID: 1}, {ID: 2}}},
			{SlotIndex: 1, Hero: domain.Hero{ID: 2}, Stars: 3},
		},
	})
	if err != nil {
		t.Fatalf("add lineup: %v", err)
	}
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM slot_items si JOIN lineup_slots ls ON ls.id = si.slot_id
		 WHERE ls.lineup_id = ?`, id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("slot items on new lineup = %d, err %v, want 2", n, err)
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

	for p := 4; p <= 8; p++ {
		if _, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 6, Label: "fill", Placement: p}); err != nil {
			t.Fatalf("fill placement %d: %v", p, err)
		}
	}
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

func TestFinalizedMatch_RejectsAllEdits(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	// Match 1 ships finalized with full boards; take one lineup and one slot.
	var lineupID, slotID int64
	if err := s.DB.QueryRowContext(ctx,
		`SELECT id FROM lineups WHERE match_id = 1 ORDER BY id LIMIT 1`).Scan(&lineupID); err != nil {
		t.Fatalf("lineup: %v", err)
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT ls.id FROM lineup_slots ls JOIN lineups l ON l.id = ls.lineup_id
		 WHERE l.match_id = 1 ORDER BY ls.id LIMIT 1`).Scan(&slotID); err != nil {
		t.Fatalf("slot: %v", err)
	}
	cases := []struct {
		name string
		call func() error
	}{
		{"add lineup", func() error { _, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 1, Placement: 3}); return err }},
		{"copy lineup", func() error { _, err := s.CopyLineup(ctx, lineupID); return err }},
		{"update lineup", func() error { return s.UpdateLineup(ctx, domain.Lineup{ID: lineupID, Placement: 3}) }},
		{"delete lineup", func() error { return s.DeleteLineup(ctx, lineupID) }},
		{"add slot", func() error { _, err := s.AddSlot(ctx, lineupID, 1, 2); return err }},
		{"set slot stars", func() error { return s.SetSlotStars(ctx, slotID, 3) }},
		{"delete slot", func() error { return s.DeleteSlot(ctx, slotID) }},
		{"add slot item", func() error { return s.AddSlotItem(ctx, slotID, 1) }},
		{"remove slot item", func() error { return s.RemoveSlotItem(ctx, slotID, 1) }},
		{"add lineup relic", func() error { return s.AddLineupRelic(ctx, lineupID, 1) }},
		{"remove lineup relic", func() error { return s.RemoveLineupRelic(ctx, lineupID, 1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, domain.ErrFinalized) {
				t.Fatalf("err = %v, want ErrFinalized", err)
			}
		})
	}
}

func TestFinalizeMatch_ValidatesInsideTxAndStampsOnce(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	// The draft (match 6) holds 3 lineups: the refusal must come from inside
	// the stamping tx and leave finalized_at untouched.
	if err := s.FinalizeMatch(ctx, 6); !errors.Is(err, domain.ErrProFinalize) {
		t.Fatalf("draft finalize err = %v, want ErrProFinalize", err)
	}
	var fin int64
	if err := s.DB.QueryRowContext(ctx,
		`SELECT finalized_at FROM matches WHERE id = 6`).Scan(&fin); err != nil || fin != 0 {
		t.Fatalf("refused finalize stamped anyway: %d err %v", fin, err)
	}

	// A me match with two lineups refuses too, even though drafts may rest at
	// any count.
	mid, err := s.CreateMatchShell(ctx, domain.Match{PatchID: 1, PlayedAt: 1, Source: "me"})
	if err != nil {
		t.Fatalf("me shell: %v", err)
	}
	for p := 1; p <= 2; p++ {
		if _, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: mid, Placement: p}); err != nil {
			t.Fatalf("me lineup %d: %v", p, err)
		}
	}
	if err := s.FinalizeMatch(ctx, mid); !errors.Is(err, domain.ErrMeFinalize) {
		t.Fatalf("me finalize err = %v, want ErrMeFinalize", err)
	}

	// Fill the pro draft to 8 and finalize; a second finalize is idempotent.
	for p := 4; p <= 8; p++ {
		if _, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 6, Label: "fill", Placement: p}); err != nil {
			t.Fatalf("fill placement %d: %v", p, err)
		}
	}
	if err := s.FinalizeMatch(ctx, 6); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	// Pin the stamp, then prove a second call cannot move it.
	if _, err := s.DB.ExecContext(ctx, `UPDATE matches SET finalized_at = 12345 WHERE id = 6`); err != nil {
		t.Fatalf("pin stamp: %v", err)
	}
	if err := s.FinalizeMatch(ctx, 6); err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT finalized_at FROM matches WHERE id = 6`).Scan(&fin); err != nil || fin != 12345 {
		t.Fatalf("re-stamp moved the marker: %d err %v, want 12345", fin, err)
	}
}

func TestAddLineup_DuplicateSlotIndexesRefusedBeforeInsert(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)

	_, err := s.AddLineup(ctx, domain.AddLineupCmd{
		MatchID: 6, Placement: 4,
		Slots: []domain.Slot{
			{SlotIndex: 2, Hero: domain.Hero{ID: 1}, Stars: 1},
			{SlotIndex: 2, Hero: domain.Hero{ID: 2}, Stars: 1},
		},
	})
	if !errors.Is(err, domain.ErrSlotTaken) {
		t.Fatalf("duplicate slot index err = %v, want ErrSlotTaken", err)
	}
}

func TestMapSlotTaken_UniqueMapsToFriendlyCopy(t *testing.T) {
	got := mapSlotTaken(sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintUnique})
	if !errors.Is(got, domain.ErrSlotTaken) {
		t.Fatalf("unique err mapped to %v, want ErrSlotTaken", got)
	}
	other := errors.New("boom")
	if got := mapSlotTaken(other); got != other {
		t.Fatalf("non-unique err must pass through, got %v", got)
	}
}

func TestWriteBadRefsRefuseAndPairsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)
	if _, err := s.CreateMatchShell(ctx, domain.Match{PatchID: 999, PlayedAt: 1, Source: "pro"}); err == nil {
		t.Fatal("shell with missing patch: want error")
	}
	_, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 99, Label: "x", Placement: 1})
	wantSentinel(t, err, domain.ErrNotFound)
	slot, err := s.AddSlot(ctx, 34, 5, 2)
	mustOK(t, err, "add slot")
	cases := []struct {
		name string
		call func() error
	}{
		{"stale hero in add lineup", func() error {
			_, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 6, Placement: 4, Slots: []domain.Slot{{Hero: domain.Hero{ID: 999}, Stars: 2}}})
			return err
		}},
		{"stale item in add lineup", func() error {
			_, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 6, Placement: 4, Slots: []domain.Slot{{Hero: domain.Hero{ID: 1}, Stars: 2, Items: []domain.Item{{ID: 999}}}}})
			return err
		}},
		{"stale hero in add slot", func() error { _, err := s.AddSlot(ctx, 34, 999, 2); return err }},
		{"stale item in add slot item", func() error { return s.AddSlotItem(ctx, slot, 999) }},
		{"stale relic in add lineup relic", func() error { return s.AddLineupRelic(ctx, 34, 999) }},
	}
	for _, tc := range cases {
		if err := tc.call(); err == nil {
			t.Fatalf("%s: want dangling-reference error", tc.name)
		}
	}
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM lineups WHERE match_id = 6`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("match 6 lineups = %d, err %v, want 3 after rollback", n, err)
	}
	mustOK(t, s.AddSlotItem(ctx, slot, 3), "add slot item")
	mustOK(t, s.RemoveSlotItem(ctx, slot, 3), "remove slot item")
	wantSentinel(t, s.RemoveSlotItem(ctx, slot, 3), domain.ErrNotFound)
	mustOK(t, s.AddLineupRelic(ctx, 36, 1), "add lineup relic")
	mustOK(t, s.RemoveLineupRelic(ctx, 36, 1), "remove lineup relic")
	wantSentinel(t, s.RemoveLineupRelic(ctx, 36, 1), domain.ErrNotFound)
}

func TestCopyLineup_PlacementExhausted(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)
	_, err := s.CopyLineup(ctx, 9999)
	wantSentinel(t, err, domain.ErrNotFound)
	for p := 4; p <= 8; p++ {
		_, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: 6, Label: "fill", Placement: p})
		mustOK(t, err, "fill")
	}
	_, err = s.CopyLineup(ctx, 34)
	if err == nil || !strings.Contains(err.Error(), "all 8 placements") {
		t.Fatalf("copy with full board err = %v, want placement exhaustion", err)
	}
}

func TestUpdateLineup_PersistsScalarsAndMapsStaleId(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)
	_, lineups, err := s.GetMatch(ctx, 6)
	if err != nil || len(lineups) == 0 || len(lineups[0].Slots) == 0 || lineups[0].ID != 34 {
		t.Fatalf("lineups = %+v, err %v, want lineup 34 with slots first", lineups, err)
	}
	board := lineups[0]
	board.Label, board.Wins, board.Networth = "renamed", 9, 12345
	mustOK(t, s.UpdateLineup(ctx, board), "update lineup")
	mustOK(t, s.SetSlotStars(ctx, board.Slots[0].ID, 3), "set slot stars")
	_, lineups, err = s.GetMatch(ctx, 6)
	if err != nil {
		t.Fatalf("reget after update: %v", err)
	}
	got := lineups[0]
	if got.Label != "renamed" || got.Wins != 9 || got.Networth != 12345 || got.Slots[0].Stars != 3 {
		t.Fatalf("after update = %+v slots[0] = %+v", got, got.Slots[0])
	}
	wantSentinel(t, s.UpdateLineup(ctx, domain.Lineup{ID: 9999, Placement: 1}), domain.ErrNotFound)
}
