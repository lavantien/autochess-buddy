package httpapi

import (
	"errors"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// editorPage renders the lineup editor; finalized matches send the visitor to the
// read-only detail page instead.
func (s *Server) editorPage(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	view, err := s.entry.Editor(r.Context(), id)
	if err != nil {
		stubPage(s.log, "matches", "matches", "no matches yet. start one from the game you just finished.").ServeHTTP(w, r)
		return
	}
	if view.Match.FinalizedAt > 0 {
		http.Redirect(w, r, "/matches/"+r.PathValue("id"), http.StatusSeeOther)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.EditorPage(view, ""))
}

// finalize is the terminal plain post: success answers 303 to the detail page,
// rule failure rerenders the editor with the banner naming the problem.
func (s *Server) finalize(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	err := s.entry.FinalizeMatch(r.Context(), id)
	if err == nil {
		http.Redirect(w, r, "/matches/"+r.PathValue("id"), http.StatusSeeOther)
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		s.mutationFallback(w, r)
		return
	}
	view, verr := s.entry.Editor(r.Context(), id)
	if verr != nil {
		s.mutationFallback(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.EditorPage(view, err.Error()))
}
