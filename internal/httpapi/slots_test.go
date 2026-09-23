package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// fixtureHeroID resolves the seeded hero's id for direct store writes.
func fixtureHeroID(t *testing.T, f editorFixture) int64 {
	t.Helper()
	h, err := f.st.GetHeroByName(context.Background(), f.heroName)
	if err != nil {
		t.Fatalf("hero: %v", err)
	}
	return h.ID
}

// assertMutationFallback pins the fallback contract: HX callers get a bare 422,
// never a fragment.
func assertMutationFallback(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("fallback body = %q, want empty", rec.Body.String())
	}
}

func assertHXRedirect(t *testing.T, rec *httptest.ResponseRecorder, code int, tail string) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d: %s", rec.Code, code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, tail) {
		t.Fatalf("location = %q, want suffix %q", loc, tail)
	}
}

func TestAddSlot_StarsOutOfRangeRendersCopy(t *testing.T) {
	f := seedEditor(t)
	l := strconv.FormatInt(f.lineupID, 10)
	m := strconv.FormatInt(f.matchID, 10)
	form := "match_id=" + m + "&hero=" + f.heroName + "&stars=9"
	rec := f.post(t, "POST", "/lineups/"+l+"/slots", form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if want := "stars must be between 1 and 3."; !strings.Contains(body, want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, body)
	}
	if !strings.Contains(body, `id="heroform-`) {
		t.Fatalf("422 must rerender the hero form, got %s", body)
	}
	view, err := f.entry.Editor(context.Background(), f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if len(view.Lineups[0].Slots) != 1 {
		t.Fatalf("bad stars must not add a slot, got %d", len(view.Lineups[0].Slots))
	}
}

func TestAddSlot_StarsOutOfRangeNonHXRedirects(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&hero=" + f.heroName + "&stars=9"
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/slots", form, false)
	assertHXRedirect(t, rec, http.StatusSeeOther, "/edit")
}

func TestAddSlot_BoardCapRejectsThirteenthHero(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	heroID := fixtureHeroID(t, f)
	slots := make([]domain.Slot, 11)
	for i := range slots {
		slots[i] = domain.Slot{SlotIndex: i, Hero: domain.Hero{ID: heroID}, Stars: 2}
	}
	_, lineupID, err := f.entry.AddLineup(ctx, f.matchID, domain.AddLineupCmd{
		Label: "full", Placement: 2, Slots: slots,
	}, 0)
	if err != nil {
		t.Fatalf("fill lineup: %v", err)
	}
	if _, err := f.st.AddSlot(ctx, lineupID, heroID, 2); err != nil {
		t.Fatalf("twelfth slot: %v", err)
	}
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&hero=" + f.heroName + "&stars=2"
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(lineupID, 10)+"/slots", form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if want := "a lineup holds at most 12 heroes."; !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body must carry cap copy %q, got %s", want, rec.Body.String())
	}
}

func TestSaveStars_StarsOutOfRangeRendersCopy(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&stars=9"
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(f.slotID, 10), form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if want := "stars must be between 1 and 3."; !strings.Contains(body, want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, body)
	}
	if !strings.Contains(body, `id="grid-`) {
		t.Fatalf("422 must rerender the grid, got %s", body)
	}
}

func TestAttachItem_UnknownItemRendersPickCopy(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&item=999"
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(f.slotID, 10)+"/items", form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if want := "pick an item from the list."; !strings.Contains(body, want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, body)
	}
	if !strings.Contains(body, `id="grid-`) {
		t.Fatalf("422 must rerender the grid, got %s", body)
	}
	view, err := f.entry.Editor(context.Background(), f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if got := len(view.Lineups[0].Slots[0].Items); got != 1 {
		t.Fatalf("slot item count = %d, want the untouched 1", got)
	}
}

func TestAttachItem_UnknownItemNonHXRedirects(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&item=999"
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(f.slotID, 10)+"/items", form, false)
	assertHXRedirect(t, rec, http.StatusSeeOther, "/edit")
}

func TestAddRelic_UnknownRelicRendersPickCopy(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&relic=999"
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/relics", form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if want := "pick a relic from the list."; !strings.Contains(body, want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, body)
	}
	if !strings.Contains(body, `id="relics-`) {
		t.Fatalf("422 must rerender the relics region, got %s", body)
	}
}

func TestAddRelic_UnknownRelicNonHXRedirects(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&relic=999"
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/relics", form, false)
	assertHXRedirect(t, rec, http.StatusSeeOther, "/edit")
}

func TestAddRelic_ReturnsRelicsRegion(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&relic=" + strconv.FormatInt(f.relicID, 10)
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/relics", form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="relics-`) || !strings.Contains(body, "tide bell") {
		t.Fatalf("add relic must rerender the relics chip, got %s", body)
	}
}

func TestRemoveRelic_NotHeldFallsBack(t *testing.T) {
	f := seedEditor(t)
	qs := "match_id=" + strconv.FormatInt(f.matchID, 10)
	rec := f.post(t, "DELETE", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/relics/"+strconv.FormatInt(f.relicID, 10)+"?"+qs, "", true)
	assertMutationFallback(t, rec)
	if err := f.st.AddLineupRelic(context.Background(), f.lineupID, f.relicID); err != nil {
		t.Fatalf("lineup must stay writable after the refused remove: %v", err)
	}
}

func TestRemoveItem_NotHeldFallsBack(t *testing.T) {
	f := seedEditor(t)
	pad, err := f.st.CreateItem(context.Background(), domain.Item{Name: "pad", Tier: 1}, nil)
	if err != nil {
		t.Fatalf("pad item: %v", err)
	}
	qs := "match_id=" + strconv.FormatInt(f.matchID, 10)
	rec := f.post(t, "DELETE", "/slots/"+strconv.FormatInt(f.slotID, 10)+"/items/"+strconv.FormatInt(pad, 10)+"?"+qs, "", true)
	assertMutationFallback(t, rec)
	view, err := f.entry.Editor(context.Background(), f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if got := len(view.Lineups[0].Slots[0].Items); got != 1 {
		t.Fatalf("slot item count = %d, want the untouched 1", got)
	}
}

func TestViewForSlot_CrossMatchFallsBack(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	heroID := fixtureHeroID(t, f)
	patch, err := f.st.CreatePatch(ctx, domain.Patch{Version: "8.1", ReleasedAt: "2026-09-02"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	matchB, err := f.entry.CreateMatch(ctx, domain.Match{PatchID: patch, PlayedAt: 1789000500, Source: "pro"})
	if err != nil {
		t.Fatalf("match b: %v", err)
	}
	viewB, _, err := f.entry.AddLineup(ctx, matchB, domain.AddLineupCmd{
		Label: "elsewhere", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: heroID}, Stars: 3}},
	}, 0)
	if err != nil {
		t.Fatalf("lineup b: %v", err)
	}
	slotB := viewB.Lineups[0].Slots[0].ID
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&stars=2"
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(slotB, 10), form, true)
	assertMutationFallback(t, rec)
	fresh, err := f.entry.Editor(ctx, matchB)
	if err != nil {
		t.Fatalf("editor b: %v", err)
	}
	if got := fresh.Lineups[0].Slots[0].Stars; got != 3 {
		t.Fatalf("cross-match post changed slot b stars to %d, want 3", got)
	}
}

func TestViewForSlot_UnknownMatchFallsBack(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(f.slotID, 10), "match_id=999999&stars=3", true)
	assertMutationFallback(t, rec)
}
