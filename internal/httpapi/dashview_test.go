package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/seed"
)

func TestDashboard_HXRootServesBarePanelAndCount(t *testing.T) {
	h, st, _, fake := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fake.viewCount = 7
	fake.heroRows = []domain.HeroRow{{
		Hero: domain.Hero{ID: 2, Name: "grim jaw", Cost: 4},
		Picks: 8, Top4: 4, AvgPlace: 4.5, PickRate: 0.25, Top4Rate: 0.5,
	}}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<!doctype html>") {
		t.Fatalf("hx swap must be the bare panel, got %s", body[:min(300, len(body))])
	}
	for _, want := range []string{`id="dashpanel-heroes"`, "grim jaw", "7 lineups in view"} {
		if !strings.Contains(body, want) {
			t.Fatalf("hx swap missing %q, got %s", want, body[:min(400, len(body))])
		}
	}
}

func TestDashboard_ItemsAndRelicsViewsRenderPanels(t *testing.T) {
	h, _, _, _ := newDashTestServer(t)
	for _, tc := range []struct{ path, panel string }{
		{"/dashboard/items", `id="dashpanel-items"`},
		{"/dashboard/relics", `id="dashpanel-relics"`},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<!doctype html>") {
			t.Fatalf("%s must render the full page, got %s", tc.path, body[:min(300, len(body))])
		}
		if !strings.Contains(body, tc.panel) {
			t.Fatalf("%s must render its panel %q, got %s", tc.path, tc.panel, body[:min(400, len(body))])
		}
	}
}

func TestMatchList_PatchParamFiltersToPatchMatches(t *testing.T) {
	h, st, _, _ := newDashTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches?patch=7.4", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "8/8") {
		t.Fatalf("patch 7.4 must keep its two finalized matches, got %s", body[:min(500, len(body))])
	}
	if strings.Contains(body, "3/8") || strings.Contains(body, "1/1") {
		t.Fatalf("patch 7.4 must hide patch 7.5 rows, got %s", body[:min(500, len(body))])
	}

	unknown := httptest.NewRequest(http.MethodGet, "/matches?patch=9.9", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, unknown)
	// Pinned quirk: an unmatched version leaves PatchID at 0, which the store
	// treats as no filter, so every match stays visible.
	ubody := rec.Body.String()
	if !strings.Contains(ubody, "3/8") || !strings.Contains(ubody, "1/1") {
		t.Fatalf("unknown patch version must fall back to the unfiltered list, got %s", ubody[:min(500, len(ubody))])
	}
}

func TestNewMatch_UnknownPatch422RerendersForm(t *testing.T) {
	fx := seedEditor(t)
	rec := fx.post(t, http.MethodPost, "/matches/new",
		"patch_id=999&source=pro&played_at=2026-09-23T20:15&notes=bogus patch", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Pinned gap: the patch_id field error is never printed because the form
	// template only renders st.err("played_at"). What is observable is the
	// rerender keeping exactly what was typed.
	for _, want := range []string{
		"<h1>new match</h1>",
		`value="pro" selected`,
		`value="2026-09-23T20:15"`,
		`value="bogus patch"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("422 rerender missing %q, got %s", want, body[:min(400, len(body))])
		}
	}
}

func TestMatchDetail_UnknownIDRendersEmptyState(t *testing.T) {
	h, _, _, _ := newDashTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches/999", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "no matches yet. start one from the game you just finished.") {
		t.Fatalf("missing match must render the empty-state page, got %s", body[:min(400, len(body))])
	}
}

func TestEditorPage_FinalizedMatchRedirectsToDetail(t *testing.T) {
	fx := seedEditor(t)
	ctx := context.Background()
	heroes, err := fx.st.ListHeroes(ctx)
	if err != nil || len(heroes) == 0 {
		t.Fatalf("heroes: %v", err)
	}
	for p := 2; p <= 8; p++ {
		_, _, err := fx.entry.AddLineup(ctx, fx.matchID, domain.AddLineupCmd{
			Label: "board " + strconv.Itoa(p), Placement: p,
			Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: heroes[0].ID}, Stars: 2}},
		}, 0)
		if err != nil {
			t.Fatalf("lineup %d: %v", p, err)
		}
	}
	rec := fx.post(t, http.MethodPost, "/matches/"+itoa(fx.matchID)+"/finalize", "", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/matches/"+itoa(fx.matchID) {
		t.Fatalf("finalize = %d %q, want 303 to the detail page", rec.Code, rec.Header().Get("Location"))
	}

	view, err := fx.entry.Editor(ctx, fx.matchID)
	if err != nil || view.Match.FinalizedAt == 0 {
		t.Fatalf("finalize persisted = %+v, err %v", view.Match, err)
	}

	rec = httptest.NewRecorder()
	fx.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches/"+itoa(fx.matchID)+"/edit", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/matches/"+itoa(fx.matchID) {
		t.Fatalf("edit location = %q, want the read-only detail page", loc)
	}
}

func TestFinalize_UnknownIDFallsBack(t *testing.T) {
	h, _, _, _ := newDashTestServer(t)
	fx := editorFixture{h: h}

	rec := fx.post(t, http.MethodPost, "/matches/999/finalize", "", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/matches" {
		t.Fatalf("plain finalize = %d %q, want 303 back to the list", rec.Code, rec.Header().Get("Location"))
	}

	hx := fx.post(t, http.MethodPost, "/matches/999/finalize", "", true)
	if hx.Code != http.StatusUnprocessableEntity {
		t.Fatalf("hx finalize status = %d, want 422", hx.Code)
	}
	if hx.Body.Len() != 0 {
		t.Fatalf("hx fallback must stay bare, got %s", hx.Body.String()[:min(200, hx.Body.Len())])
	}
}
