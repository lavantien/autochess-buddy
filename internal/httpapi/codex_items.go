package httpapi

import (
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

var itemFormKeys = []string{"name", "tier", "effect"}

func (s *Server) itemIndex(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListItems(r.Context())
	if err != nil {
		stubPage(s.log, "codex", "items", ui.CodexEmpty("items")).ServeHTTP(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.ItemsPage(items, ui.NewCodexForm()))
}

func decodeItem(f formValues, r *http.Request) domain.Item {
	return domain.Item{
		Name:   f.str("name"),
		Tier:   f.int("tier"),
		Effect: f.str("effect"),
	}
}

func (s *Server) itemCreate(w http.ResponseWriter, r *http.Request) {
	f := parseForm(r)
	st := formState(r, itemFormKeys...)
	it := decodeItem(f, r)
	components := componentIDs(r)
	if it.Name == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
	}
	if len(st.Errs) > 0 {
		items, _ := s.st.ListItems(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ItemsPage(items, st))
		return
	}
	if _, err := s.st.CreateItem(r.Context(), it, components); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		items, _ := s.st.ListItems(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ItemsPage(items, st))
		return
	}
	http.Redirect(w, r, "/items", http.StatusSeeOther)
}

func (s *Server) itemEdit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	it, components, err := s.st.GetItem(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/items", http.StatusSeeOther)
		return
	}
	all, _ := s.st.ListItems(r.Context())
	st := ui.NewCodexForm()
	st.Values["name"], st.Values["tier"], st.Values["effect"] = it.Name, itoa(int64(it.Tier)), it.Effect
	for _, c := range components {
		st.Components = append(st.Components, itoa(c))
	}
	renderPage(s.log, w, r, http.StatusOK, ui.ItemEditPage(it, all, st, ""))
}

func (s *Server) itemUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	if _, _, err := s.st.GetItem(r.Context(), id); err != nil {
		http.Redirect(w, r, "/items", http.StatusSeeOther)
		return
	}
	f := parseForm(r)
	st := formState(r, itemFormKeys...)
	it := decodeItem(f, r)
	it.ID = id
	components := componentIDs(r)
	all, _ := s.st.ListItems(r.Context())
	if it.Name == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ItemEditPage(it, all, st, ""))
		return
	}
	if err := s.st.UpdateItem(r.Context(), it, components); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ItemEditPage(it, all, st, ""))
		return
	}
	http.Redirect(w, r, "/items", http.StatusSeeOther)
}

func (s *Server) itemDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	s.codexDelete(w, r, "items", id, func() error {
		it, _, err := s.st.GetItem(r.Context(), id)
		if err != nil {
			return err
		}
		all, _ := s.st.ListItems(r.Context())
		renderPage(s.log, w, r, http.StatusConflict, ui.ItemEditPage(it, all, ui.NewCodexForm(), domain.ErrInUse.Error()))
		return nil
	})
}

// componentIDs reads the recipe multi-select.
func componentIDs(r *http.Request) []int64 {
	vals := r.PostForm["components"]
	ids := make([]int64, 0, len(vals))
	for _, v := range vals {
		if id := parseID(v); id != 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
