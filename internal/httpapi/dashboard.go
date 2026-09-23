package httpapi

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/lavantien/autochess-buddy/internal/analytics"
	"github.com/lavantien/autochess-buddy/internal/ui"
)

// dashboardHome answers GET /dashboard with the hero view.
func (s *Server) dashboardHome(w http.ResponseWriter, r *http.Request) {
	s.dashView("heroes").ServeHTTP(w, r)
}

// dashView answers one dashboard request: the full page for browsers, the bare
// panel for hx-get swaps.
func (s *Server) dashView(view string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, patchVersion := s.dashFilter(r)
		panel, n, ok := s.dashPanel(w, r, view, f)
		if !ok {
			return
		}
		patches, _ := s.st.ListPatches(r.Context())
		if isHX(r) {
			renderOOB(s.log, w, r, http.StatusOK, ui.DashPanel(view, panel))
			return
		}
		renderPage(s.log, w, r, http.StatusOK, ui.DashboardPage(view, patches, patchVersion, f, ui.DashPanel(view, panel), n))
	}
}

// dashFilter resolves the query params into the analytics filter.
func (s *Server) dashFilter(r *http.Request) (analytics.Filter, string) {
	f := analytics.Filter{Source: r.URL.Query().Get("source")}
	version := r.URL.Query().Get("patch")
	if version != "" {
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
	return f, version
}

// dashPanel runs the view's query and renders its table; n is the lineups-in-view
// count for the filter row.
func (s *Server) dashPanel(w http.ResponseWriter, r *http.Request, view string, f analytics.Filter) (templ.Component, int, bool) {
	ctx := r.Context()
	n := 0
	if count, err := s.dash.LineupsInView(ctx, f); err == nil {
		n = count
	}
	var panel templ.Component
	switch view {
	case "heroes":
		rows, err := s.dash.HeroPerformance(ctx, f)
		if err != nil {
			stubPage(s.log, "dashboard", "dashboard", "no finalized matches for this filter yet. finalize a few matches first.").ServeHTTP(w, r)
			return nil, 0, false
		}
		panel = ui.HeroTable(rows)
	case "synergies":
		rows, err := s.dash.SynergyPerformance(ctx, f)
		if err != nil {
			stubPage(s.log, "dashboard", "dashboard", "no finalized matches for this filter yet. finalize a few matches first.").ServeHTTP(w, r)
			return nil, 0, false
		}
		panel = ui.SynergyTable(rows)
	case "items":
		rows, err := s.dash.ItemPerformance(ctx, f)
		if err != nil {
			stubPage(s.log, "dashboard", "dashboard", "no finalized matches for this filter yet. finalize a few matches first.").ServeHTTP(w, r)
			return nil, 0, false
		}
		panel = ui.ItemTable(rows)
	case "relics":
		rows, err := s.dash.RelicPerformance(ctx, f)
		if err != nil {
			stubPage(s.log, "dashboard", "dashboard", "no finalized matches for this filter yet. finalize a few matches first.").ServeHTTP(w, r)
			return nil, 0, false
		}
		panel = ui.RelicTable(rows)
	}
	return panel, n, true
}
