package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// assertFieldErrors pins a ValidationError's collected field errors verbatim,
// in collection order.
func assertFieldErrors(t *testing.T, err error, want []domain.FieldError) {
	t.Helper()
	if err == nil {
		t.Fatal("err = nil, want ValidationError")
	}
	var ve domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want ValidationError", err, err)
	}
	if len(ve) != len(want) {
		t.Fatalf("field errors = %v, want %d entries %v", ve, len(want), want)
	}
	for i, w := range want {
		if ve[i] != w {
			t.Fatalf("field error %d = %+v, want %+v", i, ve[i], w)
		}
	}
}

func TestCreateMatch_SourceValidationError(t *testing.T) {
	svc := newService(t)
	id, err := svc.CreateMatch(context.Background(), domain.Match{PatchID: 1, PlayedAt: 1789000000, Source: "bot"})
	if id != 0 {
		t.Fatalf("id = %d, want 0 on validation failure", id)
	}
	assertFieldErrors(t, err, []domain.FieldError{{Field: "source", Msg: "source must be me or pro."}})
}

func TestCreateMatch_UnknownPatchPassthrough(t *testing.T) {
	svc := newService(t)
	_, err := svc.CreateMatch(context.Background(), domain.Match{PatchID: 999, PlayedAt: 1789000000, Source: "pro"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown patch err = %v, want ErrNotFound passthrough", err)
	}
}

func TestAddLineup_FinalizedGuard(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	matchID := seedProMatch(t, svc)

	for i := 1; i <= 8; i++ {
		cmd := domain.AddLineupCmd{Label: fmt.Sprintf("board %d", i), Placement: i}
		if _, _, err := svc.AddLineup(ctx, matchID, cmd, 0); err != nil {
			t.Fatalf("add lineup %d: %v", i, err)
		}
	}
	if err := svc.FinalizeMatch(ctx, matchID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	view, newID, err := svc.AddLineup(ctx, matchID, domain.AddLineupCmd{Label: "late", Placement: 8}, 0)
	if newID != 0 {
		t.Fatalf("newID = %d, want 0 on finalized guard", newID)
	}
	if len(view.Lineups) != 0 {
		t.Fatalf("view lineups = %d, want empty view on guard", len(view.Lineups))
	}
	assertFieldErrors(t, err, []domain.FieldError{{Field: "match", Msg: "this match is finalized and can no longer be edited."}})

	after, err := svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor after guard: %v", err)
	}
	if after.Match.FinalizedAt <= 0 {
		t.Fatalf("finalized_at = %d, want > 0", after.Match.FinalizedAt)
	}
	if len(after.Lineups) != 8 {
		t.Fatalf("lineups = %d, want the 8 stored ones", len(after.Lineups))
	}
}

func TestAddLineup_UnknownMatchAndUnknownCopySource(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	if _, _, err := svc.AddLineup(ctx, 999, domain.AddLineupCmd{Label: "x", Placement: 1}, 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown match err = %v, want ErrNotFound passthrough", err)
	}

	matchID := seedProMatch(t, svc)
	if _, _, err := svc.AddLineup(ctx, matchID, domain.AddLineupCmd{}, 9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown copy source err = %v, want ErrNotFound", err)
	}
	view, err := svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if len(view.Lineups) != 0 {
		t.Fatalf("lineups = %d, want 0 after failed copy", len(view.Lineups))
	}
}

func TestValidateLineup_FieldArms(t *testing.T) {
	thirteen := func(h int64) []domain.Slot {
		slots := make([]domain.Slot, 13)
		for i := range slots {
			slots[i] = domain.Slot{SlotIndex: i, Hero: domain.Hero{ID: h}, Stars: 1}
		}
		return slots
	}
	sevenItems := func() []domain.Item {
		items := make([]domain.Item, 7)
		for i := range items {
			items[i] = domain.Item{ID: 1}
		}
		return items
	}
	const (
		placementCopy = "placement must be between 1 and 8."
		recordCopy    = "wins, draws, losses and networth cannot be negative."
		proCopy       = "pick a pro from the list."
		relicCopy     = "pick a relic from the list."
	)
	badPro := int64(999)
	cases := []struct {
		name string
		mut  func(c *domain.AddLineupCmd, h int64)
		want []domain.FieldError
	}{
		{"placement low", func(c *domain.AddLineupCmd, _ int64) { c.Placement = 0 },
			[]domain.FieldError{{Field: "placement", Msg: placementCopy}}},
		{"placement high", func(c *domain.AddLineupCmd, _ int64) { c.Placement = 9 },
			[]domain.FieldError{{Field: "placement", Msg: placementCopy}}},
		{"negative wins", func(c *domain.AddLineupCmd, _ int64) { c.Wins = -1 },
			[]domain.FieldError{{Field: "record", Msg: recordCopy}}},
		{"negative networth", func(c *domain.AddLineupCmd, _ int64) { c.Networth = -1 },
			[]domain.FieldError{{Field: "record", Msg: recordCopy}}},
		{"thirteen slots", func(c *domain.AddLineupCmd, h int64) { c.Slots = thirteen(h) },
			[]domain.FieldError{{Field: "board", Msg: domain.ErrSlotCap.Error()}}},
		{"unknown pro", func(c *domain.AddLineupCmd, _ int64) { c.ProID = &badPro },
			[]domain.FieldError{{Field: "pro", Msg: proCopy}}},
		{"stars out of range", func(c *domain.AddLineupCmd, h int64) {
			c.Slots = []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: h}, Stars: 9}}
		}, []domain.FieldError{{Field: "stars", Msg: domain.ErrStarsRange.Error()}}},
		{"seven items", func(c *domain.AddLineupCmd, h int64) {
			c.Slots = []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: h}, Stars: 1, Items: sevenItems()}}
		}, []domain.FieldError{{Field: "items", Msg: domain.ErrItemCap.Error()}}},
		{"unknown relic", func(c *domain.AddLineupCmd, _ int64) { c.RelicIDs = []int64{999} },
			[]domain.FieldError{{Field: "relic", Msg: relicCopy}}},
		{"collects in order", func(c *domain.AddLineupCmd, h int64) {
			c.Placement = 0
			c.Wins = -1
			c.Slots = thirteen(h)
			c.ProID = &badPro
			c.RelicIDs = []int64{999}
		}, []domain.FieldError{
			{Field: "placement", Msg: placementCopy},
			{Field: "record", Msg: recordCopy},
			{Field: "board", Msg: domain.ErrSlotCap.Error()},
			{Field: "pro", Msg: proCopy},
			{Field: "relic", Msg: relicCopy},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc := newService(t)
			patchID, h1, _, _, _, _ := seedCodex(t, svc)
			matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
			if err != nil {
				t.Fatalf("create match: %v", err)
			}
			cmd := domain.AddLineupCmd{Label: "board", Placement: 1}
			tc.mut(&cmd, h1)
			_, newID, err := svc.AddLineup(ctx, matchID, cmd, 0)
			if newID != 0 {
				t.Fatalf("newID = %d, want 0 on validation failure", newID)
			}
			assertFieldErrors(t, err, tc.want)
		})
	}
}

func TestAddLineup_HeroByNameResolvesAndSaves(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	patchID, h1, _, _, _, _ := seedCodex(t, svc)
	matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	cmd := domain.AddLineupCmd{
		Label: "free text", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{Name: "grim jaw"}, Stars: 2}},
	}
	view, newID, err := svc.AddLineup(ctx, matchID, cmd, 0)
	if err != nil {
		t.Fatalf("add lineup: %v", err)
	}
	if newID <= 0 {
		t.Fatalf("newID = %d, want > 0", newID)
	}
	if len(view.Lineups) != 1 || len(view.Lineups[0].Slots) != 1 {
		t.Fatalf("view = %d lineups, %d slots, want 1 each", len(view.Lineups), len(view.Lineups[0].Slots))
	}
	got := view.Lineups[0].Slots[0].Hero
	if got.ID != h1 || got.Name != "grim jaw" {
		t.Fatalf("resolved hero = %d %q, want %d grim jaw", got.ID, got.Name, h1)
	}
	if view.Lineups[0].Slots[0].Stars != 2 {
		t.Fatalf("stars = %d, want 2", view.Lineups[0].Slots[0].Stars)
	}
}

func TestAddLineup_HeroByIDUnknownBareReturn(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	patchID, _, _, _, _, _ := seedCodex(t, svc)
	matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	cmd := domain.AddLineupCmd{
		Label: "stale board", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: 999, Name: "phantom"}, Stars: 1}},
	}
	_, newID, err := svc.AddLineup(ctx, matchID, cmd, 0)
	if newID != 0 {
		t.Fatalf("newID = %d, want 0", newID)
	}
	want := "no hero named phantom in the codex. add it first."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want bare copy %q", err, want)
	}
	var ve domain.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("err = %v, want bare error, not a collected ValidationError", err)
	}
}

func TestValidateLineup_HeroIDMissingWithoutName(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	patchID, _, _, _, _, _ := seedCodex(t, svc)
	matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	cmd := domain.AddLineupCmd{
		Label: "stale board", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: 999}, Stars: 2}},
	}
	_, newID, err := svc.AddLineup(ctx, matchID, cmd, 0)
	if newID != 0 {
		t.Fatalf("newID = %d, want 0 on validation failure", newID)
	}
	assertFieldErrors(t, err, []domain.FieldError{{Field: "hero", Msg: "pick a hero from the codex."}})
}

func TestEditor_PatchDeleteRefusedBehindMatchAndUnknownMatch(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	patchID, _, _, _, _, _ := seedCodex(t, svc)
	matchID, err := svc.CreateMatch(ctx, domain.Match{PatchID: patchID, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	// The match holds an FK on the patch, so the delete is refused and the
	// editor keeps rendering: the GetPatch failure arm is unreachable this way.
	if err := svc.St.DeletePatch(ctx, patchID); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete patch err = %v, want ErrInUse wrap", err)
	}
	view, err := svc.Editor(ctx, matchID)
	if err != nil {
		t.Fatalf("editor after refused delete: %v", err)
	}
	if view.PatchVersion != "8.0" {
		t.Fatalf("patch version = %q, want 8.0", view.PatchVersion)
	}

	if _, err := svc.Editor(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("editor unknown match err = %v, want ErrNotFound passthrough", err)
	}
}
