package sqlite

import (
	"context"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

func TestListMatches_RejectsUnknownStateFilter(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)
	if _, err := s.ListMatches(ctx, domain.MatchFilter{State: "bogus"}); err == nil ||
		!strings.Contains(err.Error(), "state filter must be draft or final") {
		t.Fatalf("unknown state err = %v, want state filter guard", err)
	}
}

func TestGetMatch_FreshShellHasNoChildren(t *testing.T) {
	ctx := context.Background()
	s := seedTemp(t)
	id, err := s.CreateMatchShell(ctx, domain.Match{PatchID: 2, PlayedAt: 1000, Source: "me"})
	if err != nil {
		t.Fatalf("create shell: %v", err)
	}
	m, lineups, err := s.GetMatch(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m.ID != id || m.FinalizedAt != 0 {
		t.Fatalf("shell match = %+v, want id %d unfinalized", m, id)
	}
	if len(lineups) != 0 {
		t.Fatalf("fresh shell lineups = %+v, want none", lineups)
	}
}

func TestReadsOnClosedStoreFail(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := s.ListMatches(ctx, domain.MatchFilter{}); err == nil {
		t.Fatal("ListMatches on closed store must fail")
	}
	if _, _, err := s.GetMatch(ctx, 1); err == nil {
		t.Fatal("GetMatch on closed store must fail")
	}
}
