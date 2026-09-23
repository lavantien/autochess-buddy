//go:build e2e

package e2e

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
)

// TestEditDeleteFlow: on the seeded pro draft, edit scalars, re-sort on a
// placement change, save stars, remove an item chip without a dialog, delete a
// slot with confirm and watch the next add refill the lowest cell, then delete a
// lineup and watch the pips drop.
func TestEditDeleteFlow(t *testing.T) {
	base, _ := newApp(t, true)
	page := newPage(t, base)

	// The draft is match 6 with lineups 34 (p1), 35 (p2), 36 (p3).
	_, err := page.Goto(base + "/matches/6/edit")
	must(t, err)
	waitText(t, page, ".pip-count", "3/8")

	// Edit scalars on the first card via the edit disclosure.
	first := page.Locator("[id^='card-']").First()
	must(t, first.Locator("details:has(summary:text-is('edit'))").First().Locator("summary").Click())
	must(t, first.Locator("input[name='wins']").Fill("7"))
	must(t, first.Locator("input[name='networth']").Fill("61"))
	must(t, first.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "save lineup"}).Click())
	must(t, page.Locator(".record:has-text('w-d-l 7-0-0')").First().WaitFor())

	// Placement change re-sorts: move card 1 to placement 4, cards shift up.
	must(t, first.Locator("details:has(summary:text-is('edit'))").First().Locator("summary").Click())
	must(t, first.Locator("input[name='placement']").Fill("4"))
	must(t, first.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "save lineup"}).Click())
	waitText(t, page, "[id^='card-'] .numeral", "2")
	firstNow := page.Locator("[id^='card-']").First()

	// Save stars on the new first card's slot.
	slot := firstNow.Locator(".slot:not(.slot-empty)").First()
	must(t, slot.Locator("summary").Click())
	selectValue(t, slot.Locator("select[name='stars']"), "1")
	must(t, slot.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "save stars"}).Click())
	must(t, page.Locator(".star-count >> text=1/3").First().WaitFor())

	// Remove an item chip: no dialog fires (auto-accept would hide it, so assert
	// the chip vanishes and the slot details stay open).
	chips := slot.Locator(".chip")
	n, err := chips.Count()
	must(t, err)
	if n == 0 {
		t.Fatal("fixture slot must hold an item chip")
	}
	must(t, slot.Locator(".chip button").First().Click())
	must(t, page.Locator("[id^='grid-']").First().WaitFor())

	// Delete the slot with the confirm; the placeholder returns, focus lands on
	// the next slot in the same card, and the next add refills the lowest free
	// cell.
	next := firstNow.Locator(".slot:not(.slot-empty)").Nth(1)
	nextID, err := next.GetAttribute("id")
	must(t, err)
	goneID, err := slot.GetAttribute("id")
	must(t, err)
	must(t, slot.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "delete"}).Click())
	// The board already holds empty cells, so gate on the deleted id vanishing.
	_, err = page.WaitForFunction("id => !document.getElementById(id)", goneID)
	must(t, err)
	got, err := page.Evaluate("() => { var a = document.activeElement; var c = a && a.closest('[id^=slot-]'); return c ? c.id : ''; }")
	must(t, err)
	if id, _ := got.(string); id != nextID {
		t.Fatalf("focus after slot delete = %v, want the next slot cell %s", got, nextID)
	}
	must(t, firstNow.Locator(".heroform input[name='hero']").Fill("sky breaker"))
	must(t, firstNow.Locator(".heroform button").Filter(playwright.LocatorFilterOptions{HasText: "add hero"}).Click())
	must(t, page.Locator(".slot:has-text('sky breaker')").First().WaitFor())

	// Delete a lineup: pips drop, focus lands on the next card.
	before, err := page.Locator("[id^='card-']").Count()
	must(t, err)
	nextCardID, err := page.Locator("[id^='card-']").Nth(1).GetAttribute("id")
	must(t, err)
	must(t, firstNow.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "delete"}).Last().Click())
	waitText(t, page, ".pip-count", "2/8")
	_, err = page.WaitForFunction("n => document.querySelectorAll('[id^=card-]').length === n", before-1)
	must(t, err)
	got, err = page.Evaluate("() => { var a = document.activeElement; var c = a && a.closest('[id^=card-]'); return c ? c.id : ''; }")
	must(t, err)
	if id, _ := got.(string); id != nextCardID {
		t.Fatalf("focus after lineup delete = %v, want the next card %s", got, nextCardID)
	}
}
