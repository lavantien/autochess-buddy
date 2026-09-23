package httpapi

import (
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
	classes, _ := s.ladderEntries(r, "classes")
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
		for i, rc := range list {
			_, tiers, err := s.st.GetRace(ctx, rc.ID)
			if err != nil {
				return nil, err
			}
			out[i] = ui.LadderEntry{ID: rc.ID, Name: rc.Name, Tiers: tiers}
		}
		return out, nil
	}
	list, err := s.st.ListClasses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ui.LadderEntry, len(list))
	for i, cl := range list {
		_, tiers, err := s.st.GetClass(ctx, cl.ID)
		if err != nil {
			return nil, err
		}
		out[i] = ui.LadderEntry{ID: cl.ID, Name: cl.Name, Tiers: tiers}
	}
	return out, nil
}

// synergyCreate adds a race or class from the synergies tab.
func (s *Server) synergyCreate(w http.ResponseWriter, r *http.Request, entity string) {
	f := parseForm(r)
	if f.str("name") == "" {
		s.synergy422(w, r, domain.FieldError{Field: entity + "-name", Msg: "name is required."})
		return
	}
	if entity == "races" {
		if _, err := s.st.CreateRace(r.Context(), domain.Race{Name: f.str("name")}, nil); err != nil {
			s.synergy422(w, r, domain.FieldError{Field: entity + "-name", Msg: "that name is taken."})
			return
		}
	} else {
		if _, err := s.st.CreateClass(r.Context(), domain.Class{Name: f.str("name")}, nil); err != nil {
			s.synergy422(w, r, domain.FieldError{Field: entity + "-name", Msg: "that name is taken."})
			return
		}
	}
	http.Redirect(w, r, "/races", http.StatusSeeOther)
}

// synergyUpdate dispatches on mode: rename, add or replace one tier, or delete
// one tier. Tier ops preserve the loaded name.
func (s *Server) synergyUpdate(w http.ResponseWriter, r *http.Request, entity string) {
	id := pathID(r, "id")
	f := parseForm(r)
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
		if f.str("name") == "" {
			s.synergy422(w, r, domain.FieldError{Field: entity + "-name", Msg: "name is required."})
			return
		}
		name = f.str("name")
	case "save_tier":
		count := f.int("count")
		if count < 1 {
			s.synergy422(w, r, domain.FieldError{Field: "tier", Msg: "count must be at least 1."})
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
		s.synergy422(w, r, domain.FieldError{Field: "tier", Msg: "unknown action."})
		return
	}
	var err error
	if entity == "races" {
		err = s.st.UpdateRace(r.Context(), domain.Race{ID: id, Name: name}, current)
	} else {
		err = s.st.UpdateClass(r.Context(), domain.Class{ID: id, Name: name}, current)
	}
	if err != nil {
		s.synergy422(w, r, domain.FieldError{Field: "tier", Msg: "that name is taken."})
		return
	}
	http.Redirect(w, r, "/races", http.StatusSeeOther)
}

func (s *Server) synergy422(w http.ResponseWriter, r *http.Request, fe domain.FieldError) {
	races, err := s.ladderEntries(r, "races")
	if err != nil {
		s.mutationFallback(w, r)
		return
	}
	classes, _ := s.ladderEntries(r, "classes")
	st := ui.NewCodexForm()
	st.Errs = append(st.Errs, fe)
	renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.SynergiesPage(races, classes, st))
}
