//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
)

// TestDashboardRendersNumbers: the lineups-in-view count and the first hero row
// match fixture goldens, the patch select triggers the hx-get partial swap and
// pushes the url, and reloading the pushed url renders the full page with the
// filter intact.
func TestDashboardRendersNumbers(t *testing.T) {
	base, _ := newApp(t, true)
	page := newPage(t, base)

	_, err := page.Goto(base + "/dashboard")
	must(t, err)
	must(t, page.Locator(".viewn").WaitFor())
	wantText(t, page.Locator(".viewn"), "33 lineups in view")

	// Fixture golden: sky breaker picks 9 over 33 lineups at 37/9, floor WilsonLB(5,9).
	row := page.Locator("table tbody tr").First()
	must(t, row.WaitFor())
	text, err := row.InnerText()
	must(t, err)
	for _, want := range []string{"sky breaker", "27.3%", "55.6%", "4.1", "-0.3"} {
		if !strings.Contains(text, want) {
			t.Fatalf("first hero row %q must contain %q", text, want)
		}
	}

	// The patch select swaps the panel and pushes the filter into the url.
	_, err = page.Locator("select[name='patch']").SelectOption(playwright.SelectOptionValues{Values: &[]string{"7.4"}})
	must(t, err)
	must(t, page.WaitForURL("**/dashboard/heroes?patch=7.4*"))
	must(t, page.Locator(".viewn").WaitFor())
	wantText(t, page.Locator(".viewn"), "16 lineups in view")

	// Reloading the pushed url renders the full page with the filter intact.
	_, err = page.Reload()
	must(t, err)
	must(t, page.Locator(".viewn").WaitFor())
	wantText(t, page.Locator(".viewn"), "16 lineups in view")
	sel, err := page.Evaluate("() => document.querySelector('select[name=patch]').value")
	must(t, err)
	if sel != "7.4" {
		t.Fatalf("patch select value after reload = %v, want 7.4", sel)
	}
}
