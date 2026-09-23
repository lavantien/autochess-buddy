//go:build e2e

package e2e

import (
	"context"
	"strconv"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/mxschmitt/playwright-go"
)

// seedEntryCodex gives the flow a patch, lineage, hero and item to type against.
func seedEntryCodex(t *testing.T, st interface {
	CreatePatch(context.Context, domain.Patch) (int64, error)
	CreateRace(context.Context, domain.Race, []domain.Tier) (int64, error)
	CreateClass(context.Context, domain.Class, []domain.Tier) (int64, error)
	CreateHero(context.Context, domain.Hero) (int64, error)
	CreateItem(context.Context, domain.Item, []int64) (int64, error)
}) {
	t.Helper()
	ctx := context.Background()
	if _, err := st.CreatePatch(ctx, domain.Patch{Version: "7.5", ReleasedAt: "2026-09-01"}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	race, err := st.CreateRace(ctx, domain.Race{Name: "warrior"}, nil)
	if err != nil {
		t.Fatalf("race: %v", err)
	}
	class, err := st.CreateClass(ctx, domain.Class{Name: "knight"}, nil)
	if err != nil {
		t.Fatalf("class: %v", err)
	}
	if _, err := st.CreateHero(ctx, domain.Hero{
		Name: "sky breaker", Cost: 5,
		Races:   []domain.Race{{ID: race}},
		Classes: []domain.Class{{ID: class}},
	}); err != nil {
		t.Fatalf("hero: %v", err)
	}
	if _, err := st.CreateItem(ctx, domain.Item{Name: "war horn", Tier: 2}, nil); err != nil {
		t.Fatalf("item: %v", err)
	}
}

// TestProMatchEntryFlow: create a pro match, add 8 lineups with the placement
// prefill advancing and pips filling, add heroes through the datalist row, add
// an item and save stars, use copy, hit the duplicate placement error with the
// value preserved, then finalize into the 8 card scoreboard.
func TestProMatchEntryFlow(t *testing.T) {
	base, st := newApp(t, false)
	page := newPage(t, base)
	seedEntryCodex(t, st)

	_, err := page.Goto(base + "/matches/new")
	must(t, err)
	selectValue(t, page.Locator("select[name='source']"), "pro")
	must(t, page.Locator("input[name='played_at']").Fill("2026-09-23T20:15"))
	must(t, page.Locator("input[name='notes']").Fill("vic lobby"))
	must(t, page.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "create match"}).Click())
	must(t, page.WaitForURL("**/edit"))

	addForm := page.Locator("[id^='addform-']")
	for i := 1; i <= 7; i++ {
		must(t, addForm.Locator("input[name='label']").Fill("board "+strconv.Itoa(i)))
		must(t, addForm.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "add lineup"}).Click())
		waitText(t, page, ".pip-count", strconv.Itoa(i)+"/8")
		// The reset addform lands on the cards swap; wait for its advanced
		// prefill before typing, or the late reset wipes the next entry.
		waitFor(t, page, "[id^='addform-'] input[name='placement'][value='"+strconv.Itoa(i+1)+"']")
	}

	// Board entry on the first card: datalist hero, stars, item.
	first := page.Locator("[id^='card-']").First()
	must(t, first.Locator(".heroform input[name='hero']").Fill("sky breaker"))
	selectValue(t, first.Locator(".heroform select[name='stars']"), "3")
	_, eerr := page.Evaluate(`() => {
		const f = document.querySelector('[id^=heroform-]');
		f.addEventListener('submit', () => {
			const d = new FormData(f);
			window.__sub = d.get('hero') + '|' + d.get('stars');
		});
		f.addEventListener('htmx:configRequest', e => {
			window.__cfg = JSON.stringify(e.detail.parameters);
		});
	}`)
	must(t, eerr)
	must(t, first.Locator(".heroform button").Filter(playwright.LocatorFilterOptions{HasText: "add hero"}).Click())
	must(t, first.Locator(".slot:has-text('sky breaker')").WaitFor())
	// The grid swap can land after the slot appears; anchor on an open details
	// so the select is visible, clicking again if the swap closed it.
	openSel := first.Locator(".slot:has-text('sky breaker') details[open] select[name='item']")
	for range 5 {
		must(t, first.Locator(".slot:has-text('sky breaker') summary").Click())
		page.WaitForTimeout(300)
		if n, err := openSel.Count(); err == nil && n > 0 {
			break
		}
	}
	selectLabel(t, openSel, "war horn")
	must(t, first.Locator(".slot:has-text('sky breaker') button").Filter(playwright.LocatorFilterOptions{HasText: "add item"}).Click())
	must(t, first.Locator(".chip:has-text('war horn')").WaitFor())
	selectValue(t, first.Locator(".slot:has-text('sky breaker') select[name='stars']"), "2")
	must(t, first.Locator(".slot:has-text('sky breaker') button").Filter(playwright.LocatorFilterOptions{HasText: "save stars"}).Click())
	waitText(t, page, ".slot:has-text('sky breaker') .star-count", "2/3")

	// Copy the first card: label kept, the copy fills placement 8, pips reach 8/8.
	must(t, first.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "copy"}).Click())
	must(t, page.Locator(".lcard:has-text('board 1')").Nth(1).WaitFor())
	waitText(t, page, ".pip-count", "8/8")

	// Duplicate placement: aim at the taken 1, expect spec copy and the label kept.
	must(t, addForm.Locator("input[name='placement']").Fill("1"))
	must(t, addForm.Locator("input[name='label']").Fill("clash"))
	must(t, addForm.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "add lineup"}).Click())
	must(t, page.Locator("text=placement 1 is already used by another lineup in this match.").WaitFor())
	v, err := addForm.Locator("input[name='label']").InputValue()
	must(t, err)
	if v != "clash" {
		t.Fatalf("label after 422 = %q, want clash preserved", v)
	}

	// Finalize: plain post lands on the detail page with the 8 card scoreboard.
	must(t, page.GetByRole("button").Filter(playwright.LocatorFilterOptions{HasText: "finalize match"}).Click())
	must(t, page.WaitForURL("**/matches/*"))
	must(t, page.Locator(".scorestrip").WaitFor())
	n, err := page.Locator(".scorecard").Count()
	must(t, err)
	if n != 8 {
		t.Fatalf("scoreboard cards = %d, want 8", n)
	}
}
