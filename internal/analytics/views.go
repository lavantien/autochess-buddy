// Package analytics holds the read-side service contract. The engine lives in
// this package too; the row and filter types it produces live in domain so
// every layer above can render against them (readme:100 layering).
package analytics

import (
	"context"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// Service is the analytics port the handlers consume (readme:243); the duckdb
// engine implements it.
type Service interface {
	HeroPerformance(ctx context.Context, f domain.Filter) ([]domain.HeroRow, error)
	SynergyPerformance(ctx context.Context, f domain.Filter) ([]domain.SynergyRow, error)
	ItemPerformance(ctx context.Context, f domain.Filter) ([]domain.ItemRow, error)
	RelicPerformance(ctx context.Context, f domain.Filter) ([]domain.RelicRow, error)
	NetworthByPlacement(ctx context.Context, f domain.Filter) ([]domain.PlaceRow, error)
	LineupsInView(ctx context.Context, f domain.Filter) (int, error)
}
