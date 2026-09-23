package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

func TestUpdateLineup_Placement422PreservesTypedValues(t *testing.T) {
	f := seedEditor(t)
	l := strconv.FormatInt(f.lineupID, 10)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&placement=0&label=kept&wins=1&draws=0&losses=0&networth=10"
	rec := f.post(t, "POST", "/lineups/"+l, form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="editform-`+l+`"`) {
		t.Fatalf("422 must re-render the issuing edit form, got %s", body)
	}
	// Anchor the typed 0 to the placement input: draws=0 and losses=0 also
	// render value="0", so a bare value="0" match cannot fail on regression.
	if !strings.Contains(body, `name="placement" min="1" max="8" value="0"`) || !strings.Contains(body, `value="kept"`) {
		t.Fatalf("422 must preserve typed values, got %s", body)
	}
	if want := "placement must be between 1 and 8."; !strings.Contains(body, want) {
		t.Fatalf("body must carry spec copy %q, got %s", want, body)
	}
	if !strings.Contains(body, "data-autofocus") {
		t.Fatalf("422 must autofocus the bad field, got %s", body)
	}
}

func TestUpdateLineup_NegativeRecord422HidesRecordMessage(t *testing.T) {
	f := seedEditor(t)
	l := strconv.FormatInt(f.lineupID, 10)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&placement=1&wins=-1&draws=0&losses=0&networth=0"
	rec := f.post(t, "POST", "/lineups/"+l, form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="editform-`+l+`"`) {
		t.Fatalf("422 must re-render the issuing edit form, got %s", body)
	}
	if !strings.Contains(body, `value="-1"`) {
		t.Fatalf("422 must preserve the typed wins value, got %s", body)
	}
	// Pinned current behavior: EditForm renders only placement errors; the
	// record error is computed, steals the autofocus, but never shows text.
	if want := "wins, draws, losses and networth cannot be negative."; strings.Contains(body, want) {
		t.Fatalf("record message is currently absent from EditForm; update this pin if it renders: %s", body)
	}
	if !strings.Contains(body, "data-autofocus") {
		t.Fatalf("record error must still steal the autofocus, got %s", body)
	}
}

func TestUpdateLineup_SamePlacementReturnsCardFragment(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	proID, err := f.st.CreatePro(ctx, domain.Pro{Name: "maru", Handle: "maru", PeakRank: "gm"})
	if err != nil {
		t.Fatalf("pro: %v", err)
	}
	l := strconv.FormatInt(f.lineupID, 10)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&placement=1&label=renamed&wins=3&draws=1&losses=0&networth=250&pro=" + strconv.FormatInt(proID, 10)
	rec := f.post(t, "POST", "/lineups/"+l, form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="card-`+l+`"`) {
		t.Fatalf("same-placement swap must return one card fragment, got %s", body)
	}
	if strings.Contains(body, `id="cards-`) {
		t.Fatalf("same-placement swap must not re-render the whole cards region, got %s", body)
	}
	if !strings.Contains(body, "renamed") || !strings.Contains(body, "1/8") {
		t.Fatalf("card must show the new label and unchanged pips, got %s", body)
	}
	view, err := f.entry.Editor(ctx, f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	got := view.Lineups[0]
	if got.Placement != 1 || got.Label != "renamed" || got.Wins != 3 || got.Networth != 250 || got.ProID == nil || *got.ProID != proID {
		t.Fatalf("store must keep the edited scalars, got %+v", got)
	}
}

func TestUpdateLineup_MovedPlacementReturnsCardsAndPips(t *testing.T) {
	f := seedEditor(t)
	l := strconv.FormatInt(f.lineupID, 10)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&placement=2&label=first&wins=1&draws=0&losses=0&networth=10"
	rec := f.post(t, "POST", "/lineups/"+l, form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="cards-`) || !strings.Contains(body, `id="pips-`) {
		t.Fatalf("moved placement must re-render cards and pips, got %s", body)
	}
	view, err := f.entry.Editor(context.Background(), f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if view.Lineups[0].Placement != 2 {
		t.Fatalf("placement = %d, want 2", view.Lineups[0].Placement)
	}
}

func TestUpdateLineup_NonHXRedirectsToEditAfterSave(t *testing.T) {
	f := seedEditor(t)
	l := strconv.FormatInt(f.lineupID, 10)
	m := strconv.FormatInt(f.matchID, 10)
	form := "match_id=" + m + "&placement=1&label=saved&wins=1&draws=0&losses=0&networth=10"
	rec := f.post(t, "POST", "/lineups/"+l, form, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/matches/"+m+"/edit" {
		t.Fatalf("location = %q, want /matches/%s/edit", loc, m)
	}
	view, err := f.entry.Editor(context.Background(), f.matchID)
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	if view.Lineups[0].Label != "saved" {
		t.Fatalf("non-HX save must still persist, got %+v", view.Lineups[0])
	}
}

func TestUpdateLineup_LineupOutsideNamedMatchFallsBackToBare422(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	patch, err := f.st.CreatePatch(ctx, domain.Patch{Version: "8.1", ReleasedAt: "2026-09-02"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	other, err := f.entry.CreateMatch(ctx, domain.Match{PatchID: patch, PlayedAt: 1789000100, Source: "pro"})
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	view, _, err := f.entry.AddLineup(ctx, other, domain.AddLineupCmd{Label: "elsewhere", Placement: 1}, 0)
	if err != nil {
		t.Fatalf("lineup: %v", err)
	}
	lid := strconv.FormatInt(view.Lineups[0].ID, 10)
	form := "match_id=" + strconv.FormatInt(f.matchID, 10) + "&placement=1&label=x&wins=1&draws=0&losses=0&networth=0"
	rec := f.post(t, "POST", "/lineups/"+lid, form, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("fallback must answer HX with a bare 422 and no fragments, got %s", body)
	}
}

func TestCopyLineup_ErrorRendersCardFragmentWithCopy(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	for p := 2; p <= 8; p++ {
		if _, _, err := f.entry.AddLineup(ctx, f.matchID, domain.AddLineupCmd{Label: "b" + strconv.Itoa(p), Placement: p}, 0); err != nil {
			t.Fatalf("fill lineup %d: %v", p, err)
		}
	}
	l := strconv.FormatInt(f.lineupID, 10)
	rec := f.post(t, "POST", "/matches/"+strconv.FormatInt(f.matchID, 10)+"/lineups", "copy_from="+l, true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="card-`+l+`"`) {
		t.Fatalf("copy error must render the source card fragment, got %s", body)
	}
	if want := "all 8 placements in this match are already taken."; !strings.Contains(body, want) {
		t.Fatalf("card must carry the copy error copy %q, got %s", want, body)
	}
}
