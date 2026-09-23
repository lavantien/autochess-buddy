package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

func isHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

func renderPage(log *slog.Logger, w http.ResponseWriter, r *http.Request, status int, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := comp.Render(r.Context(), w); err != nil {
		log.Error("render page", "err", err)
	}
}

func renderOOB(log *slog.Logger, w http.ResponseWriter, r *http.Request, status int, frags ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	for _, f := range frags {
		if err := f.Render(r.Context(), w); err != nil {
			log.Error("render out-of-band fragments", "err", err)
			return
		}
	}
}
