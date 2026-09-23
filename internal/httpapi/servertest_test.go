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
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	entry := service.EntryService{St: st}
	return New(quietLog(), st, entry, fakeAnalytics{}), st, entry
}

// fakeAnalytics stands in for the duckdb engine in handler tests.
type fakeAnalytics struct{}

func (fakeAnalytics) HeroPerformance(ctx context.Context, f analytics.Filter) ([]analytics.HeroRow, error) {
	return nil, nil
}

func (fakeAnalytics) SynergyPerformance(ctx context.Context, f analytics.Filter) ([]analytics.SynergyRow, error) {
	return nil, nil
}

func (fakeAnalytics) ItemPerformance(ctx context.Context, f analytics.Filter) ([]analytics.ItemRow, error) {
	return nil, nil
}

func (fakeAnalytics) RelicPerformance(ctx context.Context, f analytics.Filter) ([]analytics.RelicRow, error) {
	return nil, nil
}

func (fakeAnalytics) NetworthByPlacement(ctx context.Context, f analytics.Filter) ([]analytics.PlaceRow, error) {
	return nil, nil
}

func (fakeAnalytics) LineupsInView(ctx context.Context, f analytics.Filter) (int, error) {
	return 0, nil
}
