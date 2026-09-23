package httpapi

import (
	"errors"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// errAnswered signals the handler already wrote its answer.
var errAnswered = errors.New("request already answered")

var heroFormKeys = []string{"name", "cost", "race1", "race2", "class1", "class2", "ability", "notes"}

// heroIndex renders the heroes table with its analytics columns.
func (s *Server) heroIndex(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.ListHeroesWithStats(r.Context())
	if err != nil {
		stubPage(s.log, "codex", "heroes", ui.CodexEmpty("heroes")).ServeHTTP(w, r)
		return
	}
	races := loadList(s.log, r.Context(), "races", s.st.ListRaces)
	classes := loadList(s.log, r.Context(), "classes", s.st.ListClasses)
	renderPage(s.log, w, r, http.StatusOK, ui.HeroesPage(rows, races, classes, ui.NewCodexForm()))
}

// decodeHero turns raw form values into a hero, resolving the lineage selects.
func decodeHero(f formValues) domain.Hero {
	h := domain.Hero{
		Name:    f.str("name"),
		Cost:    f.int("cost"),
		Ability: f.str("ability"),
		Notes:   f.str("notes"),
	}
	if id := f.int64("race1"); id != 0 {
		h.Races = append(h.Races, domain.Race{ID: id})
	}
	if id := f.int64("race2"); id != 0 {
		h.Races = append(h.Races, domain.Race{ID: id})
	}
	if id := f.int64("class1"); id != 0 {
		h.Classes = append(h.Classes, domain.Class{ID: id})
	}
	if id := f.int64("class2"); id != 0 {
		h.Classes = append(h.Classes, domain.Class{ID: id})
	}
	return h
}

func (s *Server) heroCreate(w http.ResponseWriter, r *http.Request) {
	f := parseForm(r)
	st := formState(r, heroFormKeys...)
	h := decodeHero(f)
	if err := s.saveHero(w, r, h, st, 0); err != nil {
		return
	}
	http.Redirect(w, r, "/heroes", http.StatusSeeOther)
}

func (s *Server) heroEdit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	h, err := s.st.GetHero(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/heroes", http.StatusSeeOther)
		return
	}
	races := loadList(s.log, r.Context(), "races", s.st.ListRaces)
	classes := loadList(s.log, r.Context(), "classes", s.st.ListClasses)
	st := ui.NewCodexForm()
	st.Values["name"], st.Values["cost"] = h.Name, itoa(int64(h.Cost))
	st.Values["ability"], st.Values["notes"] = h.Ability, h.Notes
	if len(h.Races) > 0 {
		st.Values["race1"] = itoa(h.Races[0].ID)
	}
	if len(h.Races) > 1 {
		st.Values["race2"] = itoa(h.Races[1].ID)
	}
	if len(h.Classes) > 0 {
		st.Values["class1"] = itoa(h.Classes[0].ID)
	}
	if len(h.Classes) > 1 {
		st.Values["class2"] = itoa(h.Classes[1].ID)
	}
	renderPage(s.log, w, r, http.StatusOK, ui.HeroEditPage(h, races, classes, st, ""))
}

func (s *Server) heroUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	f := parseForm(r)
	if _, err := s.st.GetHero(r.Context(), id); err != nil {
		http.Redirect(w, r, "/heroes", http.StatusSeeOther)
		return
	}
	st := formState(r, heroFormKeys...)
	h := decodeHero(f)
	h.ID = id
	if err := s.saveHero(w, r, h, st, id); err != nil {
		return
	}
	http.Redirect(w, r, "/heroes", http.StatusSeeOther)
}

// saveHero validates and writes, rerendering with errors on failure. It reports
// whether the write landed.
func (s *Server) saveHero(w http.ResponseWriter, r *http.Request, h domain.Hero, st ui.CodexFormState, id int64) error {
	if h.Name == "" {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "name is required."})
	}
	if err := domain.ValidateHero(h); err != nil {
		if err == domain.ErrCostRange {
			st.Errs = append(st.Errs, domain.FieldError{Field: "cost", Msg: err.Error()})
		} else {
			st.Errs = append(st.Errs, domain.FieldError{Field: "lineage", Msg: err.Error()})
		}
	}
	if len(st.Errs) > 0 {
		s.renderHeroError(w, r, h, st, id)
		return errAnswered
	}
	var err error
	if id == 0 {
		_, err = s.st.CreateHero(r.Context(), h)
	} else {
		err = s.st.UpdateHero(r.Context(), h)
	}
	if err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "name", Msg: "that name is taken."})
		s.renderHeroError(w, r, h, st, id)
		return errAnswered
	}
	return nil
}

func (s *Server) renderHeroError(w http.ResponseWriter, r *http.Request, h domain.Hero, st ui.CodexFormState, id int64) {
	races := loadList(s.log, r.Context(), "races", s.st.ListRaces)
	classes := loadList(s.log, r.Context(), "classes", s.st.ListClasses)
	if id == 0 {
		rows := loadList(s.log, r.Context(), "heroes", s.st.ListHeroesWithStats)
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.HeroesPage(rows, races, classes, st))
		return
	}
	renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.HeroEditPage(h, races, classes, st, ""))
}

func (s *Server) heroDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	s.codexDelete(w, r, "heroes", id, func() error {
		h, err := s.st.GetHero(r.Context(), id)
		if err != nil {
			return err
		}
		races := loadList(s.log, r.Context(), "races", s.st.ListRaces)
		classes := loadList(s.log, r.Context(), "classes", s.st.ListClasses)
		renderPage(s.log, w, r, http.StatusConflict, ui.HeroEditPage(h, races, classes, ui.NewCodexForm(), domain.ErrInUse.Error()))
		return nil
	})
}
