package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/service"
)

func TestEntityPlural_Table(t *testing.T) {
	cases := map[string]string{
		"hero":   "heroes",
		"race":   "races",
		"class":  "classes",
		"item":   "items",
		"relic":  "relics",
		"patch":  "patches",
		"pro":    "pros",
		"synerg": "synerg", // unknown entity falls back to itself
	}
	for in, want := range cases {
		if got := entityPlural(in); got != want {
			t.Errorf("entityPlural(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAvgText(t *testing.T) {
	if got := avgText(0); got != "" {
		t.Errorf("avgText(0) = %q, want blank for unplayed", got)
	}
	if got := avgText(3.7); got != "3.7" {
		t.Errorf("avgText(3.7) = %q, want 3.7", got)
	}
}

func TestSigned(t *testing.T) {
	cases := map[float64]string{
		2.5:  "+2.5",
		0:    "0.0",
		-1.5: "-1.5",
	}
	for in, want := range cases {
		if got := signed(in); got != want {
			t.Errorf("signed(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestPlayedText(t *testing.T) {
	if got := playedText(0); got != "" {
		t.Errorf("playedText(0) = %q, want blank for never played", got)
	}
	const ts = 1735689600
	want := time.Unix(ts, 0).Format("2006-01-02 15:04")
	if got := playedText(ts); got != want || len(got) != 16 {
		t.Errorf("playedText(%d) = %q, want %q at length 16", ts, got, want)
	}
}

func TestPeakLabel(t *testing.T) {
	view := service.EditorView{Pros: []domain.Pro{{ID: 7, Name: "seth", PeakRank: "gm"}}}
	if got := peakLabel(view, domain.Lineup{}); got != "" {
		t.Errorf("no pro credit: peakLabel = %q, want blank", got)
	}
	id := int64(7)
	if got := peakLabel(view, domain.Lineup{ProID: &id}); got != "peak: gm" {
		t.Errorf("credited pro: peakLabel = %q, want \"peak: gm\"", got)
	}
	missing := int64(9)
	if got := peakLabel(view, domain.Lineup{ProID: &missing}); got != "" {
		t.Errorf("pro not in view: peakLabel = %q, want blank", got)
	}
}

func TestSpreadWidth(t *testing.T) {
	if got := spreadWidth([8]int{}, 3); got != "0%" {
		t.Errorf("no finishes: spreadWidth = %q, want 0%%", got)
	}
	shares := [8]int{3, 0, 1, 0, 2, 0, 0, 0}
	if got := spreadWidth(shares, 0); got != "50.00%" {
		t.Errorf("spreadWidth 3/6 = %q, want 50.00%%", got)
	}
	if got := spreadWidth(shares, 2); got != "16.67%" {
		t.Errorf("spreadWidth 1/6 = %q, want 16.67%%", got)
	}
}

func TestStatic_ServesEmbeddedFiles(t *testing.T) {
	h := Static()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app.css: status %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("GET /app.css: empty body, want embedded stylesheet bytes")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope.css", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope.css: status %d, want 404", rec.Code)
	}
}

func TestButton_RendersKindAndLabel(t *testing.T) {
	got := renderComp(t, Button("primary", "save"))
	for _, want := range []string{"button button-primary", ">save<"} {
		if !strings.Contains(got, want) {
			t.Errorf("button html missing %q\n got: %q", want, got)
		}
	}
}
