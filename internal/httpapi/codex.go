package httpapi

import (
	"errors"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// formState captures raw values so 422 rerenders preserve what was typed.
func formState(r *http.Request, keys ...string) ui.CodexFormState {
	st := ui.NewCodexForm()
	for _, k := range keys {
		st.Values[k] = r.PostForm.Get(k)
	}
	st.Components = r.PostForm["components"]
	return st
}

// codexDelete answers hx-delete with the HX-Redirect header and plain requests
// with a 303; entities referenced by history render the conflict instead.
func (s *Server) codexDelete(w http.ResponseWriter, r *http.Request, entity string, id int64, rerender func() error) {
	err := s.deleteEntity(r, entity, id)
	if errors.Is(err, domain.ErrInUse) {
		if rerender != nil {
			if rerr := rerender(); rerr == nil {
				return
			}
		}
		s.mutationFallback(w, r, nil)
		return
	}
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/"+entity)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/"+entity, http.StatusSeeOther)
}

// relicHandlers covers the plain quartet with no children.
func (s *Server) relicIndex(w http.ResponseWriter, r *http.Request) {
	relics, err := s.st.ListRelics(r.Context())
	if err != nil {
		stubPage(s.log, "codex", "relics", "no relics yet. add the first relic so lineups can reference it.").ServeHTTP(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.RelicsPage(relics, ui.NewCodexForm()))
}

func (s *Server) relicCreate(w http.ResponseWriter, r *http.Request) {
	f := parseForm(r)
	st := formState(r, "name", "effect")
	if f.str("name") == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
		relics, _ := s.st.ListRelics(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.RelicsPage(relics, st))
		return
	}
	if _, err := s.st.CreateRelic(r.Context(), domain.Relic{Name: f.str("name"), Effect: f.str("effect")}); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		relics, _ := s.st.ListRelics(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.RelicsPage(relics, st))
		return
	}
	http.Redirect(w, r, "/relics", http.StatusSeeOther)
}

func (s *Server) relicEdit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	rel, err := s.st.GetRelic(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/relics", http.StatusSeeOther)
		return
	}
	st := ui.NewCodexForm()
	st.Values["name"], st.Values["effect"] = rel.Name, rel.Effect
	renderPage(s.log, w, r, http.StatusOK, ui.RelicEditPage(rel, st, ""))
}

func (s *Server) relicUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	f := parseForm(r)
	st := formState(r, "name", "effect")
	rel, err := s.st.GetRelic(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/relics", http.StatusSeeOther)
		return
	}
	if f.str("name") == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.RelicEditPage(rel, st, ""))
		return
	}
	rel.Name, rel.Effect = f.str("name"), f.str("effect")
	if err := s.st.UpdateRelic(r.Context(), rel); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.RelicEditPage(rel, st, ""))
		return
	}
	http.Redirect(w, r, "/relics", http.StatusSeeOther)
}

func (s *Server) relicDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	s.codexDelete(w, r, "relics", id, func() error {
		rel, err := s.st.GetRelic(r.Context(), id)
		if err != nil {
			return err
		}
		renderPage(s.log, w, r, http.StatusConflict, ui.RelicEditPage(rel, ui.NewCodexForm(), domain.ErrInUse.Error()))
		return nil
	})
}

// patchHandlers and proHandlers follow the same plain shape.
func (s *Server) patchIndex(w http.ResponseWriter, r *http.Request) {
	patches, err := s.st.ListPatches(r.Context())
	if err != nil {
		stubPage(s.log, "codex", "patches", "no patches yet. add the first patch so lineups can reference it.").ServeHTTP(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.PatchesPage(patches, ui.NewCodexForm()))
}

func (s *Server) patchCreate(w http.ResponseWriter, r *http.Request) {
	f := parseForm(r)
	st := formState(r, "version", "released_at")
	if f.str("version") == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "version", Msg: "version is required."})
		patches, _ := s.st.ListPatches(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.PatchesPage(patches, st))
		return
	}
	if _, err := s.st.CreatePatch(r.Context(), domain.Patch{Version: f.str("version"), ReleasedAt: f.str("released_at")}); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "version", Msg: "that version is taken."})
		patches, _ := s.st.ListPatches(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.PatchesPage(patches, st))
		return
	}
	http.Redirect(w, r, "/patches", http.StatusSeeOther)
}

func (s *Server) patchEdit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	p, err := s.st.GetPatch(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/patches", http.StatusSeeOther)
		return
	}
	st := ui.NewCodexForm()
	st.Values["version"], st.Values["released_at"] = p.Version, p.ReleasedAt
	renderPage(s.log, w, r, http.StatusOK, ui.PatchEditPage(p, st, ""))
}

func (s *Server) patchUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	f := parseForm(r)
	st := formState(r, "version", "released_at")
	p, err := s.st.GetPatch(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/patches", http.StatusSeeOther)
		return
	}
	if f.str("version") == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "version", Msg: "version is required."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.PatchEditPage(p, st, ""))
		return
	}
	p.Version, p.ReleasedAt = f.str("version"), f.str("released_at")
	if err := s.st.UpdatePatch(r.Context(), p); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "version", Msg: "that version is taken."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.PatchEditPage(p, st, ""))
		return
	}
	http.Redirect(w, r, "/patches", http.StatusSeeOther)
}

func (s *Server) patchDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	s.codexDelete(w, r, "patches", id, func() error {
		p, err := s.st.GetPatch(r.Context(), id)
		if err != nil {
			return err
		}
		renderPage(s.log, w, r, http.StatusConflict, ui.PatchEditPage(p, ui.NewCodexForm(), domain.ErrInUse.Error()))
		return nil
	})
}

func (s *Server) proIndex(w http.ResponseWriter, r *http.Request) {
	pros, err := s.st.ListPros(r.Context())
	if err != nil {
		stubPage(s.log, "codex", "pros", "no pros yet. add the first pro so lineups can reference it.").ServeHTTP(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.ProsPage(pros, ui.NewCodexForm()))
}

func (s *Server) proCreate(w http.ResponseWriter, r *http.Request) {
	f := parseForm(r)
	st := formState(r, "name", "handle", "peak_rank")
	if f.str("name") == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
		pros, _ := s.st.ListPros(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ProsPage(pros, st))
		return
	}
	if _, err := s.st.CreatePro(r.Context(), domain.Pro{Name: f.str("name"), Handle: f.str("handle"), PeakRank: f.str("peak_rank")}); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		pros, _ := s.st.ListPros(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ProsPage(pros, st))
		return
	}
	http.Redirect(w, r, "/pros", http.StatusSeeOther)
}

func (s *Server) proEdit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	p, err := s.st.GetPro(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/pros", http.StatusSeeOther)
		return
	}
	st := ui.NewCodexForm()
	st.Values["name"], st.Values["handle"], st.Values["peak_rank"] = p.Name, p.Handle, p.PeakRank
	renderPage(s.log, w, r, http.StatusOK, ui.ProEditPage(p, st, ""))
}

func (s *Server) proUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	f := parseForm(r)
	st := formState(r, "name", "handle", "peak_rank")
	p, err := s.st.GetPro(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/pros", http.StatusSeeOther)
		return
	}
	if f.str("name") == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ProEditPage(p, st, ""))
		return
	}
	p.Name, p.Handle, p.PeakRank = f.str("name"), f.str("handle"), f.str("peak_rank")
	if err := s.st.UpdatePro(r.Context(), p); err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.ProEditPage(p, st, ""))
		return
	}
	http.Redirect(w, r, "/pros", http.StatusSeeOther)
}

func (s *Server) proDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	s.codexDelete(w, r, "pros", id, func() error {
		p, err := s.st.GetPro(r.Context(), id)
		if err != nil {
			return err
		}
		renderPage(s.log, w, r, http.StatusConflict, ui.ProEditPage(p, ui.NewCodexForm(), domain.ErrInUse.Error()))
		return nil
	})
}

// deleteEntity dispatches one delete to the store.
func (s *Server) deleteEntity(r *http.Request, entity string, id int64) error {
	ctx := r.Context()
	switch entity {
	case "heroes":
		return s.st.DeleteHero(ctx, id)
	case "items":
		return s.st.DeleteItem(ctx, id)
	case "relics":
		return s.st.DeleteRelic(ctx, id)
	case "patches":
		return s.st.DeletePatch(ctx, id)
	case "pros":
		return s.st.DeletePro(ctx, id)
	case "races":
		return s.st.DeleteRace(ctx, id)
	case "classes":
		return s.st.DeleteClass(ctx, id)
	}
	return domain.ErrNotFound
}
