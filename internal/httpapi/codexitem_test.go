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

// truncBody keeps failure messages readable.
func truncBody(s string) string {
	if len(s) > 400 {
		return s[:400]
	}
	return s
}

// itemSpare adds an item no lineup slot or recipe references.
func itemSpare(t *testing.T, f editorFixture, name string) int64 {
	t.Helper()
	id, err := f.st.CreateItem(context.Background(), domain.Item{Name: name, Tier: 1, Effect: "spark"}, nil)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return id
}

func postDeleteItem(t *testing.T, f editorFixture, id int64, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/items/"+strconv.FormatInt(id, 10), nil)
	req.Header.Set("Origin", "http://example.com")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func TestItemEdit_MissingItemRedirects(t *testing.T) {
	f := seedEditor(t)
	req := httptest.NewRequest(http.MethodGet, "/items/999", nil)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, truncBody(rec.Body.String()))
	}
	if loc := rec.Header().Get("Location"); loc != "/items" {
		t.Fatalf("location = %q, want /items", loc)
	}
}

func TestItemEdit_RendersStoredComponentsAsSelected(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	hammer := itemSpare(t, f, "hammer")
	blade, err := f.st.CreateItem(ctx, domain.Item{Name: "storm blade", Tier: 4, Effect: "zap"}, []int64{hammer})
	if err != nil {
		t.Fatalf("create blade: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/items/"+strconv.FormatInt(blade, 10), nil)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<h1>storm blade</h1>") {
		t.Fatalf("edit page must name the item, got %s", truncBody(body))
	}
	// The recipe multi-select preselects the stored component only.
	if !strings.Contains(body, `value="`+strconv.FormatInt(hammer, 10)+`" selected`) {
		t.Fatalf("stored component must render selected, got %s", truncBody(body))
	}
	if _, comps, err := f.st.GetItem(ctx, blade); err != nil || len(comps) != 1 || comps[0] != hammer {
		t.Fatalf("stored recipe = %v, err %v", comps, err)
	}
}

func TestItemUpdate_Redirect422AndSaveFlow(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	path := "/items/" + strconv.FormatInt(f.itemID, 10)

	// Unknown id bails before validation.
	rec := f.post(t, "POST", "/items/999", "name=phantom&tier=1", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/items" {
		t.Fatalf("missing id status = %d location = %q, want 303 /items", rec.Code, rec.Header().Get("Location"))
	}

	// Empty name rerenders the edit page with the copy and typed fields.
	rec = f.post(t, "POST", path, "name=&tier=3&effect=zap", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty name status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "name is required.") || !strings.Contains(body, `value="zap"`) {
		t.Fatalf("empty name rerender must carry the copy and typed effect, got %s", truncBody(body))
	}

	// Renaming onto a taken name refuses and leaves the row alone.
	itemSpare(t, f, "ember lance")
	rec = f.post(t, "POST", path, "name=ember lance&tier=3&effect=zap", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("taken rename status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body = rec.Body.String()
	if !strings.Contains(body, "that name is taken.") || !strings.Contains(body, `value="ember lance"`) {
		t.Fatalf("taken rerender must carry the conflict copy and typed name, got %s", truncBody(body))
	}
	if it, _, err := f.st.GetItem(ctx, f.itemID); err != nil || it.Name != "storm core" {
		t.Fatalf("refused rename must not touch the row, got %+v err %v", it, err)
	}

	// Success rewrites the fields and the recipe.
	hammer := itemSpare(t, f, "hammer")
	rec = f.post(t, "POST", path, "name=storm core mk2&tier=4&effect=zap&components="+strconv.FormatInt(hammer, 10), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/items" {
		t.Fatalf("update status = %d location = %q, want 303 /items: %s", rec.Code, rec.Header().Get("Location"), truncBody(rec.Body.String()))
	}
	it, comps, err := f.st.GetItem(ctx, f.itemID)
	if err != nil || it.Name != "storm core mk2" || it.Tier != 4 || it.Effect != "zap" {
		t.Fatalf("updated row = %+v, err %v", it, err)
	}
	if len(comps) != 1 || comps[0] != hammer {
		t.Fatalf("updated recipe = %v, want [%d]", comps, hammer)
	}
}

func TestItemDelete_InUseRefusedWithConflictBanner(t *testing.T) {
	f := seedEditor(t)
	// The fixture lineup slot holds storm core, so history refuses the delete.
	rec := postDeleteItem(t, f, f.itemID, true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, truncBody(rec.Body.String()))
	}
	if !strings.Contains(rec.Body.String(), "existing matches keep their history.") {
		t.Fatalf("body must carry the conflict copy, got %s", truncBody(rec.Body.String()))
	}
	if _, _, err := f.st.GetItem(context.Background(), f.itemID); err != nil {
		t.Fatalf("item must survive the refused delete: %v", err)
	}
}

func TestItemDelete_SuccessForPlainAndHXClients(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	gem := itemSpare(t, f, "shiny gem")
	rec := postDeleteItem(t, f, gem, false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/items" {
		t.Fatalf("plain delete status = %d location = %q, want 303 /items", rec.Code, rec.Header().Get("Location"))
	}
	if _, _, err := f.st.GetItem(ctx, gem); err == nil {
		t.Fatal("plain delete must remove the row")
	}
	plate := itemSpare(t, f, "iron plate")
	rec = postDeleteItem(t, f, plate, true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("hx delete status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); got != "/items" {
		t.Fatalf("HX-Redirect = %q, want /items", got)
	}
	if _, _, err := f.st.GetItem(ctx, plate); err == nil {
		t.Fatal("hx delete must remove the row")
	}
}

func TestItemCreate_422PreservesTypedValues(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/items", "name=&tier=2&effect=glow", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty name status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "name is required.") || !strings.Contains(body, `value="glow"`) {
		t.Fatalf("empty name rerender must carry the copy and typed effect, got %s", truncBody(body))
	}
	// A taken name rerenders with the conflict copy and the typed value.
	rec = f.post(t, "POST", "/items", "name=storm core&tier=2&effect=glow", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("taken name status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body = rec.Body.String()
	if !strings.Contains(body, "that name is taken.") || !strings.Contains(body, `value="storm core"`) {
		t.Fatalf("taken rerender must carry the conflict copy and typed name, got %s", truncBody(body))
	}
	items, err := f.st.ListItems(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("refused creates must not add rows, items = %+v err %v", items, err)
	}
}

func TestItemCreate_GarbageComponent422(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	hammer := itemSpare(t, f, "hammer")
	// A lone garbage value refuses the whole create.
	rec := f.post(t, "POST", "/items", "name=x&tier=1&effect=y&components=abc", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("garbage component status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if want := "pick components from the list."; strings.Count(body, want) != 1 {
		t.Fatalf("components error must appear exactly once, got %s", truncBody(body))
	}
	if !strings.Contains(body, `value="x"`) {
		t.Fatalf("garbage component rerender must keep the typed name, got %s", truncBody(body))
	}
	// A valid id next to the garbage must not sneak the row through either.
	rec = f.post(t, "POST", "/items", "name=frost shard&tier=2&effect=chill&components=abc&components="+strconv.FormatInt(hammer, 10), false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mixed component status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	if !strings.Contains(rec.Body.String(), "pick components from the list.") {
		t.Fatalf("mixed garbage must still name the components field, got %s", truncBody(rec.Body.String()))
	}
	items, err := f.st.ListItems(ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	for _, it := range items {
		if it.Name == "frost shard" || it.Name == "x" {
			t.Fatalf("refused creates must not add rows, got %+v", items)
		}
	}
}

func TestItemUpdate_GarbageComponent422(t *testing.T) {
	f := seedEditor(t)
	ctx := context.Background()
	path := "/items/" + strconv.FormatInt(f.itemID, 10)
	rec := f.post(t, "POST", path, "name=renamed core&tier=3&effect=zap&components=abc", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("garbage component status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if want := "pick components from the list."; strings.Count(body, want) != 1 {
		t.Fatalf("components error must appear exactly once, got %s", truncBody(body))
	}
	if !strings.Contains(body, `value="renamed core"`) {
		t.Fatalf("garbage component rerender must keep the typed name, got %s", truncBody(body))
	}
	if !strings.Contains(body, `<select name="components" multiple size="4" data-autofocus>`) {
		t.Fatalf("components select must carry data-autofocus, got %s", truncBody(body))
	}
	// The stored row and its recipe stay untouched.
	if it, _, err := f.st.GetItem(ctx, f.itemID); err != nil || it.Name != "storm core" {
		t.Fatalf("refused update must not touch the row, got %+v err %v", it, err)
	}
}

// A negative component id parses but names no real option, so it must land on
// the components field, not the name-taken copy.
func TestItemCreate_NegativeComponent422(t *testing.T) {
	f := seedEditor(t)
	rec := f.post(t, "POST", "/items", "name=voodoo&tier=2&effect=curse&components=-3", false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("negative component status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if want := "pick components from the list."; strings.Count(body, want) != 1 {
		t.Fatalf("components error must appear exactly once, got %s", truncBody(body))
	}
	if strings.Contains(body, "that name is taken.") {
		t.Fatalf("a components problem must not claim the name is taken, got %s", truncBody(body))
	}
	if !strings.Contains(body, `value="voodoo"`) {
		t.Fatalf("rerender must keep the typed name, got %s", truncBody(body))
	}
}

// staleComponentId creates three items and deletes the middle one, so its id
// cannot be recycled by the next insert and stays genuinely missing.
func staleComponentId(t *testing.T, f editorFixture) int64 {
	t.Helper()
	ctx := context.Background()
	var ids []int64
	for _, name := range []string{"probe a", "probe b", "probe c"} {
		id, err := f.st.CreateItem(ctx, domain.Item{Name: name, Tier: 1, Effect: "x"}, nil)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, id)
	}
	if err := f.st.DeleteItem(ctx, ids[1]); err != nil {
		t.Fatalf("delete middle item: %v", err)
	}
	if _, err := f.st.CreateItem(ctx, domain.Item{Name: "probe d", Tier: 1, Effect: "x"}, nil); err != nil {
		t.Fatalf("tail item: %v", err)
	}
	return ids[1]
}

// A component id that pointed at a since-deleted item hits the recipe FK; the
// 422 must name components, on both create and update.
func TestItemCreate_StaleComponent422(t *testing.T) {
	f := seedEditor(t)
	x := staleComponentId(t, f)
	form := "name=ward&tier=1&effect=guard&components=" + strconv.FormatInt(x, 10)
	rec := f.post(t, "POST", "/items", form, false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("stale component status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if want := "pick components from the list."; strings.Count(body, want) != 1 {
		t.Fatalf("components error must appear exactly once, got %s", truncBody(body))
	}
	if strings.Contains(body, "that name is taken.") {
		t.Fatalf("a stale component must not claim the name is taken, got %s", truncBody(body))
	}
}

func TestItemUpdate_StaleComponent422(t *testing.T) {
	f := seedEditor(t)
	x := staleComponentId(t, f)
	path := "/items/" + strconv.FormatInt(f.itemID, 10)
	form := "name=renamed core&tier=3&effect=zap&components=" + strconv.FormatInt(x, 10)
	rec := f.post(t, "POST", path, form, false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("stale component update status = %d, want 422: %s", rec.Code, truncBody(rec.Body.String()))
	}
	body := rec.Body.String()
	if want := "pick components from the list."; strings.Count(body, want) != 1 {
		t.Fatalf("components error must appear exactly once, got %s", truncBody(body))
	}
	if strings.Contains(body, "that name is taken.") {
		t.Fatalf("a stale component must not claim the name is taken, got %s", truncBody(body))
	}
	if it, _, err := f.st.GetItem(context.Background(), f.itemID); err != nil || it.Name != "storm core" {
		t.Fatalf("refused update must not touch the row, got %+v err %v", it, err)
	}
}
