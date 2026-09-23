package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// lineupByID finds one lineup inside a loaded editor view.
func lineupByID(view service.EditorView, id int64) *domain.Lineup {
	for i := range view.Lineups {
		if view.Lineups[i].ID == id {
			return &view.Lineups[i]
		}
	}
	return nil
}

func proIDText(l *domain.Lineup) string {
	if l.ProID == nil {
		return ""
	}
	return itoa(*l.ProID)
}

// resetEditState prefills an edit form from the lineup's stored values.
func resetEditState(l *domain.Lineup) ui.EditFormState {
	return ui.EditFormState{Placement: itoa(int64(l.Placement)), ProID: proIDText(l), Label: l.Label,
		Wins: itoa(int64(l.Wins)), Draws: itoa(int64(l.Draws)), Losses: itoa(int64(l.Losses)), Networth: itoa(int64(l.Networth))}
}

// cardFragment renders one lineup card in its resting state, for swaps that
// replace a single card.
func cardFragment(view service.EditorView, l *domain.Lineup) templ.Component {
	return ui.LineupCard(view, *l, resetEditState(l), ui.HeroFormState{Reset: true}, ui.SlotFormState{}, ui.RelicState{}, "")
}

// addLineup adds or copies a lineup card and answers with cards + pips per the
// swap table (the add form reset rides the cards re-render).
func (s *Server) addLineup(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	f := parseForm(r)
	copyFrom := f.int64("copy_from")
	cmd := domain.AddLineupCmd{
		ProID:     f.optInt64("pro"),
		Label:     f.str("label"),
		Placement: f.int("placement"),
		Wins:      f.int("wins"),
		Draws:     f.int("draws"),
		Losses:    f.int("losses"),
		Networth:  f.int("networth"),
	}
	view, err := s.entry.AddLineup(r.Context(), id, cmd, copyFrom)
	if err != nil {
		s.lineupFormError(w, r, id, copyFrom, err)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+r.PathValue("id")+"/edit", http.StatusSeeOther)
		return
	}
	renderOOB(s.log, w, r, http.StatusOK, ui.CardsRegion(view, ui.ResetAddForm(view), true), ui.PipsRegion(view, true))
}

// lineupFormError renders the issuing form's oob fragment with values preserved
// and inline errors under the first bad field.
func (s *Server) lineupFormError(w http.ResponseWriter, r *http.Request, matchID, copyFrom int64, err error) {
	view, verr := s.entry.Editor(r.Context(), matchID)
	if verr != nil {
		s.mutationFallback(w, r)
		return
	}
	st := ui.AddFormState{
		Placement: r.PostForm.Get("placement"),
		ProID:     r.PostForm.Get("pro"),
		Label:     r.PostForm.Get("label"),
		Wins:      r.PostForm.Get("wins"),
		Draws:     r.PostForm.Get("draws"),
		Losses:    r.PostForm.Get("losses"),
		Networth:  r.PostForm.Get("networth"),
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+r.PathValue("id")+"/edit", http.StatusSeeOther)
		return
	}
	var pc *domain.PlacementConflictError
	var ve domain.ValidationError
	switch {
	case errors.As(err, &pc):
		st.Errs = append(st.Errs, domain.FieldError{Field: "placement", Msg: pc.Error()})
	case errors.As(err, &ve):
		st.Errs = append(st.Errs, ve...)
	default:
		st.Errs = append(st.Errs, domain.FieldError{Field: "form", Msg: err.Error()})
	}
	if copyFrom != 0 {
		// Field-less copy form: the error renders inline beside the button.
		if l := lineupByID(view, copyFrom); l != nil {
			renderOOB(s.log, w, r, http.StatusUnprocessableEntity,
				ui.LineupCard(view, *l, resetEditState(l), ui.HeroFormState{Reset: true}, ui.SlotFormState{}, ui.RelicState{}, err.Error()))
			return
		}
	}
	renderOOB(s.log, w, r, http.StatusUnprocessableEntity, ui.AddForm(view, st, true))
}

// updateLineup saves scalar edits: card + pips when placement held, the whole
// cards region when placement moved because the server owns the sort.
func (s *Server) updateLineup(w http.ResponseWriter, r *http.Request) {
	lid := pathID(r, "id")
	f := parseForm(r)
	mid := f.int64("match_id")
	view, err := s.entry.Editor(r.Context(), mid)
	if err != nil {
		s.mutationFallback(w, r)
		return
	}
	l := lineupByID(view, lid)
	if l == nil {
		s.mutationFallback(w, r)
		return
	}
	placement := f.int("placement")
	var errs domain.ValidationError
	if placement < 1 || placement > 8 {
		errs = append(errs, domain.FieldError{Field: "placement", Msg: "placement must be between 1 and 8."})
	}
	if f.int("wins") < 0 || f.int("draws") < 0 || f.int("losses") < 0 || f.int("networth") < 0 {
		errs = append(errs, domain.FieldError{Field: "record", Msg: "wins, draws, losses and networth cannot be negative."})
	}
	if len(errs) > 0 {
		s.editFormError(w, r, view, l, f, errs)
		return
	}
	for _, other := range view.Lineups {
		if other.ID != l.ID && other.Placement == placement {
			s.editFormError(w, r, view, l, f, domain.ValidationError{{Field: "placement", Msg: (&domain.PlacementConflictError{Placement: placement}).Error()}})
			return
		}
	}
	nl := *l
	nl.ProID = f.optInt64("pro")
	nl.Label = f.str("label")
	nl.Placement = placement
	nl.Wins, nl.Draws, nl.Losses, nl.Networth = f.int("wins"), f.int("draws"), f.int("losses"), f.int("networth")
	if err := s.st.UpdateLineup(r.Context(), nl); err != nil {
		var pc *domain.PlacementConflictError
		if errors.As(err, &pc) {
			s.editFormError(w, r, view, l, f, domain.ValidationError{{Field: "placement", Msg: pc.Error()}})
			return
		}
		s.mutationFallback(w, r)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(mid)+"/edit", http.StatusSeeOther)
		return
	}
	fresh, err := s.entry.Editor(r.Context(), mid)
	if err != nil {
		s.mutationFallback(w, r)
		return
	}
	if placement != l.Placement {
		renderOOB(s.log, w, r, http.StatusOK, ui.CardsRegion(fresh, ui.ResetAddForm(fresh), true), ui.PipsRegion(fresh, true))
		return
	}
	if nl := lineupByID(fresh, lid); nl != nil {
		renderOOB(s.log, w, r, http.StatusOK, cardFragment(fresh, nl), ui.PipsRegion(fresh, true))
		return
	}
	renderOOB(s.log, w, r, http.StatusOK, ui.CardsRegion(fresh, ui.ResetAddForm(fresh), true), ui.PipsRegion(fresh, true))
}

func (s *Server) editFormError(w http.ResponseWriter, r *http.Request, view service.EditorView, l *domain.Lineup, f formValues, errs domain.ValidationError) {
	st := ui.EditFormState{
		Placement: f.str("placement"),
		ProID:     f.str("pro"),
		Label:     f.str("label"),
		Wins:      f.str("wins"),
		Draws:     f.str("draws"),
		Losses:    f.str("losses"),
		Networth:  f.str("networth"),
		Errs:      errs,
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(view.Match.ID)+"/edit", http.StatusSeeOther)
		return
	}
	renderOOB(s.log, w, r, http.StatusUnprocessableEntity, ui.EditForm(view, *l, st, true))
}

// deleteLineup removes a card; the response is cards + pips.
func (s *Server) deleteLineup(w http.ResponseWriter, r *http.Request) {
	lid := pathID(r, "id")
	f := parseForm(r)
	mid := f.int64("match_id")
	if err := s.st.DeleteLineup(r.Context(), lid); err != nil {
		s.mutationFallback(w, r)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/matches/"+itoa(mid)+"/edit", http.StatusSeeOther)
		return
	}
	view, err := s.entry.Editor(r.Context(), mid)
	if err != nil {
		s.mutationFallback(w, r)
		return
	}
	renderOOB(s.log, w, r, http.StatusOK, ui.CardsRegion(view, ui.ResetAddForm(view), true), ui.PipsRegion(view, true))
}
