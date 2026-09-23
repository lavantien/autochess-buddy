package httpapi

// Degraded-mode contract: when the store dies mid-flight or analytics fails,
// pages render their stub/empty-state answers and mutations fall back (303 for
// plain clients, bare 422 for hx) instead of panicking or 500ing.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestClosedStore_IndexRoutesRenderStubs(t *testing.T) {
	f := seedEditor(t)
	if err := f.st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	noMatches := "no matches yet. start one from the game you just finished."
	cases := []struct{ name, path, want string }{
		{"heroes", "/heroes", "heroes"},
		{"items", "/items", "items"},
		{"relics", "/relics", "relics"},
		{"patches", "/patches", "patches"},
		{"pros", "/pros", "pros"},
		{"races ladder", "/races", "synergies"},
		{"classes ladder", "/classes", "synergies"},
		{"matches", "/matches", noMatches},
		{"new match", "/matches/new", "no patches yet. add the first patch so lineups can reference it."},
		{"match detail", "/matches/999", noMatches},
		{"editor page", "/matches/" + strconv.FormatInt(f.matchID, 10) + "/edit", noMatches},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			f.h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 stub: %s", rec.Code, rec.Body.String()[:min(400, rec.Body.Len())])
			}
			body := rec.Body.String()
			if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, tc.want) {
				t.Fatalf("stub page missing %q: %s", tc.want, body[:min(400, len(body))])
			}
		})
	}
}

func TestClosedStore_MutationsFallBack(t *testing.T) {
	f := seedEditor(t)
	if err := f.st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	m := strconv.FormatInt(f.matchID, 10)
	l := strconv.FormatInt(f.lineupID, 10)
	sl := strconv.FormatInt(f.slotID, 10)
	it := strconv.FormatInt(f.itemID, 10)
	rl := strconv.FormatInt(f.relicID, 10)
	// createMatch has no row here: its store-error fallback sits behind a
	// validation mirror (the closed store yields an empty patch list, which
	// fails validation first), so the arm is unreachable through HTTP.
	mutations := []struct{ name, method, path, form string }{
		{"finalize", "POST", "/matches/" + m + "/finalize", ""},
		{"add lineup", "POST", "/matches/" + m + "/lineups", "match_id=" + m + "&placement=3&label=x"},
		{"update lineup", "POST", "/lineups/" + l, "match_id=" + m + "&placement=2&label=x"},
		{"add slot", "POST", "/lineups/" + l + "/slots", "match_id=" + m + "&hero=" + f.heroName + "&stars=2"},
		{"save stars", "POST", "/slots/" + sl, "match_id=" + m + "&stars=2"},
		{"attach item", "POST", "/slots/" + sl + "/items", "match_id=" + m + "&item=" + it},
		{"remove item", "DELETE", "/slots/" + sl + "/items/" + it + "?match_id=" + m, ""},
		{"add relic", "POST", "/lineups/" + l + "/relics", "match_id=" + m + "&relic=" + rl},
		{"remove relic", "DELETE", "/lineups/" + l + "/relics/" + rl + "?match_id=" + m, ""},
		{"delete slot", "DELETE", "/slots/" + sl + "?match_id=" + m, ""},
		{"delete lineup", "DELETE", "/lineups/" + l + "?match_id=" + m, ""},
	}
	for _, tc := range mutations {
		t.Run(tc.name+"/non-hx", func(t *testing.T) {
			rec := f.post(t, tc.method, tc.path, tc.form, false)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303 fallback: %s", rec.Code, rec.Body.String()[:min(400, rec.Body.Len())])
			}
			if loc := rec.Header().Get("Location"); loc != "/matches" {
				t.Fatalf("location = %q, want /matches", loc)
			}
		})
		t.Run(tc.name+"/hx", func(t *testing.T) {
			rec := f.post(t, tc.method, tc.path, tc.form, true)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want bare 422", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("body = %q, want empty", rec.Body.String())
			}
		})
	}
}

func TestClosedStore_SynergyCreateFallsBackTo422(t *testing.T) {
	f := seedEditor(t)
	if err := f.st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	for _, path := range []string{"/races", "/classes"} {
		t.Run(path, func(t *testing.T) {
			// synergy422's mutation fallback: hx gets a bare 422, no fragments.
			rec := f.post(t, "POST", path, "name=ember", true)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String()[:min(400, rec.Body.Len())])
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("body = %q, want bare 422", rec.Body.String())
			}
		})
	}
}

func TestClosedStore_SynergyCreateNonHXRedirects(t *testing.T) {
	// Pinned quirk: the non-hx synergy fallback rides the generic
	// mutationFallback, so a plain client lands on /matches, not /races.
	f := seedEditor(t)
	if err := f.st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	rec := f.post(t, "POST", "/races", "name=ember", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/matches" {
		t.Fatalf("status = %d location = %q, want 303 /matches", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAddLineup_NonHXInvalidRedirectsToEditor(t *testing.T) {
	f := seedEditor(t)
	m := strconv.FormatInt(f.matchID, 10)
	l := strconv.FormatInt(f.lineupID, 10)
	for _, tc := range []struct{ name, path, form string }{
		{"add lineup", "/matches/" + m + "/lineups", "match_id=" + m + "&placement=0&label=x"},
		{"update lineup", "/lineups/" + l, "match_id=" + m + "&placement=0&label=x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.post(t, "POST", tc.path, tc.form, false)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303 to editor: %s", rec.Code, rec.Body.String()[:min(400, rec.Body.Len())])
			}
			if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/edit") {
				t.Fatalf("location = %q, want editor", loc)
			}
		})
	}
}

func TestDashboard_AnalyticsErrorsRenderStub(t *testing.T) {
	h, _, _, fake := newDashTestServer(t)
	fake.err = errors.New("duckdb down")
	const stub = "no finalized matches for this filter yet. finalize a few matches first."
	for _, path := range []string{"/dashboard", "/dashboard/heroes", "/dashboard/synergies", "/dashboard/items", "/dashboard/relics"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("HX-Request", "true")
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 stub: %s", rec.Code, rec.Body.String()[:min(400, rec.Body.Len())])
			}
			body := rec.Body.String()
			if !strings.Contains(body, stub) {
				t.Fatalf("stub copy missing: %s", body[:min(400, len(body))])
			}
		})
	}
}
