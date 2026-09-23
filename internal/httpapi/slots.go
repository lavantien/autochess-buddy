package httpapi

import (
	"fmt"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// addSlot appends a hero cell: the response redraws the grid and resets the hero
// form with its input focused.
func (s *Server) addSlot(w http.ResponseWriter, r *http.Request) {
	lid := pathID(r, "id")
	f := parseForm(r)
	view, err := s.entry.Editor(r.Context(), f.int64("match_id"))
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	l := lineupByID(view, lid)
	if l == nil {
		s.mutationFallback(w, r, fmt.Errorf("lineup %d not in view", lid))
		return
	}
	name := f.str("hero")
	stars := f.int("stars")
	if stars == 0 {
		stars = 2
	}
	fail := func(msg string, code int) {
		if !isHX(r) {
			http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
			return
		}
		renderOOB(s.log, w, r, code, ui.HeroForm(view, *l, ui.HeroFormState{Hero: name, Stars: f.str("stars"), Err: msg}, true))
	}
	hero, err := s.st.GetHeroByName(r.Context(), name)
	if err != nil {
		fail(domain.ErrNoHeroNamed(name).Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := domain.ValidateSlotStars(stars); err != nil {
		fail(err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := domain.ValidateAddSlot(l.Slots); err != nil {
		fail(err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if _, err := s.st.AddSlot(r.Context(), lid, hero.ID, stars); err != nil {
		fail(err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
		return
	}
	fresh, err := s.entry.Editor(r.Context(), view.Match.ID)
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	if fl := lineupByID(fresh, lid); fl != nil {
		renderOOB(s.log, w, r, http.StatusOK,
			ui.GridRegion(fresh, *fl, ui.SlotFormState{}, true),
			ui.HeroForm(fresh, *fl, ui.HeroFormState{Reset: true}, true))
		return
	}
	s.mutationFallback(w, r, err)
}

// saveStars rewrites one slot's stars; the response redraws the grid.
func (s *Server) saveStars(w http.ResponseWriter, r *http.Request) {
	sid := pathID(r, "id")
	f := parseForm(r)
	view, l := s.viewForSlot(w, r, f.int64("match_id"), sid)
	if l == nil {
		return
	}
	stars := f.int("stars")
	if err := domain.ValidateSlotStars(stars); err != nil {
		s.gridError(w, r, view, l, sid, "stars", err.Error())
		return
	}
	if err := s.st.SetSlotStars(r.Context(), sid, stars); err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	s.gridOK(w, r, view.Match.ID, l.ID, ui.SlotFormState{OpenSlotID: sid, FocusSlotID: sid, FocusField: "details"})
}

// attachItem adds an item to a slot; the response redraws the grid with that
// slot's details rendered open and the item select focused.
func (s *Server) attachItem(w http.ResponseWriter, r *http.Request) {
	sid := pathID(r, "id")
	f := parseForm(r)
	view, l := s.viewForSlot(w, r, f.int64("match_id"), sid)
	if l == nil {
		return
	}
	itemID := f.int64("item")
	held := make([]int64, 0, 6)
	for _, sl := range l.Slots {
		if sl.ID == sid {
			for _, it := range sl.Items {
				held = append(held, it.ID)
			}
		}
	}
	if err := domain.ValidateAddItem(held); err != nil {
		s.gridError(w, r, view, l, sid, "item", err.Error())
		return
	}
	if err := s.st.AddSlotItem(r.Context(), sid, itemID); err != nil {
		s.gridError(w, r, view, l, sid, "item", "pick an item from the list.")
		return
	}
	s.gridOK(w, r, view.Match.ID, l.ID, ui.SlotFormState{OpenSlotID: sid, FocusSlotID: sid, FocusField: "item"})
}

// removeItem detaches one item; the response redraws the grid with the slot's
// details open.
func (s *Server) removeItem(w http.ResponseWriter, r *http.Request) {
	sid := pathID(r, "id")
	itemID := pathID(r, "itemId")
	f := parseForm(r)
	view, l := s.viewForSlot(w, r, f.int64("match_id"), sid)
	if l == nil {
		return
	}
	if err := s.st.RemoveSlotItem(r.Context(), sid, itemID); err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	s.gridOK(w, r, view.Match.ID, l.ID, ui.SlotFormState{OpenSlotID: sid, FocusSlotID: sid, FocusField: "details"})
}

// deleteSlot removes a board cell; the response redraws the grid.
func (s *Server) deleteSlot(w http.ResponseWriter, r *http.Request) {
	sid := pathID(r, "id")
	f := parseForm(r)
	view, l := s.viewForSlot(w, r, f.int64("match_id"), sid)
	if l == nil {
		return
	}
	if err := s.st.DeleteSlot(r.Context(), sid); err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	s.gridOK(w, r, view.Match.ID, l.ID, ui.SlotFormState{})
}

// addRelic attaches a relic to a lineup; the response redraws the relics row.
func (s *Server) addRelic(w http.ResponseWriter, r *http.Request) {
	lid := pathID(r, "id")
	f := parseForm(r)
	view, err := s.entry.Editor(r.Context(), f.int64("match_id"))
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	l := lineupByID(view, lid)
	if l == nil {
		s.mutationFallback(w, r, fmt.Errorf("lineup %d not in view", lid))
		return
	}
	if err := s.st.AddLineupRelic(r.Context(), lid, f.int64("relic")); err != nil {
		s.relicError(w, r, view, l, "pick a relic from the list.")
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
		return
	}
	fresh, err := s.entry.Editor(r.Context(), view.Match.ID)
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	if fl := lineupByID(fresh, lid); fl != nil {
		renderOOB(s.log, w, r, http.StatusOK, ui.RelicsRegion(fresh, *fl, ui.RelicState{Open: true, Focus: true}, true))
		return
	}
	s.mutationFallback(w, r, fmt.Errorf("lineup %d vanished", lid))
}

// removeRelic detaches one relic; the response redraws the relics row with the
// disclosure open.
func (s *Server) removeRelic(w http.ResponseWriter, r *http.Request) {
	lid := pathID(r, "id")
	relicID := pathID(r, "relicId")
	f := parseForm(r)
	view, err := s.entry.Editor(r.Context(), f.int64("match_id"))
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	l := lineupByID(view, lid)
	if l == nil {
		s.mutationFallback(w, r, fmt.Errorf("lineup %d not in view", lid))
		return
	}
	if err := s.st.RemoveLineupRelic(r.Context(), lid, relicID); err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
		return
	}
	fresh, err := s.entry.Editor(r.Context(), view.Match.ID)
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	if fl := lineupByID(fresh, lid); fl != nil {
		renderOOB(s.log, w, r, http.StatusOK, ui.RelicsRegion(fresh, *fl, ui.RelicState{Open: true, FocusSummary: true}, true))
		return
	}
	s.mutationFallback(w, r, fmt.Errorf("lineup %d vanished", lid))
}

// viewForSlot loads the editor view and the lineup owning the slot, answering the
// fallback when either is missing.
func (s *Server) viewForSlot(w http.ResponseWriter, r *http.Request, matchID, slotID int64) (service.EditorView, *domain.Lineup) {
	view, err := s.entry.Editor(r.Context(), matchID)
	if err != nil {
		s.mutationFallback(w, r, err)
		return view, nil
	}
	for i := range view.Lineups {
		for _, sl := range view.Lineups[i].Slots {
			if sl.ID == slotID {
				return view, &view.Lineups[i]
			}
		}
	}
	s.mutationFallback(w, r, fmt.Errorf("slot %d not in match %d", slotID, matchID))
	return view, nil
}

func (s *Server) gridOK(w http.ResponseWriter, r *http.Request, matchID, lineupID int64, st ui.SlotFormState) {
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(matchID)+"/edit", http.StatusSeeOther)
		return
	}
	fresh, err := s.entry.Editor(r.Context(), matchID)
	if err != nil {
		s.mutationFallback(w, r, err)
		return
	}
	if fl := lineupByID(fresh, lineupID); fl != nil {
		renderOOB(s.log, w, r, http.StatusOK, ui.GridRegion(fresh, *fl, st, true))
		return
	}
	s.mutationFallback(w, r, err)
}

func (s *Server) gridError(w http.ResponseWriter, r *http.Request, view service.EditorView, l *domain.Lineup, slotID int64, field, msg string) {
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
		return
	}
	renderOOB(s.log, w, r, http.StatusUnprocessableEntity,
		ui.GridRegion(view, *l, ui.SlotFormState{OpenSlotID: slotID, BadSlotID: slotID, BadField: field, BadMsg: msg}, true))
}

func (s *Server) relicError(w http.ResponseWriter, r *http.Request, view service.EditorView, l *domain.Lineup, msg string) {
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
		return
	}
	renderOOB(s.log, w, r, http.StatusUnprocessableEntity,
		ui.RelicsRegion(view, *l, ui.RelicState{Open: true, Err: msg}, true))
}
