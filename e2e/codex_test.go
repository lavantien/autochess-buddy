//go:build e2e

package e2e

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
)

// TestCodexHeroFlow: create a race with tiers, create a hero, see it in the
// table, edit its cost, then delete with the hx-confirm + HX-Redirect dance.
func TestCodexHeroFlow(t *testing.T) {
	base, _ := newApp(t, false)
	page := newPage(t, base)

	openDetails := func(detailsSel string) {
		t.Helper()
		waitFor(t, page, detailsSel)
		must(t, page.Locator(detailsSel+" summary").Click())
	}

	_, err := page.Goto(base + "/races")
	must(t, err)
	openDetails("details:has(summary:text-is('+ new race'))")
	must(t, page.Locator("details:has(summary:text-is('+ new race')) input[name='name']").Fill("human"))
	must(t, page.Locator("details:has(summary:text-is('+ new race')) button").Click())
	must(t, page.WaitForURL("**/races"))

	openDetails("details:has(summary:text-is('+ new class'))")
	must(t, page.Locator("details:has(summary:text-is('+ new class')) input[name='name']").Fill("warrior"))
	must(t, page.Locator("details:has(summary:text-is('+ new class')) button").Click())
	must(t, page.WaitForURL("**/classes"))

	// Give human a tier ladder.
	ladder := page.Locator(".ladder", playwright.PageLocatorOptions{HasText: playwright.String("human")})
	waitFor(t, page, ".ladder")
	must(t, ladder.Locator("input[name='count']").First().Fill("2"))
	must(t, ladder.Locator("input[name='effect']").First().Fill("all humans +10% atk"))
	must(t, ladder.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "save tier"}).First().Click())
	must(t, page.Locator("text=all humans +10% atk").First().WaitFor())

	_, err = page.Goto(base + "/heroes")
	must(t, err)
	openDetails("details:has(summary:text-is('+ new hero'))")
	must(t, page.Locator("details:has(summary:text-is('+ new hero')) input[name='name']").Fill("grim jaw"))
	must(t, page.Locator("details:has(summary:text-is('+ new hero')) input[name='cost']").Fill("4"))
	selectLabel(t, page.Locator("details:has(summary:text-is('+ new hero')) select[name='race1']"), "human")
	selectLabel(t, page.Locator("details:has(summary:text-is('+ new hero')) select[name='class1']"), "warrior")
	must(t, page.Locator("details:has(summary:text-is('+ new hero')) button").Filter(playwright.LocatorFilterOptions{HasText: "save hero"}).Click())
	must(t, page.WaitForURL("**/heroes"))
	must(t, page.Locator("table a:has-text('grim jaw')").WaitFor())

	// Edit the cost on the edit page.
	must(t, page.Locator("table a:has-text('grim jaw')").Click())
	must(t, page.Locator("input[name='cost']").Fill("5"))
	must(t, page.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "save hero"}).Click())
	must(t, page.WaitForURL("**/heroes"))
	must(t, page.Locator("table a:has-text('grim jaw')").WaitFor())

	// Delete: hx-confirm auto-accepted, HX-Redirect lands on the index without the row.
	must(t, page.Locator("table a:has-text('grim jaw')").Click())
	must(t, page.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "delete hero"}).Click())
	must(t, page.WaitForURL("**/heroes"))
	gone(t, page.Locator("table a:has-text('grim jaw')"))
}

// TestCodexDeleteConflictShowsInBrowser: an in-use item's delete must surface
// the 409 conflict page in the browser, not just over plain http. htmx swaps
// the response into the page so the banner names the conflict.
func TestCodexDeleteConflictShowsInBrowser(t *testing.T) {
	base, st := newApp(t, true)
	page := newPage(t, base)

	var itemID int64
	if err := st.DB.QueryRow(`SELECT item_id FROM slot_items LIMIT 1`).Scan(&itemID); err != nil {
		t.Fatalf("find an in-use item: %v", err)
	}
	path := "/items/" + strconv.FormatInt(itemID, 10)
	_, err := page.Goto(base + path)
	must(t, err)
	waitFor(t, page, "form.inline-form")

	must(t, page.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "delete item"}).Click())
	waitText(t, page, ".banner", "existing matches keep their history.")
	if got := page.URL(); !strings.Contains(got, path) {
		t.Fatalf("conflict must stay on %s, navigated to %s", path, got)
	}
	// The row survives: the store refused the delete.
	if _, _, err := st.GetItem(context.Background(), itemID); err != nil {
		t.Fatalf("in-use item must survive the refused delete: %v", err)
	}
}
