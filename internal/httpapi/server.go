package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/analytics"
	"github.com/lavantien/autochess-buddy/internal/service"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// Server carries every dependency the handlers need. Dashboard partials land in a
// later task; until then those routes render the spec empty state.
type Server struct {
	log   *slog.Logger
	st    *sqlite.Store
	entry service.EntryService
	dash  analytics.Service
}

func New(log *slog.Logger, st *sqlite.Store, entry service.EntryService, dash analytics.Service) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{log: log, st: st, entry: entry, dash: dash}
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.Static()))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	mux.HandleFunc("GET /dashboard", s.dashboardHome)
	mux.HandleFunc("GET /dashboard/heroes", s.dashView("heroes"))
	mux.HandleFunc("GET /dashboard/synergies", s.dashView("synergies"))
	mux.HandleFunc("GET /dashboard/items", s.dashView("items"))
	mux.HandleFunc("GET /dashboard/relics", s.dashView("relics"))

	mux.HandleFunc("GET /heroes", s.heroIndex)
	mux.HandleFunc("POST /heroes", s.heroCreate)
	mux.HandleFunc("GET /heroes/{id}", s.heroEdit)
	mux.HandleFunc("POST /heroes/{id}", s.heroUpdate)
	mux.HandleFunc("DELETE /heroes/{id}", s.heroDelete)

	// Races and classes share the synergies tab and its ladder editor.
	toSynergies := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/races", http.StatusSeeOther)
	}
	mux.HandleFunc("GET /races", s.synergyIndex)
	mux.HandleFunc("POST /races", func(w http.ResponseWriter, r *http.Request) { s.synergyCreate(w, r, "races") })
	mux.HandleFunc("GET /races/{id}", toSynergies)
	mux.HandleFunc("POST /races/{id}", func(w http.ResponseWriter, r *http.Request) { s.synergyUpdate(w, r, "races") })
	mux.HandleFunc("DELETE /races/{id}", func(w http.ResponseWriter, r *http.Request) { s.codexDelete(w, r, "races", pathID(r, "id"), nil) })
	mux.HandleFunc("GET /classes", s.synergyIndex)
	mux.HandleFunc("POST /classes", func(w http.ResponseWriter, r *http.Request) { s.synergyCreate(w, r, "classes") })
	mux.HandleFunc("GET /classes/{id}", toSynergies)
	mux.HandleFunc("POST /classes/{id}", func(w http.ResponseWriter, r *http.Request) { s.synergyUpdate(w, r, "classes") })
	mux.HandleFunc("DELETE /classes/{id}", func(w http.ResponseWriter, r *http.Request) { s.codexDelete(w, r, "classes", pathID(r, "id"), nil) })

	mux.HandleFunc("GET /items", s.itemIndex)
	mux.HandleFunc("POST /items", s.itemCreate)
	mux.HandleFunc("GET /items/{id}", s.itemEdit)
	mux.HandleFunc("POST /items/{id}", s.itemUpdate)
	mux.HandleFunc("DELETE /items/{id}", s.itemDelete)

	mux.HandleFunc("GET /relics", s.relicIndex)
	mux.HandleFunc("POST /relics", s.relicCreate)
	mux.HandleFunc("GET /relics/{id}", s.relicEdit)
	mux.HandleFunc("POST /relics/{id}", s.relicUpdate)
	mux.HandleFunc("DELETE /relics/{id}", s.relicDelete)

	mux.HandleFunc("GET /patches", s.patchIndex)
	mux.HandleFunc("POST /patches", s.patchCreate)
	mux.HandleFunc("GET /patches/{id}", s.patchEdit)
	mux.HandleFunc("POST /patches/{id}", s.patchUpdate)
	mux.HandleFunc("DELETE /patches/{id}", s.patchDelete)

	mux.HandleFunc("GET /pros", s.proIndex)
	mux.HandleFunc("POST /pros", s.proCreate)
	mux.HandleFunc("GET /pros/{id}", s.proEdit)
	mux.HandleFunc("POST /pros/{id}", s.proUpdate)
	mux.HandleFunc("DELETE /pros/{id}", s.proDelete)

	mux.HandleFunc("GET /matches", s.matchList)
	mux.HandleFunc("GET /matches/new", s.newMatchForm)
	mux.HandleFunc("POST /matches/new", s.createMatch)
	mux.HandleFunc("GET /matches/{id}", s.matchDetail)
	mux.HandleFunc("GET /matches/{id}/edit", s.editorPage)
	mux.HandleFunc("POST /matches/{id}/lineups", s.addLineup)
	mux.HandleFunc("POST /matches/{id}/finalize", s.finalize)

	mux.HandleFunc("POST /lineups/{id}/slots", s.addSlot)
	mux.HandleFunc("POST /slots/{id}/items", s.attachItem)
	mux.HandleFunc("POST /lineups/{id}/relics", s.addRelic)
	mux.HandleFunc("POST /lineups/{id}", s.updateLineup)
	mux.HandleFunc("POST /slots/{id}", s.saveStars)
	mux.HandleFunc("DELETE /slots/{id}", s.deleteSlot)
	mux.HandleFunc("DELETE /lineups/{id}", s.deleteLineup)
	mux.HandleFunc("DELETE /slots/{id}/items/{itemId}", s.removeItem)
	mux.HandleFunc("DELETE /lineups/{id}/relics/{relicId}", s.removeRelic)

	return logRequests(log, OriginGuard(mux))
}

func stubPage(log *slog.Logger, section, title, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPage(log, w, r, http.StatusOK, ui.BaseLayout(section, title, ui.EmptyState(message, nil)))
	}
}

// mutationFallback answers requests whose match context cannot be loaded: non-hx
// gets sent back to the matches list, hx gets a bare 422 with no fragments. The
// cause is logged so a failure never looks like quiet success.
func (s *Server) mutationFallback(w http.ResponseWriter, r *http.Request, cause error) {
	if cause != nil {
		s.log.Warn("mutation fallback", "method", r.Method, "path", r.URL.Path, "err", cause)
	}
	if isHX(r) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	http.Redirect(w, r, "/matches", http.StatusSeeOther)
}

// loadList fetches an option list for a rerender; a load failure logs and
// yields nil so the page still renders its primary answer.
func loadList[T any](log *slog.Logger, ctx context.Context, what string, load func(context.Context) ([]T, error)) []T {
	rows, err := load(ctx)
	if err != nil {
		log.Warn("load "+what, "err", err)
	}
	return rows
}
