package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/analytics"
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
	t.Cleanup(func() { st.Close() })
	entry := service.EntryService{St: st}
	fake := &fakeAnalytics{}
	return New(quietLog(), st, entry, fake), st, entry, fake
}

// fakeAnalytics stands in for the duckdb engine in handler tests; its fields
// feed the dashboard panel assertions.
type fakeAnalytics struct {
	heroRows    []analytics.HeroRow
	placeRows   []analytics.PlaceRow
	viewCount   int
	lastFilter  analytics.Filter
	filterCalls int
}

func (f *fakeAnalytics) HeroPerformance(ctx context.Context, fl analytics.Filter) ([]analytics.HeroRow, error) {
	f.lastFilter = fl
	f.filterCalls++
	return f.heroRows, nil
}

func (f *fakeAnalytics) SynergyPerformance(ctx context.Context, fl analytics.Filter) ([]analytics.SynergyRow, error) {
	f.lastFilter = fl
	return nil, nil
}

func (f *fakeAnalytics) ItemPerformance(ctx context.Context, fl analytics.Filter) ([]analytics.ItemRow, error) {
	f.lastFilter = fl
	return nil, nil
}

func (f *fakeAnalytics) RelicPerformance(ctx context.Context, fl analytics.Filter) ([]analytics.RelicRow, error) {
	f.lastFilter = fl
	return nil, nil
}

func (f *fakeAnalytics) NetworthByPlacement(ctx context.Context, fl analytics.Filter) ([]analytics.PlaceRow, error) {
	f.lastFilter = fl
	return f.placeRows, nil
}

func (f *fakeAnalytics) LineupsInView(ctx context.Context, fl analytics.Filter) (int, error) {
	f.lastFilter = fl
	return f.viewCount, nil
}
