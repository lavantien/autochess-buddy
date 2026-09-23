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

func TestClassCreate_303And422Pair(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/classes", "name=paladin", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/classes" {
		t.Fatalf("create class status = %d location = %q, want 303 /classes: %s", rec.Code, rec.Header().Get("Location"), truncBody(rec.Body.String()))
	}
	classes, err := f.st.ListClasses(context.Background())
	if err != nil {
		t.Fatalf("list classes: %v", err)
	}
	found := false
	for _, cl := range classes {
		if cl.Name == "paladin" {
			found = true
		}
	}
	if !found {
		t.Fatalf("paladin must be stored, classes = %+v", classes)
	}
	// Empty name rerenders with the copy under the class create form only.
	rec = f.post(t, "POST", "/classes", "name=", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty name status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	if got := strings.Count(rec.Body.String(), "name is required."); got != 1 {
		t.Fatalf("name error must render once under the class create form, got %d: %s", got, truncBody(rec.Body.String()))
	}
	// A name the seeded knight class already holds refuses with the typed value.
	rec = f.post(t, "POST", "/classes", "name=knight", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("taken status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "that name is taken.") {
		t.Fatalf("body must render the taken copy, got %s", truncBody(body))
	}
	if !strings.Contains(body, `value="knight" data-autofocus`) {
		t.Fatalf("create input must keep the typed value and take focus, got %s", truncBody(body))
	}
}

func TestClassLadder_TierAppendReplaceDelete(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	class, err := f.st.CreateClass(ctx, domain.Class{Name: "paladin"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	path := "/classes/" + strconv.FormatInt(class, 10)
	post := func(form string) *httptest.ResponseRecorder {
		return f.post(t, "POST", path, form, false)
	}
	if rec := post("mode=save_tier&count=3&effect=smite"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/classes" {
		t.Fatalf("append tier status = %d location = %q, want 303 /classes: %s", rec.Code, rec.Header().Get("Location"), truncBody(rec.Body.String()))
	}
	_, tiers, err := f.st.GetClass(ctx, class)
	if err != nil || len(tiers) != 1 || tiers[0].Count != 3 || tiers[0].Effect != "smite" {
		t.Fatalf("tier after save = %+v, err %v", tiers, err)
	}
	// The same count replaces the rung instead of appending.
	if rec := post("mode=save_tier&count=3&effect=avenging light"); rec.Code != http.StatusSeeOther {
		t.Fatalf("replace tier status = %d, want 303: %s", rec.Code, truncBody(rec.Body.String()))
	}
	_, tiers, err = f.st.GetClass(ctx, class)
	if err != nil || len(tiers) != 1 || tiers[0].Effect != "avenging light" {
		t.Fatalf("tier after replace = %+v, err %v", tiers, err)
	}
	// A new count appends a second rung.
	if rec := post("mode=save_tier&count=5&effect=bulwark"); rec.Code != http.StatusSeeOther {
		t.Fatalf("append second tier status = %d, want 303", rec.Code)
	}
	// Deleting a count drops only that rung.
	if rec := post("mode=delete_tier&count=3"); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete tier status = %d, want 303: %s", rec.Code, truncBody(rec.Body.String()))
	}
	_, tiers, err = f.st.GetClass(ctx, class)
	if err != nil || len(tiers) != 1 || tiers[0].Count != 5 || tiers[0].Effect != "bulwark" {
		t.Fatalf("tier after delete = %+v, err %v", tiers, err)
	}
}

func TestClassTier_422NamesEntityAndKeepsTypedValues(t *testing.T) {
	f := seedEditor(t)
	class, err := f.st.CreateClass(context.Background(), domain.Class{Name: "paladin"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	rec := f.post(t, "POST", "/classes/"+strconv.FormatInt(class, 10), "mode=save_tier&count=0&effect=fury", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if got := strings.Count(body, "paladin tier count must be at least 1."); got != 1 {
		t.Fatalf("tier error must name the entity and render once, got %d: %s", got, truncBody(body))
	}
	if !strings.Contains(body, `value="0"`) || !strings.Contains(body, `value="fury"`) {
		t.Fatalf("tier 422 must keep the typed count and effect, got %s", truncBody(body))
	}
}

func TestSynergyUpdate_UnknownModeRerendersWith422(t *testing.T) {
	f := seedEditor(t)
	class, err := f.st.CreateClass(context.Background(), domain.Class{Name: "paladin"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	rec := f.post(t, "POST", "/classes/"+strconv.FormatInt(class, 10), "mode=lob", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	if got := strings.Count(rec.Body.String(), "unknown action."); got != 1 {
		t.Fatalf("unknown action copy must render once, got %d: %s", got, truncBody(rec.Body.String()))
	}
	// The refused mutation must not touch the ladder row.
	_, tiers, err := f.st.GetClass(context.Background(), class)
	if err != nil || len(tiers) != 0 {
		t.Fatalf("tiers must stay empty, got %+v err %v", tiers, err)
	}
}

func TestSynergyUpdate_MissingLadderRedirectsPerEntity(t *testing.T) {
	f := seedEditor(t)
	// Each ladder falls back to its own tab.
	for _, tc := range []struct{ path, want string }{
		{"/races/999", "/races"},
		{"/classes/999", "/classes"},
	} {
		rec := f.post(t, "POST", tc.path, "mode=save_name&name=ghost", false)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s status = %d, want 303: %s", tc.path, rec.Code, truncBody(rec.Body.String()))
		}
		if loc := rec.Header().Get("Location"); loc != tc.want {
			t.Fatalf("%s location = %q, want %s", tc.path, loc, tc.want)
		}
	}
}

func TestSynergyIndex_RendersBothLaddersAndTierRows(t *testing.T) {
	f := seedEditor(t)
	race, err := f.st.CreateRace(context.Background(), domain.Race{Name: "human"}, nil)
	if err != nil {
		t.Fatalf("create race: %v", err)
	}
	rec := f.post(t, "POST", "/races/"+strconv.FormatInt(race, 10), "mode=save_tier&count=2&effect=roar", false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save tier status = %d, want 303: %s", rec.Code, truncBody(rec.Body.String()))
	}
	req := httptest.NewRequest(http.MethodGet, "/races", nil)
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<div class="ladder">`, "<h1>races</h1>", "<h1>classes</h1>",
		"beast", "knight", `<td>2</td><td>roar</td>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("synergies page must contain %q, got %s", want, truncBody(body))
		}
	}
}

func TestDeleteRaceAndClassRoutes(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	race, err := f.st.CreateRace(ctx, domain.Race{Name: "ember"}, nil)
	if err != nil {
		t.Fatalf("create race: %v", err)
	}
	class, err := f.st.CreateClass(ctx, domain.Class{Name: "seer"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	rec := f.post(t, "DELETE", "/races/"+strconv.FormatInt(race, 10), "", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/races" {
		t.Fatalf("delete race status = %d location = %q, want 303 /races", rec.Code, rec.Header().Get("Location"))
	}
	rec = f.post(t, "DELETE", "/classes/"+strconv.FormatInt(class, 10), "", true)
	if rec.Code != http.StatusNoContent || rec.Header().Get("HX-Redirect") != "/classes" {
		t.Fatalf("delete class hx status = %d redirect = %q, want 204 /classes", rec.Code, rec.Header().Get("HX-Redirect"))
	}
	races, err := f.st.ListRaces(ctx)
	if err != nil {
		t.Fatalf("list races: %v", err)
	}
	for _, r := range races {
		if r.ID == race {
			t.Fatal("race row survived delete")
		}
	}
	// A race still held by the seeded hero answers with the 409 synergies page
	// and the conflict copy under its ladder, not a bogus 303.
	hero, err := f.st.GetHeroByName(ctx, f.heroName)
	if err != nil {
		t.Fatalf("hero: %v", err)
	}
	if len(hero.Races) == 0 {
		t.Fatal("seeded hero has no race to delete under it")
	}
	rec = f.post(t, "DELETE", "/races/"+strconv.FormatInt(hero.Races[0].ID, 10), "", false)
	if rec.Code != http.StatusConflict {
		t.Fatalf("in-use race delete status = %d, want 409: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<h1>races</h1>") || !strings.Contains(body, domain.ErrInUse.Error()) {
		t.Fatalf("in-use race delete must rerender the synergies page with the conflict copy, got %s", truncBody(body))
	}
	// The hx arm of the same contract on the class side: the class behind the
	// hero refuses too, with no HX-Redirect pretending the delete landed.
	if len(hero.Classes) == 0 {
		t.Fatal("seeded hero has no class to delete under it")
	}
	rec = f.post(t, "DELETE", "/classes/"+strconv.FormatInt(hero.Classes[0].ID, 10), "", true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("in-use class delete hx status = %d, want 409: %s", rec.Code, truncBody(rec.Body.String()))
	}
	if hxr := rec.Header().Get("HX-Redirect"); hxr != "" {
		t.Fatalf("in-use class delete must not send HX-Redirect, got %q", hxr)
	}
	// The FK refuses the delete: the race must survive behind the hero.
	races, err = f.st.ListRaces(context.Background())
	if err != nil {
		t.Fatalf("list races after refused delete: %v", err)
	}
	survived := false
	for _, r := range races {
		if r.ID == hero.Races[0].ID {
			survived = true
		}
	}
	if !survived {
		t.Fatal("race row vanished behind an in-use hero despite the FK refusal")
	}
}
