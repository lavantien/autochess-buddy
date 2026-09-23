package ui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

//go:embed static
var staticFS embed.FS

func Static() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("ui: " + err.Error())
	}
	return http.FileServerFS(sub)
}

func placeClass(placement int) string {
	switch {
	case placement == 1:
		return "place-1"
	case placement <= 4:
		return "place-2"
	default:
		return "place-5"
	}
}

func costClass(cost int) string {
	return "c" + strconv.Itoa(cost)
}

func spreadLabel(shares [8]int) string {
	parts := make([]string, len(shares))
	for i, s := range shares {
		parts[i] = strconv.Itoa(s)
	}
	return "finishes by placement: " + strings.Join(parts, ", ")
}

func spreadWidth(shares [8]int, i int) string {
	total := 0
	for _, s := range shares {
		total += s
	}
	if total == 0 {
		return "0%"
	}
	pct := float64(shares[i]) / float64(total) * 100
	return strconv.FormatFloat(pct, 'f', 2, 64) + "%"
}

func spreadTier(place int) string {
	switch {
	case place == 1:
		return "spread-tier-1"
	case place <= 4:
		return "spread-tier-2"
	default:
		return "spread-tier-3"
	}
}

var codexSingular = map[string]string{
	"heroes":  "hero",
	"races":   "race",
	"classes": "class",
	"items":   "item",
	"relics":  "relic",
	"patches": "patch",
	"pros":    "pro",
}

func CodexEmpty(entity string) string {
	return fmt.Sprintf("no %s yet. add the first %s so lineups can reference it.", entity, codexSingular[entity])
}

type tab struct {
	Label  string
	Href   string
	Active bool
}

func codexTabs(active string) []tab {
	tabs := []tab{
		{Label: "heroes", Href: "/heroes"},
		{Label: "synergies", Href: "/races"},
		{Label: "items", Href: "/items"},
		{Label: "relics", Href: "/relics"},
		{Label: "patches", Href: "/patches"},
		{Label: "pros", Href: "/pros"},
	}
	for i := range tabs {
		if tabs[i].Href == "/"+active || (active == "classes" && tabs[i].Label == "synergies") {
			tabs[i].Active = true
		}
	}
	return tabs
}

var legendEntries = [][2]string{
	{"pick rate", "share of lineups in view that played the hero"},
	{"top 4", "of the lineups playing this hero, the share finishing 1st through 4th"},
	{"floor", "the cautious reading of top 4, a wilson 95 percent lower bound, with the lineups on hand the true rate could be as low as this"},
	{"avg place", "mean finishing placement of the lineups playing the hero, lower is better"},
	{"vs field", "difference from the average placement of all lineups in the same filter, negative is better"},
	{"synergy lift", "for a race or class at tier count k, average placement of lineups with at least k units minus the field average, negative is better"},
	{"item lift", "average placement of slots holding the item minus the average placement of the same hero in slots without it, negative is better"},
	{"relic lift", "same shape over the lineups holding the relic"},
}
