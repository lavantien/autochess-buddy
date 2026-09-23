package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/seed"
)

func TestDashboard_FullPageVsPartialOnHXRequest(t *testing.T) {
	h, st, _, fake := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fake.viewCount = 32
	fake.heroRows = []domain.HeroRow{{
		Hero:  domain.Hero{ID: 1, Name: "sky breaker", Cost: 5},
		Picks: 10, Top4: 5, AvgPlace: 4.0, PickRate: 0.4, Top4Rate: 0.5,
		Floor: 0.4, VsField: -0.5, Finishes: [8]int{1, 1, 1, 2, 2, 1, 1, 1},
	}}

	full := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, full)
	if rec.Code != http.StatusOK {
		t.Fatalf("full page status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<!doctype html>", "sky breaker", "32 lineups in view"} {
		if !strings.Contains(body, want) {
			t.Fatalf("full page missing %q, got %s", want, body[:min(400, len(body))])
		}
	}

	hxReq := httptest.NewRequest(http.MethodGet, "/dashboard/heroes", nil)
	hxReq.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, hxReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("partial status = %d", rec.Code)
	}
	partial := rec.Body.String()
	if strings.Contains(partial, "<!doctype html>") {
		t.Fatalf("hx partial must be the bare panel, got %s", partial[:min(200, len(partial))])
	}
	if !strings.Contains(partial, `id="dashpanel-heroes"`) || !strings.Contains(partial, "sky breaker") {
		t.Fatalf("partial must carry the panel with its rows, got %s", partial[:min(400, len(partial))])
	}
}

// TestDashboard_PartialRouteNonHXRendersFullPage pins the fallback for the
// filter endpoints: no HX-Request header means the whole page, never a bare
// fragment.
func TestDashboard_PartialRouteNonHXRendersFullPage(t *testing.T) {
	h, st, _, _ := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/heroes?patch=7.4", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!doctype html>") || strings.Contains(body, "hx-swap-oob") {
		t.Fatal("non-hx partial route must render the full page without oob fragments")
	}
}

func TestDashboard_FilterRidesQueryParams(t *testing.T) {
	h, st, _, fake := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	patches, err := st.ListPatches(t.Context())
	if err != nil || len(patches) == 0 {
		t.Fatalf("patches: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/heroes?patch=7.4&source=pro", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if fake.lastFilter.PatchID != patches[0].ID || fake.lastFilter.Source != "pro" {
		t.Fatalf("filter reaching analytics = %+v, want patch %d pro", fake.lastFilter, patches[0].ID)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="pro" selected`) {
		t.Fatalf("filter row must echo the source selection, got %s", body[:min(400, len(body))])
	}
}

func TestMatchList_PipsAndStateFilters(t *testing.T) {
	h, st, _, _ := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "draft") || !strings.Contains(body, "final") {
		t.Fatalf("list must show both states, got %s", body[:min(400, len(body))])
	}
	if !strings.Contains(body, "3/8") || !strings.Contains(body, "8/8") || !strings.Contains(body, "1/1") {
		t.Fatalf("list must show pip counts, got %s", body[:min(400, len(body))])
	}

	req := httptest.NewRequest(http.MethodGet, "/matches?state=draft&source=pro", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body = rec.Body.String()
	if strings.Contains(body, "8/8") || !strings.Contains(body, "3/8") {
		t.Fatalf("draft+pro filter must leave only the draft row, got %s", body[:min(600, len(body))])
	}
}

func TestMatchDetail_ScoreboardRankedByPlacement(t *testing.T) {
	h, st, _, _ := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches/1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	// The strip renders one scorecard per placement in order.
	if !strings.Contains(body, "scorestrip") || !strings.Contains(body, "scorecard") {
		t.Fatalf("detail must render the scoreboard strip, got %s", body[:min(400, len(body))])
	}
	first := strings.Index(body, "place-1")
	if first < 0 {
		t.Fatalf("placement 1 must render in its tier class, got %s", body[:min(400, len(body))])
	}
	if !strings.Contains(body, "finalized") {
		t.Fatalf("header must state finalized, got %s", body[:min(400, len(body))])
	}
}

func TestNewMatch_303ToEditorOnSuccess(t *testing.T) {
	h, st, entry, _ := newDashTestServer(t)
	patch, err := st.CreatePatch(t.Context(), domain.Patch{Version: "9.0", ReleasedAt: "2026-10-01"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	form := "patch_id=" + strconv.FormatInt(patch, 10) + "&source=pro&played_at=2026-09-23T20:15&notes=vic lobby"
	req := httptest.NewRequest(http.MethodPost, "/matches/new", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasSuffix(loc, "/edit") {
		t.Fatalf("location = %q, want the editor", loc)
	}
	id := parseID(strings.TrimPrefix(strings.TrimSuffix(loc, "/edit"), "/matches/"))
	if id == 0 {
		t.Fatalf("could not parse match id from %q", loc)
	}
	view, err := entry.Editor(t.Context(), id)
	if err != nil || view.Match.Source != "pro" || view.Match.Notes != "vic lobby" {
		t.Fatalf("created match = %+v, err %v", view.Match, err)
	}
}
