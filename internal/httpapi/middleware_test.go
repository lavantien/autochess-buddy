package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okNext() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestOriginGuard_BlocksForeignOriginPost(t *testing.T) {
	guard := OriginGuard(okNext())

	req := httptest.NewRequest(http.MethodPost, "/matches/new", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign origin: status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "evil.example") {
		t.Fatalf("foreign origin: body %q must name the cause", rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/matches/new", nil)
	rec2 := httptest.NewRecorder()
	guard.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("no origin headers: status = %d, want 403", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "without origin") {
		t.Fatalf("no origin headers: body %q must name the cause", rec2.Body.String())
	}
}

func TestOriginGuard_AllowsSameOriginAndReferer(t *testing.T) {
	guard := OriginGuard(okNext())

	req := httptest.NewRequest(http.MethodPost, "/matches/new", nil)
	req.Host = "localhost:8077"
	req.Header.Set("Origin", "http://localhost:8077")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("same origin: status = %d, want 200", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/matches/new", nil)
	req2.Host = "localhost:8077"
	req2.Header.Set("Referer", "http://localhost:8077/matches")
	rec2 := httptest.NewRecorder()
	guard.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("same referer: status = %d, want 200", rec2.Code)
	}
}

func TestOriginGuard_SkipsGet(t *testing.T) {
	guard := OriginGuard(okNext())
	req := httptest.NewRequest(http.MethodGet, "/matches", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get with foreign origin: status = %d, want 200", rec.Code)
	}
}

func TestHeaderHost_UnparseableOrHostlessOrigin(t *testing.T) {
	for _, origin := range []string{"://bad", "not-a-url"} {
		req := httptest.NewRequest(http.MethodPost, "/matches/new", nil)
		req.Header.Set("Origin", origin)
		if host, ok := headerHost(req); ok || host != "" {
			t.Fatalf("origin %q: headerHost = (%q, %v), want empty false", origin, host, ok)
		}
	}
}
