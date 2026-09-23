package httpapi

import (
	"errors"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// matchList renders the matches table with its plain GET filters.
func (s *Server) matchList(w http.ResponseWriter, r *http.Request) {
	f := sqlite.MatchFilter{
		Source: r.URL.Query().Get("source"),
		State:  r.URL.Query().Get("state"),
	}
	if version := r.URL.Query().Get("patch"); version != "" {
		patches, err := s.st.ListPatches(r.Context())
		if err == nil {
			for _, p := range patches {
				if p.Version == version {
					f.PatchID = p.ID
					break
				}
			}
		}
	}
	rows, err := s.st.ListMatches(r.Context(), f)
	if err != nil {
		stubPage(s.log, "matches", "matches", "no matches yet. start one from the game you just finished.").ServeHTTP(w, r)
		return
	}
	patches, _ := s.st.ListPatches(r.Context())
	renderPage(s.log, w, r, http.StatusOK, ui.MatchesListPage(rows, f, patches))
}

// newMatchForm renders the create shell page.
func (s *Server) newMatchForm(w http.ResponseWriter, r *http.Request) {
	patches, err := s.st.ListPatches(r.Context())
	if err != nil {
		stubPage(s.log, "matches", "new match", "no patches yet. add the first patch so lineups can reference it.").ServeHTTP(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.NewMatchPage(patches, ui.NewCodexForm()))
}

// createMatch is a plain post: 303 to the editor on success, full-page 422 with
// preserved values on failure.
func (s *Server) createMatch(w http.ResponseWriter, r *http.Request) {
	f := parseForm(r)
	st := ui.NewCodexForm()
	st.Values["patch_id"] = f.str("patch_id")
	st.Values["source"] = f.str("source")
	st.Values["played_at"] = f.str("played_at")
	st.Values["notes"] = f.str("notes")
	if f.str("source") == "" {
		st.Values["source"] = "pro"
	}
	playedAt, err := domain.ParsePlayedAt(f.str("played_at"))
	if err != nil {
		st.Errs = append(st.Errs, domain.FieldError{Field: "played_at", Msg: "played at must be a datetime."})
	}
	m := domain.Match{PatchID: f.int64("patch_id"), PlayedAt: playedAt, Source: st.Values["source"], Notes: f.str("notes")}
	id, cerr := s.entry.CreateMatch(r.Context(), m)
	if cerr != nil && len(st.Errs) == 0 {
		st.Errs = append(st.Errs, domain.FieldError{Field: "patch_id", Msg: "pick a patch from the list."})
	}
	if len(st.Errs) > 0 {
		patches, _ := s.st.ListPatches(r.Context())
		renderPage(s.log, w, r, http.StatusUnprocessableEntity, ui.NewMatchPage(patches, st))
		return
	}
	http.Redirect(w, r, "/matches/"+itoa(id)+"/edit", http.StatusSeeOther)
}

// matchDetail renders the scoreboard strip and read-only boards.
func (s *Server) matchDetail(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	m, lineups, err := s.st.GetMatch(r.Context(), id)
	if err != nil {
		stubPage(s.log, "matches", "matches", "no matches yet. start one from the game you just finished.").ServeHTTP(w, r)
		return
	}
	p, err := s.st.GetPatch(r.Context(), m.PatchID)
	if err != nil {
		s.mutationFallback(w, r)
		return
	}
	renderPage(s.log, w, r, http.StatusOK, ui.MatchDetailPage(m, p.Version, lineups))
}

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
