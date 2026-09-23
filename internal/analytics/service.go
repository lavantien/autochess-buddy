package analytics

import (
	"context"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// rate is k/n, 0 for an empty denominator, so a hero with no picks in view
// still renders instead of dividing by zero.
func rate(k, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(k) / float64(n)
}

// HeroPerformance assembles hero rows: pick and top4 rates over the view,
// the Wilson lower bound as ranking floor, and placement delta against the
// field average. Heroes with no pick in view do not appear.
func (e *engine) HeroPerformance(ctx context.Context, f Filter) ([]HeroRow, error) {
	raws, err := e.queryHeroes(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]HeroRow, len(raws))
	for i, r := range raws {
		out[i] = HeroRow{
			Hero:          domain.Hero{ID: r.ID, Name: r.Name, Cost: r.Cost, Ability: r.Ability, Notes: r.Notes},
			Picks:         r.Picks,
			Top4:          r.Top4,
			LineupsInView: r.LineupsInView,
			AvgPlace:      r.AvgPlace,
			Finishes:      r.F,
			PickRate:      rate(r.Picks, r.LineupsInView),
			Top4Rate:      rate(r.Top4, r.Picks),
			Floor:         domain.WilsonLB(r.Top4, r.Picks),
			VsField:       r.AvgPlace - r.FieldAvg,
		}
	}
	return out, nil
}

// SynergyPerformance maps the tier ladder rows one to one. Lift already
// carries the empty-slice fallback to the field average.
func (e *engine) SynergyPerformance(ctx context.Context, f Filter) ([]SynergyRow, error) {
	raws, err := e.querySynergies(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]SynergyRow, len(raws))
	for i, r := range raws {
		out[i] = SynergyRow{
			Kind:      r.Kind,
			ID:        r.ID,
			Name:      r.Name,
			TierCount: r.TierCount,
			Lineups:   r.Lineups,
			Lift:      r.Lift,
			Finishes:  r.F,
		}
	}
	return out, nil
}

// ItemPerformance maps the item rows one to one; the lift is held-slot
// placement minus same-hero baseline.
func (e *engine) ItemPerformance(ctx context.Context, f Filter) ([]ItemRow, error) {
	raws, err := e.queryItems(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]ItemRow, len(raws))
	for i, r := range raws {
		out[i] = ItemRow{
			Item:          domain.Item{ID: r.ID, Name: r.Name, Tier: r.Tier, Effect: r.Effect},
			SlotsWith:     r.SlotsWith,
			LineupsInView: r.LineupsInView,
			Lift:          r.Lift,
			Finishes:      r.F,
		}
	}
	return out, nil
}

// RelicPerformance maps the relic rows one to one; the lift is holding-lineup
// placement minus the without-half of the view.
func (e *engine) RelicPerformance(ctx context.Context, f Filter) ([]RelicRow, error) {
	raws, err := e.queryRelics(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]RelicRow, len(raws))
	for i, r := range raws {
		out[i] = RelicRow{
			Relic:         domain.Relic{ID: r.ID, Name: r.Name, Effect: r.Effect},
			Lineups:       r.Lineups,
			LineupsInView: r.LineupsInView,
			Lift:          r.Lift,
			Finishes:      r.F,
		}
	}
	return out, nil
}

// NetworthByPlacement maps the per-placement averages one to one.
func (e *engine) NetworthByPlacement(ctx context.Context, f Filter) ([]PlaceRow, error) {
	raws, err := e.queryNetworth(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]PlaceRow, len(raws))
	for i, r := range raws {
		out[i] = PlaceRow(r)
	}
	return out, nil
}

// LineupsInView returns the number of finalized lineups matching the filter.
func (e *engine) LineupsInView(ctx context.Context, f Filter) (int, error) {
	return e.countView(ctx, f)
}
