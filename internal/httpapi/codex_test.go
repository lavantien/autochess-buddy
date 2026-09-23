package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/seed"
)

func seedCodexFix(t *testing.T) editorFixture {
	t.Helper()
	f := seedEditor(t)
	return f
}

func TestCreateHero_303OnSuccess(t *testing.T) {
	f := seedCodexFix(t)
	race, _ := f.st.CreateRace(context.Background(), domain.Race{Name: "elf"}, nil)
	class, _ := f.st.CreateClass(context.Background(), domain.Class{Name: "hunter"}, nil)
	form := "name=dusk ranger&cost=3&race1=" + strconv.FormatInt(race, 10) + "&class1=" + strconv.FormatInt(class, 10)
	rec := f.post(t, "POST", "/heroes", form, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/heroes" {
		t.Fatalf("location = %q, want /heroes", loc)
	}
	if _, err := f.st.GetHeroByName(context.Background(), "dusk ranger"); err != nil {
		t.Fatalf("hero must exist: %v", err)
	}
}

func TestCreateHero_422RerendersWithErrors(t *testing.T) {
	f := seedCodexFix(t)
	// Cost out of range: the cost copy fires and values survive.
	rec := f.post(t, "POST", "/heroes", "name=shade&cost=9", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="shade"`) || !strings.Contains(body, `value="9"`) {
		t.Fatalf("rerender must preserve values, got %s", body)
	}
	if !strings.Contains(body, "cost must be between 1 and 5.") {
		t.Fatalf("rerender must name the field error, got %s", body)
	}
	if !strings.Contains(body, "data-autofocus") {
		t.Fatalf("first bad field must take focus, got %s", body)
	}
	// Valid cost, no lineage selected: the lineage copy fires alone.
	rec = f.post(t, "POST", "/heroes", "name=shade&cost=3", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("lineage case status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "a hero carries 1 to 2 races and 1 to 2 classes.") {
		t.Fatalf("rerender must name the lineage error, got %s", rec.Body.String())
	}
}

func TestDeleteHero_HXRedirectHeader(t *testing.T) {
	f := seedCodexFix(t)
	race, _ := f.st.CreateRace(context.Background(), domain.Race{Name: "elf"}, nil)
	class, _ := f.st.CreateClass(context.Background(), domain.Class{Name: "hunter"}, nil)
	id, err := f.st.CreateHero(context.Background(), domain.Hero{
		Name: "dusk ranger", Cost: 3,
		Races:   []domain.Race{{ID: race}},
		Classes: []domain.Class{{ID: class}},
	})
	if err != nil {
		t.Fatalf("create spare hero: %v", err)
	}
	path := "/heroes/" + strconv.FormatInt(id, 10)
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); got != "/heroes" {
		t.Fatalf("HX-Redirect = %q, want /heroes", got)
	}
	if _, err := f.st.GetHero(context.Background(), id); err == nil {
		t.Fatal("hero must be gone")
	}
}

func TestDeleteHero_InUseRendersConflict(t *testing.T) {
	f := seedCodexFix(t)
	// The fixture lineup holds grim jaw, so history refuses the delete.
	h, _ := f.st.GetHeroByName(context.Background(), f.heroName)
	path := "/heroes/" + strconv.FormatInt(h.ID, 10)
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "existing matches keep their history.") {
		t.Fatalf("body must carry the conflict copy, got %s", rec.Body.String())
	}
	if _, err := f.st.GetHero(context.Background(), h.ID); err != nil {
		t.Fatalf("hero must survive the refused delete: %v", err)
	}
}

func TestSaveTier_RoundTrip(t *testing.T) {
	f := seedCodexFix(t)
	race, _ := f.st.CreateRace(context.Background(), domain.Race{Name: "human"}, nil)
	post := func(vals ...string) *httptest.ResponseRecorder {
		t.Helper()
		v := url.Values{}
		for i := 0; i+1 < len(vals); i += 2 {
			v.Set(vals[i], vals[i+1])
		}
		return f.post(t, "POST", "/races/"+strconv.FormatInt(race, 10), v.Encode(), false)
	}
	rec := post("mode", "save_tier", "count", "2", "effect", "all humans +10% atk")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save tier status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	_, tiers, err := f.st.GetRace(context.Background(), race)
	if err != nil || len(tiers) != 1 || tiers[0].Count != 2 || tiers[0].Effect != "all humans +10% atk" {
		t.Fatalf("tier after save = %+v, err %v", tiers, err)
	}
	// Replace the rung, then delete it.
	if rec := post("mode", "save_tier", "count", "2", "effect", "all humans +25% atk"); rec.Code != http.StatusSeeOther {
		t.Fatalf("replace tier status = %d", rec.Code)
	}
	if rec := post("mode", "delete_tier", "count", "2"); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete tier status = %d", rec.Code)
	}
	_, tiers, err = f.st.GetRace(context.Background(), race)
	if err != nil || len(tiers) != 0 {
		t.Fatalf("tier after delete = %+v, err %v", tiers, err)
	}
}

func TestProAndPatch_303And422Pairs(t *testing.T) {
	f := seedCodexFix(t)
	// Pro pair.
	if rec := f.post(t, "POST", "/pros", "name=drift&handle=drifttt&peak_rank=challenger", false); rec.Code != http.StatusSeeOther {
		t.Fatalf("pro create status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	rec := f.post(t, "POST", "/pros", "name=", false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "name is required.") {
		t.Fatalf("pro create empty status = %d, want 422 with copy: %s", rec.Code, rec.Body.String())
	}
	// Patch pair.
	if rec := f.post(t, "POST", "/patches", "version=8.1&released_at=2026-10-01", false); rec.Code != http.StatusSeeOther {
		t.Fatalf("patch create status = %d, want 303", rec.Code)
	}
	rec = f.post(t, "POST", "/patches", "version=", false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "version is required.") {
		t.Fatalf("patch create empty status = %d, want 422 with copy: %s", rec.Code, rec.Body.String())
	}
}

// TestCodexIndexShowsSeededAnalytics pins the read-only columns on real data.
func TestCodexIndexShowsSeededAnalytics(t *testing.T) {
	h, st, _ := newTestServer(t)
	if err := seed.Load(st.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/heroes", nil)
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "grim jaw") || !strings.Contains(body, "9") {
		t.Fatalf("heroes table must show the fixture hero and its lineups count, got %s", body)
	}
}
