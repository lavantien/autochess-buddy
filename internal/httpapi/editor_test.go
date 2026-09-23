package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

// editorFixture seeds one pro match with 1 lineup holding 1 hero and returns the
// pieces the handler tests poke at.
type editorFixture struct {
	h        http.Handler
	st       *sqlite.Store
	entry    service.EntryService
	matchID  int64
	lineupID int64
	slotID   int64
	itemID   int64
	relicID  int64
	heroName string
}

func seedEditor(t *testing.T) editorFixture {
	t.Helper()
	h, st, entry := newTestServer(t)
	ctx := context.Background()
	patch, err := st.CreatePatch(ctx, domain.Patch{Version: "8.0", ReleasedAt: "2026-09-01"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	race, err := st.CreateRace(ctx, domain.Race{Name: "beast"}, nil)
	if err != nil {
		t.Fatalf("race: %v", err)
	}
	class, err := st.CreateClass(ctx, domain.Class{Name: "knight"}, nil)
	if err != nil {
		t.Fatalf("class: %v", err)
	}
	hero, err := st.CreateHero(ctx, domain.Hero{Name: "grim jaw", Cost: 2, Races: []domain.Race{{ID: race}}, Classes: []domain.Class{{ID: class}}})
	if err != nil {
		t.Fatalf("hero: %v", err)
	}
	itemID, err := st.CreateItem(ctx, domain.Item{Name: "storm core", Tier: 3}, nil)
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	relicID, err := st.CreateRelic(ctx, domain.Relic{Name: "tide bell", Effect: "heal"})
	if err != nil {
		t.Fatalf("relic: %v", err)
	}
	matchID, err := entry.CreateMatch(ctx, domain.Match{PatchID: patch, PlayedAt: 1789000000, Source: "pro"})
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	view, _, err := entry.AddLineup(ctx, matchID, domain.AddLineupCmd{
		Label: "first", Placement: 1,
		Slots: []domain.Slot{{SlotIndex: 0, Hero: domain.Hero{ID: hero}, Stars: 2, Items: []domain.Item{{ID: itemID}}}},
	}, 0)
	if err != nil {
		t.Fatalf("lineup: %v", err)
	}
	return editorFixture{h: h, st: st, entry: entry, matchID: matchID,
		lineupID: view.Lineups[0].ID, slotID: view.Lineups[0].Slots[0].ID,
		itemID: itemID, relicID: relicID, heroName: "grim jaw"}
}

func (f editorFixture) post(t *testing.T, method, path, form string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func TestAddLineup_OOBResponseContainsCardsAndPips(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/lineups", "placement=2&label=second", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="cards-`) || !strings.Contains(body, `id="pips-`) {
		t.Fatalf("oob response must carry cards and pips fragments, got %s", body)
	}
	if !strings.Contains(body, "second") || !strings.Contains(body, "2/8") {
		t.Fatalf("response must show the new card and advanced pips, got %s", body)
	}
}

func TestAddLineup_422PreservesValuesAndMarksFirstError(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/lineups", "placement=9&label=kept", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="addform-`) {
		t.Fatalf("422 must re-render the issuing form, got %s", body)
	}
	if !strings.Contains(body, `value="kept"`) || !strings.Contains(body, `value="9"`) {
		t.Fatalf("422 must preserve typed values, got %s", body)
	}
	if !strings.Contains(body, "placement must be between 1 and 8.") {
		t.Fatalf("422 must name the bad field, got %s", body)
	}
	if !strings.Contains(body, "data-autofocus") {
		t.Fatalf("422 must autofocus the first bad field, got %s", body)
	}
}

func TestAddLineup_PlacementConflictRendersSpecCopy(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/lineups", "placement=1&label=clash", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if want := "placement 1 is already used by another lineup in this match."; !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, rec.Body.String())
	}
}

func TestAddLineup_NonHXRedirectsToEditor(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/lineups", "placement=2&label=second", false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/edit") {
		t.Fatalf("location = %q, want the editor", loc)
	}
}

func TestCopyLineup_ResetsProAndScalars(t *testing.T) {
	f := seedEditor(t)
	form := "copy_from=" + strconv.FormatInt(f.lineupID, 10) + "&match_id=" + strconv.FormatInt(f.matchID, 10)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/lineups", form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "first") || !strings.Contains(body, "2/8") {
		t.Fatalf("copy must clone the label into a second card, got %s", body)
	}
	view, err := f.entry.Editor(context.Background(), f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	var copy *domain.Lineup
	for i := range view.Lineups {
		if view.Lineups[i].Placement == 2 {
			copy = &view.Lineups[i]
		}
	}
	if copy == nil || copy.ProID != nil || copy.Wins != 0 || copy.Networth != 0 || len(copy.Slots) != 1 {
		t.Fatalf("copy scalars must reset: %+v", copy)
	}
}

func TestAddSlot_FillsLowestFreeCell(t *testing.T) {
	f := seedEditor(t)
	if err := f.st.DeleteSlot(context.Background(), f.slotID); err != nil {
		t.Fatalf("punch hole: %v", err)
	}
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&hero=" + f.heroName + "&stars=2"
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/slots", form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `id="grid-`) || !strings.Contains(rec.Body.String(), `id="heroform-`) {
		t.Fatalf("add hero must return grid and reset heroform, got %s", rec.Body.String())
	}
	view, _ := f.entry.Editor(context.Background(), f.matchID)
	for _, sl := range view.Lineups[0].Slots {
		if sl.SlotIndex != 0 {
			t.Fatalf("new slot index = %d, want the refilled 0", sl.SlotIndex)
		}
	}
}

func TestAddSlot_UnknownHeroRendersSpecCopy(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&hero=ghost&stars=2"
	rec := f.post(t, "POST", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/slots", form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if want := "no hero named ghost in the codex. add it first."; !strings.Contains(body, want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, body)
	}
	if !strings.Contains(body, `value="ghost"`) {
		t.Fatalf("hero form must preserve the typed name, got %s", body)
	}
}

func TestDeleteSlot_ReturnsGrid(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10)
	rec := f.post(t, "DELETE", "/slots/"+strconv.FormatInt(f.slotID, 10)+"?"+form, "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="grid-`) {
		t.Fatalf("delete slot must return the grid, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "slot-empty") {
		t.Fatalf("deleted cell must render the placeholder, got %s", rec.Body.String())
	}
}

func TestDeleteLineup_ReturnsCardsAndPips(t *testing.T) {
	f := seedEditor(t)
	qs := "match_id=" + strconv.FormatInt(f.matchID, 10)
	rec := f.post(t, "DELETE", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"?"+qs, "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="cards-`) || !strings.Contains(body, `id="pips-`) {
		t.Fatalf("delete lineup must return cards and pips, got %s", body)
	}
	if !strings.Contains(body, "0/8") {
		t.Fatalf("pips must drop to 0/8, got %s", body)
	}
}

func TestFinalize_NonHXRedirectsToDetail(t *testing.T) {
	f := seedEditor(t)
	for p := 2; p <= 8; p++ {
		if _, _, err := f.entry.AddLineup(context.Background(), f.matchID, domain.AddLineupCmd{Label: "b" + strconv.Itoa(p), Placement: p}, 0); err != nil {
			t.Fatalf("fill lineup %d: %v", p, err)
		}
	}
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/finalize", "", false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/matches/"+strconv.FormatInt(f.matchID, 10)) {
		t.Fatalf("location = %q, want the detail page", loc)
	}
}

func TestFinalize_InvalidRendersBannerNamingProblem(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/finalize", "", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 rerender", rec.Code)
	}
	if want := "a pro match needs 8 lineups with placements 1 through 8 before it can be finalized."; !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("banner must name the problem, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "role=\"alert\"") {
		t.Fatalf("banner must be an alert, got %s", rec.Body.String())
	}
}

func TestSaveStars_UpdatesGrid(t *testing.T) {
	f := seedEditor(t)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&stars=3"
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(f.slotID, 10), form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="grid-`) {
		t.Fatalf("save stars must return the grid, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "3/3") {
		t.Fatalf("grid must show 3 stars, got %s", rec.Body.String())
	}
}

func TestAttachItem_CapsAtSix(t *testing.T) {
	f := seedEditor(t)
	// The slot already holds 1 item; five more legit ids then the cap fires.
	ids := []int64{f.itemID}
	for i := 0; i < 4; i++ {
		id, err := f.st.CreateItem(context.Background(), domain.Item{Name: "pad" + strconv.Itoa(i), Tier: 1}, nil)
		if err != nil {
			t.Fatalf("pad item: %v", err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := f.st.AddSlotItem(context.Background(), f.slotID, id); err != nil {
			t.Fatalf("pad slot item: %v", err)
		}
	}
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&item=" + strconv.FormatInt(ids[1], 10)
	rec := f.post(t, "POST", "/slots/"+strconv.FormatInt(f.slotID, 10)+"/items", form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if want := "a slot holds at most 6 items."; !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body must carry the cap copy, got %s", rec.Body.String())
	}
}

func TestRemoveRelic_RerendersRelics(t *testing.T) {
	f := seedEditor(t)
	if err := f.st.AddLineupRelic(context.Background(), f.lineupID, f.relicID); err != nil {
		t.Fatalf("attach relic: %v", err)
	}
	qs := "match_id=" + strconv.FormatInt(f.matchID, 10)
	rec := f.post(t, "DELETE", "/lineups/"+strconv.FormatInt(f.lineupID, 10)+"/relics/"+strconv.FormatInt(f.relicID, 10)+"?"+qs, "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="relics-`) {
		t.Fatalf("remove relic must return the relics region, got %s", body)
	}
	if strings.Count(body, "chip-label") != 0 {
		t.Fatalf("relic chip must be gone, got %s", body)
	}
}
