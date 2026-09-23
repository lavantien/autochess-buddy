package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderComp(t *testing.T, c templ.Component) string {
	t.Helper()
	var sb strings.Builder
	if err := c.Render(context.Background(), &sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestPips_Golden(t *testing.T) {
	got := renderComp(t, Pips(3, 8))
	want := `<span class="pips">` +
		`<span class="pip pip-full"></span> ` +
		`<span class="pip pip-full"></span> ` +
		`<span class="pip pip-full"></span> ` +
		`<span class="pip"></span> ` +
		`<span class="pip"></span> ` +
		`<span class="pip"></span> ` +
		`<span class="pip"></span> ` +
		`<span class="pip"></span> ` +
		`<span class="pip-count">3/8</span></span>`
	if got != want {
		t.Fatalf("pips golden mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestSpreadBar_AriaCarriesShares(t *testing.T) {
	got := renderComp(t, SpreadBar([8]int{3, 0, 1, 0, 2, 0, 0, 0}))
	wantLabel := `aria-label="finishes by placement: 3, 0, 1, 0, 2, 0, 0, 0"`
	if !strings.Contains(got, wantLabel) {
		t.Fatalf("aria label must carry all 8 shares\n got: %q\nwant substring: %q", got, wantLabel)
	}
	if n := strings.Count(got, `<span class="spread-seg`); n != 3 {
		t.Fatalf("want 3 non-empty segments, got %d in %q", n, got)
	}
}
