package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// adminEntity parameterizes the relic, patch, and pro admin flows. The three
// handler trios in codex.go are textually identical apart from field names,
// copy, and store calls, so one table drives every case.
type adminEntity struct {
	plural   string
	field    string   // the required and conflict-checked form field
	required string   // copy for an empty field
	taken    string   // copy for a duplicate name or version
	kept     []string // value attributes that must survive an empty-field rerender
	seed     func(t *testing.T, f *editorFixture) int64
	nameOf   func(t *testing.T, f *editorFixture, id int64) string
	spare    func(t *testing.T, f *editorFixture, name string) int64
	form     func(name string) string
	pin      func(t *testing.T, f *editorFixture, id int64)
	gone     func(t *testing.T, f *editorFixture, id int64)
}

func codexAdminEntities() []adminEntity {
	return []adminEntity{
		{
			plural: "relics", field: "name",
			required: "name is required.", taken: "that name is taken.",
			kept: []string{`value="ember"`},
			seed: func(t *testing.T, f *editorFixture) int64 {
				t.Helper()
				return f.relicID
			},
			nameOf: func(t *testing.T, f *editorFixture, id int64) string {
				t.Helper()
				r, err := f.st.GetRelic(context.Background(), id)
				if err != nil {
					t.Fatalf("relic %d must load: %v", id, err)
				}
				return r.Name
			},
			spare: func(t *testing.T, f *editorFixture, name string) int64 {
				t.Helper()
				id, err := f.st.CreateRelic(context.Background(), domain.Relic{Name: name, Effect: "ember"})
				if err != nil {
					t.Fatalf("spare relic: %v", err)
				}
				return id
			},
			form: func(name string) string { return "name=" + name + "&effect=ember" },
			pin: func(t *testing.T, f *editorFixture, id int64) {
				t.Helper()
				if _, _, err := f.entry.AddLineup(context.Background(), f.matchID, domain.AddLineupCmd{
					Label: "relic run", Placement: 2, RelicIDs: []int64{id},
				}, 0); err != nil {
					t.Fatalf("pin relic to history: %v", err)
				}
			},
			gone: func(t *testing.T, f *editorFixture, id int64) {
				t.Helper()
				if _, err := f.st.GetRelic(context.Background(), id); !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("relic %d must be gone, err = %v", id, err)
				}
			},
		},
		{
			plural: "patches", field: "version",
			required: "version is required.", taken: "that version is taken.",
			kept: []string{`value="2026-11-01"`},
			seed: func(t *testing.T, f *editorFixture) int64 {
				t.Helper()
				ps, err := f.st.ListPatches(context.Background())
				if err != nil {
					t.Fatalf("list patches: %v", err)
				}
				for _, p := range ps {
					if p.Version == "8.0" {
						return p.ID
					}
				}
				t.Fatal("fixture patch 8.0 missing")
				return 0
			},
			nameOf: func(t *testing.T, f *editorFixture, id int64) string {
				t.Helper()
				p, err := f.st.GetPatch(context.Background(), id)
				if err != nil {
					t.Fatalf("patch %d must load: %v", id, err)
				}
				return p.Version
			},
			spare: func(t *testing.T, f *editorFixture, name string) int64 {
				t.Helper()
				id, err := f.st.CreatePatch(context.Background(), domain.Patch{Version: name, ReleasedAt: "2026-11-01"})
				if err != nil {
					t.Fatalf("spare patch: %v", err)
				}
				return id
			},
			form: func(name string) string { return "version=" + name + "&released_at=2026-11-01" },
			// The fixture match already references 8.0, so the seeded patch is pinned.
			pin: func(t *testing.T, f *editorFixture, id int64) {},
			gone: func(t *testing.T, f *editorFixture, id int64) {
				t.Helper()
				if _, err := f.st.GetPatch(context.Background(), id); !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("patch %d must be gone, err = %v", id, err)
				}
			},
		},
		{
			plural: "pros", field: "name",
			required: "name is required.", taken: "that name is taken.",
			kept: []string{`value="nov"`, `value="grandmaster"`},
			seed: func(t *testing.T, f *editorFixture) int64 {
				t.Helper()
				id, err := f.st.CreatePro(context.Background(), domain.Pro{Name: "apex drift", Handle: "drift", PeakRank: "challenger"})
				if err != nil {
					t.Fatalf("seed pro: %v", err)
				}
				return id
			},
			nameOf: func(t *testing.T, f *editorFixture, id int64) string {
				t.Helper()
				p, err := f.st.GetPro(context.Background(), id)
				if err != nil {
					t.Fatalf("pro %d must load: %v", id, err)
				}
				return p.Name
			},
			spare: func(t *testing.T, f *editorFixture, name string) int64 {
				t.Helper()
				id, err := f.st.CreatePro(context.Background(), domain.Pro{Name: name, Handle: "nov", PeakRank: "grandmaster"})
				if err != nil {
					t.Fatalf("spare pro: %v", err)
				}
				return id
			},
			form: func(name string) string { return "name=" + name + "&handle=nov&peak_rank=grandmaster" },
			pin: func(t *testing.T, f *editorFixture, id int64) {
				t.Helper()
				if _, _, err := f.entry.AddLineup(context.Background(), f.matchID, domain.AddLineupCmd{
					Label: "credit run", Placement: 2, ProID: &id,
				}, 0); err != nil {
					t.Fatalf("pin pro to history: %v", err)
				}
			},
			gone: func(t *testing.T, f *editorFixture, id int64) {
				t.Helper()
				if _, err := f.st.GetPro(context.Background(), id); !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("pro %d must be gone, err = %v", id, err)
				}
			},
		},
	}
}

// clip bounds failure output so a full page dump never floods the test log.
func clip(body string) string {
	if len(body) > 400 {
		return body[:400]
	}
	return body
}

func TestCodexAdminEntityFlows(t *testing.T) {
	for _, e := range codexAdminEntities() {
		t.Run(e.plural, func(t *testing.T) {
			idPath := func(id int64) string {
				return "/" + e.plural + "/" + strconv.FormatInt(id, 10)
			}
			del := func(t *testing.T, f editorFixture, id int64, hx bool) *httptest.ResponseRecorder {
				t.Helper()
				req := httptest.NewRequest(http.MethodDelete, idPath(id), nil)
				req.Header.Set("Origin", "http://example.com")
				if hx {
					req.Header.Set("HX-Request", "true")
				}
				rec := httptest.NewRecorder()
				f.h.ServeHTTP(rec, req)
				return rec
			}

			t.Run("edit missing id redirects to index", func(t *testing.T) {
				f := seedEditor(t)
				rec := f.post(t, "GET", idPath(9999), "", false)
				if rec.Code != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303: %s", rec.Code, clip(rec.Body.String()))
				}
				if loc := rec.Header().Get("Location"); loc != "/"+e.plural {
					t.Fatalf("location = %q, want /%s", loc, e.plural)
				}
			})

			t.Run("update missing id redirects to index", func(t *testing.T) {
				f := seedEditor(t)
				rec := f.post(t, "POST", idPath(9999), e.form("ghost"), false)
				if rec.Code != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303: %s", rec.Code, clip(rec.Body.String()))
				}
				if loc := rec.Header().Get("Location"); loc != "/"+e.plural {
					t.Fatalf("location = %q, want /%s", loc, e.plural)
				}
			})

			t.Run("edit existing renders prefilled form", func(t *testing.T) {
				f := seedEditor(t)
				id := e.seed(t, &f)
				name := e.nameOf(t, &f, id)
				rec := f.post(t, "GET", idPath(id), "", false)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200: %s", rec.Code, clip(rec.Body.String()))
				}
				body := rec.Body.String()
				if !strings.Contains(body, "<h1>"+name+"</h1>") {
					t.Fatalf("edit page must show the row, got %s", clip(body))
				}
				if !strings.Contains(body, `value="`+name+`"`) {
					t.Fatalf("edit form must prefill the %s input, got %s", e.field, clip(body))
				}
			})

			t.Run("update empty field rerenders 422", func(t *testing.T) {
				f := seedEditor(t)
				id := e.seed(t, &f)
				before := e.nameOf(t, &f, id)
				rec := f.post(t, "POST", idPath(id), e.form(""), false)
				if rec.Code != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422: %s", rec.Code, clip(rec.Body.String()))
				}
				body := rec.Body.String()
				if got := strings.Count(body, e.required); got != 1 {
					t.Fatalf("%s must render once on the edit page, got %d: %s", e.required, got, clip(body))
				}
				for _, kept := range e.kept {
					if !strings.Contains(body, kept) {
						t.Fatalf("rerender must keep %s, got %s", kept, clip(body))
					}
				}
				if now := e.nameOf(t, &f, id); now != before {
					t.Fatalf("refused update must not rename the row, got %q want %q", now, before)
				}
			})

			t.Run("update taken name rerenders 422", func(t *testing.T) {
				f := seedEditor(t)
				takenID := e.seed(t, &f)
				sp := e.spare(t, &f, "spare row")
				target := e.nameOf(t, &f, takenID)
				rec := f.post(t, "POST", idPath(sp), e.form(target), false)
				if rec.Code != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422: %s", rec.Code, clip(rec.Body.String()))
				}
				body := rec.Body.String()
				if got := strings.Count(body, e.taken); got != 1 {
					t.Fatalf("%s must render exactly once, got %d: %s", e.taken, got, clip(body))
				}
				if !strings.Contains(body, `value="`+target+`" data-autofocus`) {
					t.Fatalf("input must keep the typed %s and take focus, got %s", e.field, clip(body))
				}
				if now := e.nameOf(t, &f, sp); now != "spare row" {
					t.Fatalf("refused rename must keep the spare row, got %q", now)
				}
			})

			t.Run("update success persists new name", func(t *testing.T) {
				f := seedEditor(t)
				id := e.seed(t, &f)
				rec := f.post(t, "POST", idPath(id), e.form("renamed row"), false)
				if rec.Code != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303: %s", rec.Code, clip(rec.Body.String()))
				}
				if loc := rec.Header().Get("Location"); loc != "/"+e.plural {
					t.Fatalf("location = %q, want /%s", loc, e.plural)
				}
				if now := e.nameOf(t, &f, id); now != "renamed row" {
					t.Fatalf("update must persist the new name, got %q", now)
				}
			})

			t.Run("delete in use renders conflict 409", func(t *testing.T) {
				f := seedEditor(t)
				id := e.seed(t, &f)
				e.pin(t, &f, id)
				rec := del(t, f, id, true)
				if rec.Code != http.StatusConflict {
					t.Fatalf("status = %d, want 409: %s", rec.Code, clip(rec.Body.String()))
				}
				body := rec.Body.String()
				if !strings.Contains(body, domain.ErrInUse.Error()) {
					t.Fatalf("body must carry the conflict copy, got %s", clip(body))
				}
				if !strings.Contains(body, "<h1>"+e.nameOf(t, &f, id)+"</h1>") {
					t.Fatalf("409 must rerender the edit page for the kept row, got %s", clip(body))
				}
			})

			t.Run("delete plain redirects and drops row", func(t *testing.T) {
				f := seedEditor(t)
				id := e.spare(t, &f, "spare row")
				rec := del(t, f, id, false)
				if rec.Code != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303: %s", rec.Code, clip(rec.Body.String()))
				}
				if loc := rec.Header().Get("Location"); loc != "/"+e.plural {
					t.Fatalf("location = %q, want /%s", loc, e.plural)
				}
				e.gone(t, &f, id)
			})

			t.Run("delete hx sends redirect header", func(t *testing.T) {
				f := seedEditor(t)
				id := e.spare(t, &f, "spare row")
				rec := del(t, f, id, true)
				if rec.Code != http.StatusNoContent {
					t.Fatalf("status = %d, want 204: %s", rec.Code, clip(rec.Body.String()))
				}
				if got := rec.Header().Get("HX-Redirect"); got != "/"+e.plural {
					t.Fatalf("HX-Redirect = %q, want /%s", got, e.plural)
				}
				e.gone(t, &f, id)
			})

			t.Run("create success redirects and persists", func(t *testing.T) {
				f := seedEditor(t)
				rec := f.post(t, "POST", "/"+e.plural, e.form("fresh row"), false)
				if rec.Code != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303: %s", rec.Code, clip(rec.Body.String()))
				}
				if loc := rec.Header().Get("Location"); loc != "/"+e.plural {
					t.Fatalf("location = %q, want /%s", loc, e.plural)
				}
				dup := f.post(t, "POST", "/"+e.plural, e.form("fresh row"), false)
				if dup.Code != http.StatusUnprocessableEntity {
					t.Fatalf("repeat create status = %d, want 422: %s", dup.Code, clip(dup.Body.String()))
				}
			})

			t.Run("create duplicate rerenders 422", func(t *testing.T) {
				f := seedEditor(t)
				name := e.nameOf(t, &f, e.seed(t, &f))
				rec := f.post(t, "POST", "/"+e.plural, e.form(name), false)
				if rec.Code != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422: %s", rec.Code, clip(rec.Body.String()))
				}
				body := rec.Body.String()
				if got := strings.Count(body, e.taken); got != 1 {
					t.Fatalf("%s must render once on the index page, got %d: %s", e.taken, got, clip(body))
				}
				if !strings.Contains(body, `value="`+name+`" data-autofocus`) {
					t.Fatalf("create input must keep the typed value and take focus, got %s", clip(body))
				}
			})
		})
	}
}
