package ui

import (
	"strconv"
	"time"
)

// pct renders a share as a percent with one decimal.
func pct(v float64) string {
	return strconv.FormatFloat(v*100, 'f', 1, 64) + "%"
}

// num renders a plain metric with one decimal.
func num(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// signed renders a delta with an explicit plus for positives.
func signed(v float64) string {
	if v > 0 {
		return "+" + strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// playedText renders a unix timestamp as the list page's local stamp.
func playedText(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format("2006-01-02 15:04")
}

// dashViews is the tab strip over the analytics partials.
var dashViews = []string{"heroes", "synergies", "items", "relics"}
