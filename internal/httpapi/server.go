package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/lavantien/autochess-buddy/internal/ui"
)

func New(log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.Static()))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	dashEmpty := "no finalized matches for this filter yet. finalize a few matches first."
	mux.HandleFunc("GET /dashboard", stubPage(log, "dashboard", "dashboard", dashEmpty))
	mux.HandleFunc("GET /dashboard/heroes", stubPage(log, "dashboard", "hero performance", dashEmpty))
	mux.HandleFunc("GET /dashboard/synergies", stubPage(log, "dashboard", "synergy lift", dashEmpty))
	mux.HandleFunc("GET /dashboard/items", stubPage(log, "dashboard", "item lift", dashEmpty))
	mux.HandleFunc("GET /dashboard/relics", stubPage(log, "dashboard", "relic lift", dashEmpty))

	for _, entity := range []string{"heroes", "races", "classes", "items", "relics", "patches", "pros"} {
		mux.HandleFunc("GET /"+entity, stubPage(log, "codex", entity, ui.CodexEmpty(entity)))
		mux.HandleFunc("POST /"+entity, mutStub("/"+entity))
		mux.HandleFunc("GET /"+entity+"/{id}", stubPage(log, "codex", entity, ui.CodexEmpty(entity)))
		mux.HandleFunc("POST /"+entity+"/{id}", mutStub("/"+entity))
		mux.HandleFunc("DELETE /"+entity+"/{id}", mutStub("/"+entity))
	}

	matchesEmpty := "no matches yet. start one from the game you just finished."
	mux.HandleFunc("GET /matches", stubPage(log, "matches", "matches", matchesEmpty))
	mux.HandleFunc("GET /matches/new", stubPage(log, "matches", "new match", matchesEmpty))
	mux.HandleFunc("POST /matches/new", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/matches", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /matches/{id}", stubPage(log, "matches", "match", matchesEmpty))
	mux.HandleFunc("GET /matches/{id}/edit", stubPage(log, "matches", "edit match", matchesEmpty))
	mux.HandleFunc("POST /matches/{id}/lineups", mutStub("/matches"))
	mux.HandleFunc("POST /matches/{id}/finalize", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/matches/"+r.PathValue("id"), http.StatusSeeOther)
	})

	toMatches := mutStub("/matches")
	mux.HandleFunc("POST /lineups/{id}/slots", toMatches)
	mux.HandleFunc("POST /slots/{id}/items", toMatches)
	mux.HandleFunc("POST /lineups/{id}/relics", toMatches)
	mux.HandleFunc("POST /lineups/{id}", toMatches)
	mux.HandleFunc("POST /slots/{id}", toMatches)
	mux.HandleFunc("DELETE /slots/{id}", toMatches)
	mux.HandleFunc("DELETE /lineups/{id}", toMatches)
	mux.HandleFunc("DELETE /slots/{id}/items/{itemId}", toMatches)
	mux.HandleFunc("DELETE /lineups/{id}/relics/{relicId}", toMatches)

	return logRequests(log, OriginGuard(mux))
}

func stubPage(log *slog.Logger, section, title, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPage(log, w, r, http.StatusOK, ui.BaseLayout(section, title, ui.EmptyState(message, nil)))
	}
}

func mutStub(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isHX(r) {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}
