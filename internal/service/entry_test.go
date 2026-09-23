package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

func newService(t *testing.T) EntryService {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return EntryService{St: st}
}

func seedCodex(t *testing.T, svc EntryService) (patchID, h1, h2, itemID, relicID, proID int64) {
	t.Helper()
	ctx := context.Background()
	var err error
	patchID, err = svc.St.CreatePatch(ctx, domain.Patch{Version: "8.0", ReleasedAt: "2026-09-01"})
	if err != nil {
		t.Fatalf("create patch: %v", err)
	}
	race, err := svc.St.CreateRace(ctx, domain.Race{Name: "beast"}, nil)
	if err != nil {
		t.Fatalf("create race: %v", err)
	}
	class, err := svc.St.CreateClass(ctx, domain.Class{Name: "knight"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	h1, err = svc.St.CreateHero(ctx, domain.Hero{
		Name: "grim jaw", Cost: 2,
		Races:   []domain.Race{{ID: race}},
		Classes: []domain.Class{{ID: class}},
	})
	if err != nil {
		t.Fatalf("create hero: %v", err)
	}
	h2, err = svc.St.CreateHero(ctx, domain.Hero{
		Name: "sky breaker", Cost: 5,
		Races:   []domain.Race{{ID: race}},
		Classes: []domain.Class{{ID: class}},
	})
	if err != nil {
		t.Fatalf("create hero: %v", err)
	}
	itemID, err = svc.St.CreateItem(ctx, domain.Item{Name: "storm core", Tier: 3}, nil)
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	relicID, err = svc.St.CreateRelic(ctx, domain.Relic{Name: "tide bell", Effect: "heal on kill"})
	if err != nil {
		t.Fatalf("create relic: %v", err)
	}
	proID, err = svc.St.CreatePro(ctx, domain.Pro{Name: "drift", Handle: "drifttt", PeakRank: "challenger"})
	if err != nil {
		t.Fatalf("create pro: %v", err)
	}
	return patchID, h1, h2, itemID, relicID, proID
}

func seedProMatch(t *testing.T, svc EntryService) int64 {
	t.Helper()
	patchID, _, _, _, _, _ := seedCodex(t, svc)
	id, err := svc.CreateMatch(context.Background(), domain.Match{
		PatchID: patchID, PlayedAt: 1789000000, Source: "pro",
	})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}
	return id
}

func TestAddLineup_FriendlyPlacementConflict(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	patchID, _, _, _, _, _ := seedCodex(t, svc)
	matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}
	if _, err := svc.AddLineup(ctx, matchID, domain.AddLineupCmd{Label: "first", Placement: 1}, 0); err != nil {
		t.Fatalf("add first lineup: %v", err)
	}
	_, err = svc.AddLineup(ctx, matchID, domain.AddLineupCmd{Label: "clash", Placement: 1}, 0)
	var pc *domain.PlacementConflictError
	if !errors.As(err, &pc) {
		t.Fatalf("conflict err = %v, want PlacementConflictError", err)
	}
	if pc.Placement != 1 {
		t.Fatalf("conflict placement = %d, want 1", pc.Placement)
	}
	want := "placement 1 is already used by another lineup in this match."
	if pc.Error() != want {
		t.Fatalf("conflict copy = %q, want %q", pc.Error(), want)
	}
	view, err := svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if len(view.Lineups) != 1 {
		t.Fatalf("lineups = %d, want only the first one", len(view.Lineups))
	}
}

func TestAddLineup_UnknownHeroNameMapsToFriendlyCopy(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	matchID := seedProMatch(t, svc)

	cmd := domain.AddLineupCmd{
		Label: "ghost", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{Name: "ghost"}}},
	}
	if _, err := svc.AddLineup(ctx, matchID, cmd, 0); err == nil ||
		err.Error() != "no hero named ghost in the codex. add it first." {
		t.Fatalf("unknown hero err = %v, want friendly copy", err)
	}
	view, err := svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if len(view.Lineups) != 0 {
		t.Fatalf("lineups = %d, want 0 after failed add", len(view.Lineups))
	}
}

func TestFinalize_RejectsProSevenLineups(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	matchID := seedProMatch(t, svc)

	for i := 1; i <= 7; i++ {
		cmd := domain.AddLineupCmd{Label: fmt.Sprintf("board %d", i), Placement: i}
		if _, err := svc.AddLineup(ctx, matchID, cmd, 0); err != nil {
			t.Fatalf("add lineup %d: %v", i, err)
		}
	}
	err := svc.FinalizeMatch(ctx, matchID)
	if !errors.Is(err, domain.ErrProFinalize) {
		t.Fatalf("finalize err = %v, want ErrProFinalize", err)
	}
	view, err := svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if view.Match.FinalizedAt != 0 {
		t.Fatalf("finalized_at = %d, want still draft", view.Match.FinalizedAt)
	}
	if _, err := svc.AddLineup(ctx, matchID, domain.AddLineupCmd{Label: "board 8", Placement: 8}, 0); err != nil {
		t.Fatalf("add lineup 8: %v", err)
	}
	if err := svc.FinalizeMatch(ctx, matchID); err != nil {
		t.Fatalf("finalize with 8: %v", err)
	}
	view, err = svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if view.Match.FinalizedAt <= 0 {
		t.Fatalf("finalized_at = %d, want > 0", view.Match.FinalizedAt)
	}
}

func TestCopyLineup_BoardClonedLabelKeptPlacementNext(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	patchID, h1, h2, itemID, relicID, proID := seedCodex(t, svc)
	matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	cmd := domain.AddLineupCmd{
		Label: "round one", Placement: 1, ProID: &proID,
		Wins: 5, Draws: 1, Losses: 2, Networth: 4000,
		Slots: []domain.Slot{
			{SlotIndex: 0, Hero: domain.Hero{ID: h1}, Stars: 3, Items: []domain.Item{{ID: itemID}}},
			{SlotIndex: 1, Hero: domain.Hero{ID: h2}, Stars: 1},
		},
		RelicIDs: []int64{relicID},
	}
	first, err := svc.AddLineup(ctx, matchID, cmd, 0)
	if err != nil {
		t.Fatalf("add source lineup: %v", err)
	}
	if len(first.Lineups) != 1 {
		t.Fatalf("first view lineups = %d, want 1", len(first.Lineups))
	}
	boardID := first.Lineups[0].ID

	view, err := svc.AddLineup(ctx, matchID, domain.AddLineupCmd{}, boardID)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	var got *domain.Lineup
	for i := range view.Lineups {
		if view.Lineups[i].ID != boardID {
			got = &view.Lineups[i]
		}
	}
	if got == nil {
		t.Fatal("copy not found in editor view")
	}
	if got.Label != "round one" {
		t.Fatalf("label = %q, want round one", got.Label)
	}
	if got.Placement != 2 {
		t.Fatalf("placement = %d, want 2 (next free)", got.Placement)
	}
	if got.ProID != nil || got.Wins != 0 || got.Draws != 0 || got.Losses != 0 || got.Networth != 0 {
		t.Fatalf("scalars not reset: %+v", got)
	}
	if len(got.Slots) != 2 {
		t.Fatalf("slots = %d, want 2", len(got.Slots))
	}
	wantHeroes := []int64{h1, h2}
	wantStars := []int{3, 1}
	for i, sl := range got.Slots {
		if sl.SlotIndex != i || sl.Hero.ID != wantHeroes[i] || sl.Stars != wantStars[i] {
			t.Fatalf("slot %d = (idx %d, hero %d, %d stars), want (%d, %d, %d)",
				i, sl.SlotIndex, sl.Hero.ID, sl.Stars, i, wantHeroes[i], wantStars[i])
		}
	}
	if items := got.Slots[0].Items; len(items) != 1 || items[0].ID != itemID {
		t.Fatalf("slot 0 items = %+v, want [%d]", items, itemID)
	}
	if len(got.Relics) != 1 || got.Relics[0].ID != relicID {
		t.Fatalf("relics = %+v, want [%d]", got.Relics, relicID)
	}
}
