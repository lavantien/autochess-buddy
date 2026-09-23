package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// seedTempMatch builds a draft match on the seed fixture (patch 1, source per
// arg) plus one lineup at placement 1 with a slot on hero 1.
func seedTempMatch(t *testing.T, source string) (*Store, int64, int64) {
	t.Helper()
	s := seedTemp(t)
	ctx := context.Background()
	id, err := s.CreateMatchShell(ctx, domain.Match{PatchID: 1, PlayedAt: 1789000000, Source: source})
	if err != nil {
		t.Fatalf("match shell: %v", err)
	}
	lid, err := s.AddLineup(ctx, domain.AddLineupCmd{
		MatchID: id, Label: "one", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: 1}, Stars: 1}},
	})
	if err != nil {
		t.Fatalf("lineup: %v", err)
	}
	return s, id, lid
}

func TestFinalizeMatch_MeSourceRules(t *testing.T) {
	ctx := context.Background()
	s, id, _ := seedTempMatch(t, "me")
	if err := s.FinalizeMatch(ctx, id); err != nil {
		t.Fatalf("me match with 1 lineup: %v", err)
	}
	// Pinned: re-finalizing a stamped match is an idempotent nil, the write
	// guard (ErrFinalized) fires on lineup writes, not on the marker itself.
	if err := s.FinalizeMatch(ctx, id); err != nil {
		t.Fatalf("double finalize err = %v, want nil", err)
	}
}

func TestFinalizeMatch_MeSourceWrongCount(t *testing.T) {
	ctx := context.Background()
	s, id, _ := seedTempMatch(t, "me")
	if _, err := s.AddLineup(ctx, domain.AddLineupCmd{MatchID: id, Label: "two", Placement: 2}); err != nil {
		t.Fatalf("second lineup: %v", err)
	}
	if err := s.FinalizeMatch(ctx, id); !errors.Is(err, domain.ErrMeFinalize) {
		t.Fatalf("me 2-lineup finalize err = %v, want ErrMeFinalize", err)
	}
}

func TestFinalizeMatch_UnknownMatch(t *testing.T) {
	s := seedTemp(t)
	if err := s.FinalizeMatch(context.Background(), 424242); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
