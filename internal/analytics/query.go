package analytics

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed queries/*.sql
var queryFS embed.FS

// querySQL returns the embedded catalogue query by stem. The embed glob makes
// a missing file a compile error, so the read cannot fail at runtime.
func querySQL(name string) string {
	q, err := queryFS.ReadFile("queries/" + name + ".sql")
	if err != nil {
		panic("catalogue query " + name + ": " + err.Error())
	}
	return string(q)
}

// catalogueQuery renders an embedded catalogue query for a filter. Each SQL
// text carries exactly ONE filters token at its guard predicate — the
// count-1 replace below depends on that invariant. Bound parameters are not
// used on purpose: this duckdb prebuilt linked against the current Windows
// toolchain crashes the process on any bind, named or positional, while
// parameterless statements run clean, so the filters inline as literals.
func catalogueQuery(name string, f Filter) string {
	return strings.Replace(querySQL(name), "{{filters}}", filterSQL(f), 1)
}

// filterSQL emits the guard clauses narrowing the view to finalized matches
// and the optional patch and source. Zero filter values emit nothing.
func filterSQL(f Filter) string {
	clauses := ""
	if f.PatchID != 0 {
		clauses += fmt.Sprintf(" AND m.patch_id = %d", f.PatchID)
	}
	if f.Source != "" {
		clauses += " AND m.source = " + quoteLiteral(f.Source)
	}
	return clauses
}

// quoteLiteral renders a SQL string literal with embedded single quotes
// doubled.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Raw rows mirror the catalogue SQL columns one to one. The finish columns
// scan straight into the [8]int arrays; service.go turns these into the view
// types from views.go.

type heroRaw struct {
	ID            int64
	Name          string
	Cost          int
	Ability       string
	Notes         string
	Picks         int
	Top4          int
	AvgPlace      float64
	F             [8]int
	LineupsInView int
	FieldAvg      float64
}

type synergyRaw struct {
	Kind      string
	ID        int64
	Name      string
	TierCount int
	Lineups   int
	Lift      float64
	F         [8]int
}

type itemRaw struct {
	ID            int64
	Name          string
	Tier          int
	Effect        string
	SlotsWith     int
	LineupsInView int
	Lift          float64
	F             [8]int
}

type relicRaw struct {
	ID            int64
	Name          string
	Effect        string
	Lineups       int
	LineupsInView int
	Lift          float64
	F             [8]int
}

type placeRaw struct {
	Placement   int
	N           int
	AvgNetworth float64
}

type fieldRaw struct {
	AvgPlace float64
	N        int
}

// gather runs one catalogue query inside a single batch and scans every row
// in column order. An empty result returns no rows and no error.
func gather[T any](ctx context.Context, e *engine, name string, f Filter, scan func(*sql.Rows, *T) error) ([]T, error) {
	var out []T
	err := e.batch(ctx, catalogueQuery(name, f), nil, func(r *sql.Rows) error {
		for r.Next() {
			var v T
			if err := scan(r, &v); err != nil {
				return err
			}
			out = append(out, v)
		}
		return r.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func scanHero(r *sql.Rows, h *heroRaw) error {
	return r.Scan(&h.ID, &h.Name, &h.Cost, &h.Ability, &h.Notes,
		&h.Picks, &h.Top4, &h.AvgPlace,
		&h.F[0], &h.F[1], &h.F[2], &h.F[3],
		&h.F[4], &h.F[5], &h.F[6], &h.F[7],
		&h.LineupsInView, &h.FieldAvg)
}

func scanSynergy(r *sql.Rows, s *synergyRaw) error {
	return r.Scan(&s.Kind, &s.ID, &s.Name, &s.TierCount, &s.Lineups, &s.Lift,
		&s.F[0], &s.F[1], &s.F[2], &s.F[3],
		&s.F[4], &s.F[5], &s.F[6], &s.F[7])
}

func scanItem(r *sql.Rows, it *itemRaw) error {
	return r.Scan(&it.ID, &it.Name, &it.Tier, &it.Effect,
		&it.SlotsWith, &it.LineupsInView, &it.Lift,
		&it.F[0], &it.F[1], &it.F[2], &it.F[3],
		&it.F[4], &it.F[5], &it.F[6], &it.F[7])
}

func scanRelic(r *sql.Rows, rl *relicRaw) error {
	return r.Scan(&rl.ID, &rl.Name, &rl.Effect,
		&rl.Lineups, &rl.LineupsInView, &rl.Lift,
		&rl.F[0], &rl.F[1], &rl.F[2], &rl.F[3],
		&rl.F[4], &rl.F[5], &rl.F[6], &rl.F[7])
}

func scanPlace(r *sql.Rows, p *placeRaw) error {
	return r.Scan(&p.Placement, &p.N, &p.AvgNetworth)
}

func (e *engine) queryHeroes(ctx context.Context, f Filter) ([]heroRaw, error) {
	return gather(ctx, e, "heroes", f, scanHero)
}

func (e *engine) querySynergies(ctx context.Context, f Filter) ([]synergyRaw, error) {
	return gather(ctx, e, "synergies", f, scanSynergy)
}

func (e *engine) queryItems(ctx context.Context, f Filter) ([]itemRaw, error) {
	return gather(ctx, e, "items", f, scanItem)
}

func (e *engine) queryRelics(ctx context.Context, f Filter) ([]relicRaw, error) {
	return gather(ctx, e, "relics", f, scanRelic)
}

func (e *engine) queryNetworth(ctx context.Context, f Filter) ([]placeRaw, error) {
	return gather(ctx, e, "networth", f, scanPlace)
}

// queryField and countView read the single aggregate row their queries always
// produce.
func (e *engine) queryField(ctx context.Context, f Filter) (fieldRaw, error) {
	var out fieldRaw
	err := e.batch(ctx, catalogueQuery("field", f), nil, func(r *sql.Rows) error {
		if !r.Next() {
			return r.Err()
		}
		return r.Scan(&out.AvgPlace, &out.N)
	})
	return out, err
}

func (e *engine) countView(ctx context.Context, f Filter) (int, error) {
	var n int
	err := e.batch(ctx, catalogueQuery("count", f), nil, func(r *sql.Rows) error {
		if !r.Next() {
			return r.Err()
		}
		return r.Scan(&n)
	})
	return n, err
}
