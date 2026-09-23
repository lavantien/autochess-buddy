// Package analytics holds the read-side view contracts. The engine lives in the duckdb
// subpackage; stage A fixes only the row shapes and the service interface both D and E
// compile against.
package analytics

import (
	"context"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// Filter scopes a view. Zero values mean all.
type Filter struct {
	PatchID int64
	Source  string
}

type HeroRow struct {
	Hero                       domain.Hero
	Picks, Top4, LineupsInView int
	AvgPlace                   float64
	Finishes                   [8]int // placement 1..8 histogram, index 0 unused
	// Rates, floor and vs-field are assembled in stage D on top of the counts.
	PickRate, Top4Rate, Floor, VsField float64
}

type SynergyRow struct {
	Kind      string // "race" or "class"
	ID        int64
	Name      string
	TierCount int
	Lineups   int
	Lift      float64
	Finishes  [8]int
}

type ItemRow struct {
	Item          domain.Item
	SlotsWith     int
	LineupsInView int
	Lift          float64
	Finishes      [8]int
}

type RelicRow struct {
	Relic         domain.Relic
	Lineups       int
	LineupsInView int
	Lift          float64
	Finishes      [8]int
}

type PlaceRow struct {
	Placement   int
	N           int
	AvgNetworth float64
}

// Service is the analytics port the handlers consume (readme:243); the duckdb engine
// implements it.
type Service interface {
	HeroPerformance(ctx context.Context, f Filter) ([]HeroRow, error)
	SynergyPerformance(ctx context.Context, f Filter) ([]SynergyRow, error)
	ItemPerformance(ctx context.Context, f Filter) ([]ItemRow, error)
	RelicPerformance(ctx context.Context, f Filter) ([]RelicRow, error)
	NetworthByPlacement(ctx context.Context, f Filter) ([]PlaceRow, error)
	LineupsInView(ctx context.Context, f Filter) (int, error)
}
