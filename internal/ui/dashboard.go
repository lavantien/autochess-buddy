package ui

import (
	"strconv"
	"time"

	"github.com/lavantien/autochess-buddy/internal/domain"
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

// patchKnown reports whether the dashboard filter's version names a real patch
// row; an unknown version renders a disabled option instead of "all".
func patchKnown(patches []domain.Patch, version string) bool {
	for _, p := range patches {
		if p.Version == version {
			return true
		}
	}
	return false
}
