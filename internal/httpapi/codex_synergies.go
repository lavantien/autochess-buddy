package httpapi

import (
	"context"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// synergyIndex renders races and classes with their tier ladders.
func (s *Server) synergyIndex(w http.ResponseWriter, r *http.Request) {
	races, err := s.ladderEntries(r, "races")
	if err != nil {
		stubPage(s.log, "codex", "races", ui.CodexEmpty("races")).ServeHTTP(w, r)
		return
	}
	classes := loadList(s.log, r.Context(), "classes", func(ctx context.Context) ([]ui.LadderEntry, error) {
		return s.ladderEntries(r, "classes")
	})
	renderPage(s.log, w, r, http.StatusOK, ui.SynergiesPage(races, classes, ui.NewCodexForm()))
}

func (s *Server) ladderEntries(r *http.Request, entity string) ([]ui.LadderEntry, error) {
	ctx := r.Context()
	if entity == "races" {
		list, err := s.st.ListRaces(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]ui.LadderEntry, len(list))
		for i := range list {
			_, tiers, err := s.st.GetRace(ctx, list[i].ID)
			if err != nil {
				return nil, err
			}
			out[i] = ui.LadderEntry{ID: list[i].ID, Name: list[i].Name, Tiers: tiers}
		}
		return out, nil
	}
	list, err := s.st.ListClasses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ui.LadderEntry, len(list))
	for i := range list {
		_, tiers, err := s.st.GetClass(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		out[i] = ui.LadderEntry{ID: list[i].ID, Name: list[i].Name, Tiers: tiers}
	}
	return out, nil
}

// synergyNameTaken reports whether another race or class already holds name;
// skip excludes the entity being renamed so keeping its own name stays legal.
// On a load failure it reports false: the store's unique constraint then
// refuses the write and the error surfaces through mutationFallback.
func (s *Server) synergyNameTaken(r *http.Request, entity, name string, skip int64) bool {
	if entity == "races" {
		list, err := s.st.ListRaces(r.Context())
		if err != nil {
			s.log.Warn("load races for name check", "err", err)
			return false
		}
		for _, rc := range list {
			if rc.ID != skip && rc.Name == name {
				return true
			}
		}
		return false
	}
	list, err := s.st.ListClasses(r.Context())
	if err != nil {
		s.log.Warn("load classes for name check", "err", err)
		return false
	}
	for _, cl := range list {
		if cl.ID != skip && cl.Name == name {
			return true
		}
	}
	return false
}

// synergyCreate adds a race or class from the synergies tab.
func (s *Server) synergyCreate(w http.ResponseWriter, r *http.Request, entity string) {
	f := parseForm(r)
	name := f.str("name")
	nameKey := entity + "-name"
	if name == "" {
		s.synergy422(w, r, nameKey, "name is required.", nil)
		return
	}
	if s.synergyNameTaken(r, entity, name, 0) {
		s.synergy422(w, r, nameKey, "that name is taken.", map[string]string{nameKey: name})
		return
	}
	if entity == "races" {
		if _, err := s.st.CreateRace(r.Context(), domain.Race{Name: name}, nil); err != nil {
			s.mutationFallback(w, r, err)
			return
		}
	} else if _, err := s.st.CreateClass(r.Context(), domain.Class{Name: name}, nil); err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	http.Redirect(w, r, "/races", http.StatusSeeOther)
}

// synergyUpdate dispatches on mode: rename, add or replace one tier, or delete
// one tier. Errors land under the ladder that issued them, keyed by entity id.
func (s *Server) synergyUpdate(w http.ResponseWriter, r *http.Request, entity string) {
	id := pathID(r, "id")
	f := parseForm(r)
	nameKey := ui.LadderKey(entity, id, "name")
	tierKey := ui.LadderKey(entity, id, "tier")
	var current []domain.Tier
	var name string
	if entity == "races" {
		rc, tiers, err := s.st.GetRace(r.Context(), id)
		if err != nil {
			http.Redirect(w, r, "/races", http.StatusSeeOther)
			return
		}
		current, name = tiers, rc.Name
	} else {
		cl, tiers, err := s.st.GetClass(r.Context(), id)
		if err != nil {
			http.Redirect(w, r, "/races", http.StatusSeeOther)
			return
		}
		current, name = tiers, cl.Name
	}
	switch f.str("mode") {
	case "save_name":
		typed := f.str("name")
		if typed == "" {
			s.synergy422(w, r, nameKey, "name is required.", nil)
			return
		}
		if s.synergyNameTaken(r, entity, typed, id) {
			s.synergy422(w, r, nameKey, "that name is taken.", map[string]string{nameKey: typed})
			return
		}
		name = typed
	case "save_tier":
		count := f.int("count")
		if count < 1 {
			s.synergy422(w, r, tierKey, name+" tier count must be at least 1.", map[string]string{
				tierKey + "-count":  f.str("count"),
				tierKey + "-effect": f.str("effect"),
			})
			return
		}
		replaced := false
		for i := range current {
			if current[i].Count == count {
				current[i].Effect = f.str("effect")
				replaced = true
			}
		}
		if !replaced {
			current = append(current, domain.Tier{LineageID: id, Count: count, Effect: f.str("effect")})
		}
	case "delete_tier":
		count := f.int("count")
		kept := current[:0]
		for _, t := range current {
			if t.Count != count {
				kept = append(kept, t)
			}
		}
		current = kept
	default:
		s.synergy422(w, r, tierKey, "unknown action.", nil)
		return
	}
	var err error
	if entity == "races" {
		err = s.st.UpdateRace(r.Context(), domain.Race{ID: id, Name: name}, current)
	} else {
		err = s.st.UpdateClass(r.Context(), domain.Class{ID: id, Name: name}, current)
	}
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	http.Redirect(w, r, "/races", http.StatusSeeOther)
}

// synergy422 rerenders the synergies page with the error under its ladder and
// the typed values preserved under the keys the template binds.
func (s *Server) synergy422(w http.ResponseWriter, r *http.Request, field, msg string, values map[string]string) {
	races, err := s.ladderEntries(r, "races")
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	classes, err := s.ladderEntries(r, "classes")
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	st := ui.NewCodexForm()
	for k, v := range values {
		st.Values[k] = v
	}
	st.Errs = append(st.Errs, domain.FieldError{Field: field, Msg: msg})
	renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.SynergiesPage(races, classes, st))
}
