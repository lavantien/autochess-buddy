package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func quietServer() http.Handler {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestRoutes_RegisterAllSpecPaths(t *testing.T) {
	h := quietServer()
	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"landing redirects to dashboard", "GET", "/", http.StatusSeeOther},
		{"dashboard overview", "GET", "/dashboard", http.StatusOK},
		{"dashboard hero partial", "GET", "/dashboard/heroes", http.StatusOK},
		{"dashboard synergy partial", "GET", "/dashboard/synergies", http.StatusOK},
		{"dashboard item partial", "GET", "/dashboard/items", http.StatusOK},
		{"dashboard relic partial", "GET", "/dashboard/relics", http.StatusOK},
		{"matches list", "GET", "/matches", http.StatusOK},
		{"match create form", "GET", "/matches/new", http.StatusOK},
		{"match create submit", "POST", "/matches/new", http.StatusSeeOther},
		{"match detail", "GET", "/matches/14", http.StatusOK},
		{"lineup editor", "GET", "/matches/14/edit", http.StatusOK},
		{"add lineup card", "POST", "/matches/14/lineups", http.StatusSeeOther},
		{"finalize match", "POST", "/matches/14/finalize", http.StatusSeeOther},
		{"add hero slot", "POST", "/lineups/9/slots", http.StatusSeeOther},
		{"attach item", "POST", "/slots/3/items", http.StatusSeeOther},
		{"attach relic", "POST", "/lineups/9/relics", http.StatusSeeOther},
		{"edit lineup scalars", "POST", "/lineups/9", http.StatusSeeOther},
		{"edit slot stars", "POST", "/slots/3", http.StatusSeeOther},
		{"remove item", "DELETE", "/slots/3/items/5", http.StatusSeeOther},
		{"remove relic", "DELETE", "/lineups/9/relics/2", http.StatusSeeOther},
		{"remove slot", "DELETE", "/slots/3", http.StatusSeeOther},
		{"remove lineup", "DELETE", "/lineups/9", http.StatusSeeOther},
	}
	for _, e := range []string{"heroes", "races", "classes", "items", "relics", "patches", "pros"} {
		cases = append(cases,
			struct {
				name, method, path string
				want               int
			}{"codex list " + e, "GET", "/" + e, http.StatusOK},
			struct {
				name, method, path string
				want               int
			}{"codex create " + e, "POST", "/" + e, http.StatusSeeOther},
			struct {
				name, method, path string
				want               int
			}{"codex detail " + e, "GET", "/" + e + "/1", http.StatusOK},
			struct {
				name, method, path string
				want               int
			}{"codex update " + e, "POST", "/" + e + "/1", http.StatusSeeOther},
			struct {
				name, method, path string
				want               int
			}{"codex delete " + e, "DELETE", "/" + e + "/1", http.StatusSeeOther},
		)
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		if c.method != http.MethodGet {
			req.Header.Set("Origin", "http://example.com")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Fatalf("%s %s (%s): status = %d, want %d", c.method, c.path, c.name, rec.Code, c.want)
		}
		if c.path == "/" && rec.Header().Get("Location") != "/dashboard" {
			t.Fatalf("landing redirect location = %q, want /dashboard", rec.Header().Get("Location"))
		}
		if c.method == http.MethodGet && c.want == http.StatusOK {
			body := rec.Body.String()
			if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "autochess companion") {
				t.Fatalf("%s: non-hx get must render a full page, got %q", c.path, body)
			}
		}
	}
}

func TestRoutes_RenderSpecEmptyStates(t *testing.T) {
	h := quietServer()
	cases := []struct {
		path     string
		contains string
	}{
		{"/dashboard", "no finalized matches for this filter yet. finalize a few matches first."},
		{"/heroes", "no heroes yet. add the first hero so lineups can reference it."},
		{"/races", "no races yet. add the first race so lineups can reference it."},
		{"/matches", "no matches yet. start one from the game you just finished."},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), c.contains) {
			t.Fatalf("%s: body %q must contain %q", c.path, rec.Body.String(), c.contains)
		}
	}
}

func TestRoutes_HxMutationsReturnPlainStatus(t *testing.T) {
	h := quietServer()
	cases := []struct {
		method, path string
	}{
		{"POST", "/matches/14/lineups"},
		{"POST", "/slots/3/items"},
		{"DELETE", "/heroes/1"},
		{"DELETE", "/slots/3"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		req.Header.Set("HX-Request", "true")
		req.Header.Set("Origin", "http://example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s with HX-Request: status = %d, want 200", c.method, c.path, rec.Code)
		}
	}
}
