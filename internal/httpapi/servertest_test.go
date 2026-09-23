package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

// quietLog keeps test output pristine.
func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestServer wires a real store and entry service over a temp db with a fake
// analytics backend.
func newTestServer(t *testing.T) (http.Handler, *sqlite.Store, service.EntryService) {
	h, st, entry, _ := newDashTestServer(t)
	return h, st, entry
}

// newDashTestServer also hands back the fake analytics for dashboard assertions.
func newDashTestServer(t *testing.T) (http.Handler, *sqlite.Store, service.EntryService, *fakeAnalytics) {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	entry := service.EntryService{St: st}
	fake := &fakeAnalytics{}
	return New(quietLog(), st, entry, fake), st, entry, fake
}

// editorFixture seeds one pro match with 1 lineup holding 1 hero and returns the
// pieces the handler tests poke at.
type editorFixture struct {
	h        http.Handler
	st       *sqlite.Store
	entry    service.EntryService
	matchID  int64
	lineupID int64
	slotID   int64
	itemID   int64
	relicID  int64
	heroName string
}

func seedEditor(t *testing.T) editorFixture {
	t.Helper()
	h, st, entry := newTestServer(t)
	ctx := context.Background()
	patch, err := st.CreatePatch(ctx, domain.Patch{Version: "8.0", ReleasedAt: "2026-09-01"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	race, err := st.CreateRace(ctx, domain.Race{Name: "beast"}, nil)
	if err != nil {
		t.Fatalf("race: %v", err)
	}
	class, err := st.CreateClass(ctx, domain.Class{Name: "knight"}, nil)
	if err != nil {
		t.Fatalf("class: %v", err)
	}
	hero, err := st.CreateHero(ctx, domain.Hero{Name: "grim jaw", Cost: 2, Races: []domain.Race{{ID: race}}, Classes: []domain.Class{{ID: class}}})
	if err != nil {
		t.Fatalf("hero: %v", err)
	}
	itemID, err := st.CreateItem(ctx, domain.Item{Name: "storm core", Tier: 3}, nil)
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	relicID, err := st.CreateRelic(ctx, domain.Relic{Name: "tide bell", Effect: "heal"})
	if err != nil {
		t.Fatalf("relic: %v", err)
	}
	matchID, err := entry.CreateMatch(ctx, domain.Match{PatchID: patch, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	view, _, err := entry.AddLineup(ctx, matchID, domain.AddLineupCmd{
		Label: "first", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: hero}, Stars: 2, Items: []domain.Item{{ID: itemID}}}},
	}, 0)
	if err != nil {
		t.Fatalf("lineup: %v", err)
	}
	return editorFixture{h: h, st: st, entry: entry, matchID: matchID,
		lineupID: view.Lineups[0].ID, slotID: view.Lineups[0].Slots[0].ID,
		itemID: itemID, relicID: relicID, heroName: "grim jaw"}
}

func (f editorFixture) post(t *testing.T, method, path, form string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

// fakeAnalytics stands in for the duckdb engine in handler tests; its fields
// feed the dashboard panel assertions.
type fakeAnalytics struct {
	heroRows    []domain.HeroRow
	placeRows   []domain.PlaceRow
	viewCount   int
	lastFilter  domain.Filter
	filterCalls int
}

func (f *fakeAnalytics) HeroPerformance(ctx context.Context, fl domain.Filter) ([]domain.HeroRow, error) {
	f.lastFilter = fl
	f.filterCalls++
	return f.heroRows, nil
}

func (f *fakeAnalytics) SynergyPerformance(ctx context.Context, fl domain.Filter) ([]domain.SynergyRow, error) {
	f.lastFilter = fl
	return nil, nil
}

func (f *fakeAnalytics) ItemPerformance(ctx context.Context, fl domain.Filter) ([]domain.ItemRow, error) {
	f.lastFilter = fl
	return nil, nil
}

func (f *fakeAnalytics) RelicPerformance(ctx context.Context, fl domain.Filter) ([]domain.RelicRow, error) {
	f.lastFilter = fl
	return nil, nil
}

func (f *fakeAnalytics) NetworthByPlacement(ctx context.Context, fl domain.Filter) ([]domain.PlaceRow, error) {
	f.lastFilter = fl
	return f.placeRows, nil
}

func (f *fakeAnalytics) LineupsInView(ctx context.Context, fl domain.Filter) (int, error) {
	f.lastFilter = fl
	return f.viewCount, nil
}
