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

func heroID(id int64) string { return strconv.FormatInt(id, 10) }

// TestHeroEdit_LineageSelects pins the stored 0/1/2 races against 0/1/2 classes
// onto the four lineage selects of the edit page.
func TestHeroEdit_LineageSelects(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	elf, err := f.st.CreateRace(ctx, domain.Race{Name: "elf"}, nil)
	if err != nil {
		t.Fatalf("race: %v", err)
	}
	hunter, err := f.st.CreateClass(ctx, domain.Class{Name: "hunter"}, nil)
	if err != nil {
		t.Fatalf("class: %v", err)
	}
	gr, err := f.st.GetHeroByName(ctx, f.heroName)
	if err != nil {
		t.Fatalf("seed hero: %v", err)
	}
	r1, c1 := gr.Races[0].ID, gr.Classes[0].ID
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/heroes/"+heroID(gr.ID), nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `value="grim jaw"`) || !strings.Contains(body, `value="2"`) {
		t.Fatalf("seed edit must 200 and prefill name and cost, got %d: %s", rec.Code, body)
	}

	cases := []struct {
		name           string
		races, classes []int64
		r1, r2, c1, c2 int64
	}{
		{"bare soul", nil, nil, 0, 0, 0, 0},
		{"lone beast", []int64{r1}, nil, r1, 0, 0, 0},
		{"lone knight", nil, []int64{c1}, 0, 0, c1, 0},
		{"twin races", []int64{r1, elf}, nil, r1, elf, 0, 0},
		{"twin classes", nil, []int64{c1, hunter}, 0, 0, c1, hunter},
		{"one of each", []int64{r1}, []int64{c1}, r1, 0, c1, 0},
		{"full kit", []int64{r1, elf}, []int64{c1, hunter}, r1, elf, c1, hunter},
		{"swapped twins", []int64{elf, r1}, []int64{hunter, c1}, elf, r1, hunter, c1},
	}
	for _, tc := range cases {
		races, classes := []domain.Race{}, []domain.Class{}
		for _, id := range tc.races {
			races = append(races, domain.Race{ID: id})
		}
		for _, id := range tc.classes {
			classes = append(classes, domain.Class{ID: id})
		}
		id, err := f.st.CreateHero(ctx, domain.Hero{Name: tc.name, Cost: 2, Races: races, Classes: classes})
		if err != nil {
			t.Fatalf("create %s: %v", tc.name, err)
		}
		rec := httptest.NewRecorder()
		f.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/heroes/"+heroID(id), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200: %s", tc.name, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		picks := map[string]int64{"race1": tc.r1, "race2": tc.r2, "class1": tc.c1, "class2": tc.c2}
		want := 0
		for slot, wantID := range picks {
			if wantID == 0 {
				continue
			}
			pick := `value="` + heroID(wantID) + `" selected>`
			if !strings.Contains(body, pick) {
				t.Errorf("%s: %s must carry %s: %s", tc.name, slot, pick, body)
			}
			want++
		}
		if got := strings.Count(body, "selected"); got != want {
			t.Errorf("%s: selected marks = %d, want %d: %s", tc.name, got, want, body)
		}
	}
}

func TestHeroEdit_MissingIDRedirects(t *testing.T) {
	f := seedEditor(t)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/heroes/4242", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/heroes" {
		t.Fatalf("location = %q, want /heroes", loc)
	}
}

func TestHeroUpdate_MissingIDRedirects(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/heroes/4242", "name=ember mage&cost=2", false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/heroes" {
		t.Fatalf("location = %q, want /heroes", loc)
	}
}

func TestHeroUpdate_422ArmsAndValidRename(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	gr, err := f.st.GetHeroByName(ctx, f.heroName)
	if err != nil {
		t.Fatalf("seed hero: %v", err)
	}
	r1, c1 := gr.Races[0].ID, gr.Classes[0].ID
	spare, err := f.st.CreateHero(ctx, domain.Hero{
		Name: "ash monk", Cost: 3,
		Races:   []domain.Race{{ID: r1}},
		Classes: []domain.Class{{ID: c1}},
	})
	if err != nil {
		t.Fatalf("spare hero: %v", err)
	}
	path := "/heroes/" + heroID(spare)

	cases := []struct {
		label, form, copy string
	}{
		{"cost 0", "name=ash monk&cost=0", "cost must be between 1 and 5."},
		{"cost 9", "name=ash monk&cost=9", "cost must be between 1 and 5."},
		{"no lineage", "name=ash monk&cost=3", "a hero carries 1 to 2 races and 1 to 2 classes."},
	}
	for _, tc := range cases {
		rec := f.post(t, "POST", path, tc.form, false)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status = %d, want 422: %s", tc.label, rec.Code, rec.Body.String())
		}
		if got := strings.Count(rec.Body.String(), tc.copy); got != 1 {
			t.Fatalf("%s: copy must render once, got %d: %s", tc.label, got, rec.Body.String())
		}
	}

	// Renaming onto a taken name keeps the typed value and the store untouched.
	rec := f.post(t, "POST", path, "name=grim jaw&cost=3&race1="+heroID(r1)+"&class1="+heroID(c1), false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("taken: status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if got := strings.Count(rec.Body.String(), "that name is taken."); got != 1 {
		t.Fatalf("taken copy must render once, got %d: %s", got, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `value="grim jaw"`) {
		t.Fatalf("taken rerender must keep the typed name: %s", rec.Body.String())
	}
	if h, err := f.st.GetHeroByName(ctx, "ash monk"); err != nil || h.Cost != 3 {
		t.Fatalf("failed updates must leave the store untouched: %+v, err %v", h, err)
	}

	rec = f.post(t, "POST", path, "name=ember mage&cost=4&race1="+heroID(r1)+"&class1="+heroID(c1), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/heroes" {
		t.Fatalf("rename: status = %d, want 303 to /heroes: %s", rec.Code, rec.Body.String())
	}
	if h, err := f.st.GetHeroByName(ctx, "ember mage"); err != nil || h.ID != spare || h.Cost != 4 {
		t.Fatalf("rename must land: %+v, err %v", h, err)
	}
	if _, err := f.st.GetHeroByName(ctx, "ash monk"); err == nil {
		t.Fatal("old name must be gone after the rename")
	}
}

// TestHeroCreate_LineageSlotCombos drives decodeHero through both select slots.
func TestHeroCreate_LineageSlotCombos(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	elf, err := f.st.CreateRace(ctx, domain.Race{Name: "elf"}, nil)
	if err != nil {
		t.Fatalf("race: %v", err)
	}
	hunter, err := f.st.CreateClass(ctx, domain.Class{Name: "hunter"}, nil)
	if err != nil {
		t.Fatalf("class: %v", err)
	}
	gr, err := f.st.GetHeroByName(ctx, f.heroName)
	if err != nil {
		t.Fatalf("seed hero: %v", err)
	}
	r1, c1 := gr.Races[0].ID, gr.Classes[0].ID
	cases := []struct {
		name, form     string
		races, classes []int64
	}{
		{"both twos", "name=both twos&cost=2&race1=" + heroID(r1) + "&race2=" + heroID(elf) +
			"&class1=" + heroID(c1) + "&class2=" + heroID(hunter), []int64{r1, elf}, []int64{c1, hunter}},
		{"second slots", "name=second slots&cost=2&race2=" + heroID(elf) + "&class2=" + heroID(hunter),
			[]int64{elf}, []int64{hunter}},
		{"mixed slots", "name=mixed slots&cost=2&race1=" + heroID(r1) + "&class2=" + heroID(hunter),
			[]int64{r1}, []int64{hunter}},
	}
	for _, tc := range cases {
		rec := f.post(t, "POST", "/heroes", tc.form, false)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s: status = %d, want 303: %s", tc.name, rec.Code, rec.Body.String())
		}
		h, err := f.st.GetHeroByName(ctx, tc.name)
		if err != nil {
			t.Fatalf("%s: hero must exist: %v", tc.name, err)
		}
		if len(h.Races) != len(tc.races) || len(h.Classes) != len(tc.classes) {
			t.Fatalf("%s: lineages = %d races %d classes, want %d/%d", tc.name,
				len(h.Races), len(h.Classes), len(tc.races), len(tc.classes))
		}
		for i, id := range tc.races {
			if h.Races[i].ID != id {
				t.Fatalf("%s: race %d = %d, want %d", tc.name, i, h.Races[i].ID, id)
			}
		}
		for i, id := range tc.classes {
			if h.Classes[i].ID != id {
				t.Fatalf("%s: class %d = %d, want %d", tc.name, i, h.Classes[i].ID, id)
			}
		}
	}
}

// TestHeroErrorRerenders_CopyOncePerPage pins both renderHeroError arms: the
// create arm rerenders the heroes index, the update arm the edit page.
func TestHeroErrorRerenders_CopyOncePerPage(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	rec := f.post(t, "POST", "/heroes", "name=shade&cost=9", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create arm: status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if got := strings.Count(body, "cost must be between 1 and 5."); got != 1 {
		t.Fatalf("create arm copy must render once, got %d: %s", got, body)
	}
	if !strings.Contains(body, "grim jaw") {
		t.Fatalf("create arm must rerender the heroes index, got %s", body)
	}

	gr, err := f.st.GetHeroByName(ctx, f.heroName)
	if err != nil {
		t.Fatalf("seed hero: %v", err)
	}
	rec = f.post(t, "POST", "/heroes/"+heroID(gr.ID), "name=grim jaw&cost=2", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update arm: status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if got := strings.Count(body, "a hero carries 1 to 2 races and 1 to 2 classes."); got != 1 {
		t.Fatalf("update arm copy must render once, got %d: %s", got, body)
	}
	if !strings.Contains(body, `value="grim jaw"`) {
		t.Fatalf("update arm must rerender the edit page with the value, got %s", body)
	}
}
