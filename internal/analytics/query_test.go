package analytics

import (
	"context"
	"math"
	"testing"
)

// Golden numbers derive from the fixture structure, never from engine output:
// the pro lineups give every hero 8 picks covering placements 1..8 once, the me
// lineup 33 (match 5, patch 2) adds one pick at placement 1, and draft lineups
// 34..36 are never in a view because their match has finalized_at = 0.

// TestQuoteLiteral_NeutralizesQuoteAttack pins the only hand-written link in
// the literal-composition chain: a hostile Source must arrive as one inert
// string literal, never as SQL.
func TestQuoteLiteral_NeutralizesQuoteAttack(t *testing.T) {
	_, e := openAnalytics(t)
	rows, err := e.HeroPerformance(context.Background(), Filter{Source: "pro' OR '1'='1"})
	if err != nil {
		t.Fatalf("hostile source must not error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("hostile source must match nothing, got %d rows", len(rows))
	}
}

func TestHeroQuery_FilterScoping(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		f     Filter
		picks int
	}{
		{"default view keeps pro plus me", Filter{}, 9},
		{"pro only", Filter{Source: "pro"}, 8},
		{"patch 1", Filter{PatchID: 1}, 4},
		{"patch 2 keeps the me match", Filter{PatchID: 2}, 5},
		{"me only", Filter{Source: "me"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := e.queryHeroes(ctx, tc.f)
			if err != nil {
				t.Fatalf("queryHeroes: %v", err)
			}
			var h *heroRaw
			for i := range rows {
				if rows[i].ID == 1 {
					h = &rows[i]
				}
			}
			if h == nil {
				t.Fatalf("hero 1 missing from %d rows", len(rows))
			}
			if h.Picks != tc.picks {
				t.Errorf("hero 1 picks = %d, want %d", h.Picks, tc.picks)
			}
		})
	}
	t.Run("unknown source matches nothing", func(t *testing.T) {
		rows, err := e.queryHeroes(ctx, Filter{Source: "bogus"})
		if err != nil {
			t.Fatalf("queryHeroes: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("got %d rows, want 0", len(rows))
		}
	})
	t.Run("pro view scoping numbers", func(t *testing.T) {
		rows, err := e.queryHeroes(ctx, Filter{Source: "pro"})
		if err != nil {
			t.Fatalf("queryHeroes: %v", err)
		}
		if len(rows) != 12 {
			t.Fatalf("got %d heroes, want 12", len(rows))
		}
		h := rows[0]
		if h.ID != 1 {
			t.Fatalf("first row is hero %d, want 1 (ORDER BY id)", h.ID)
		}
		if h.Top4 != 4 {
			t.Errorf("hero 1 top4 = %d, want 4", h.Top4)
		}
		if h.AvgPlace != 4.5 {
			t.Errorf("hero 1 avg place = %v, want 4.5", h.AvgPlace)
		}
		if h.LineupsInView != 32 {
			t.Errorf("lineups in view = %d, want 32", h.LineupsInView)
		}
		if h.F != [8]int{1, 1, 1, 1, 1, 1, 1, 1} {
			t.Errorf("finishes = %v, want one per placement", h.F)
		}
	})
}

func TestSynergyQuery_TiersAndSlices(t *testing.T) {
	_, e := openAnalytics(t)
	rows, err := e.querySynergies(context.Background(), Filter{})
	if err != nil {
		t.Fatalf("querySynergies: %v", err)
	}
	// 3 races + 3 classes, each with tier ladders at counts 2, 4 and 6.
	if len(rows) != 18 {
		t.Fatalf("got %d rows, want 18", len(rows))
	}
	fieldAvg := 145.0 / 33.0
	cases := []struct {
		kind string
		id   int64
		k    int
		n    int
		lift float64
	}{
		{"race", 1, 2, 8, 4.5 - fieldAvg},
		{"race", 1, 4, 0, 0},
		{"race", 3, 2, 8, 4.5 - fieldAvg},
		{"class", 1, 2, 8, 4.5 - fieldAvg},
		{"class", 2, 2, 0, 0},
		{"class", 3, 2, 1, 1.0 - fieldAvg},
		{"class", 3, 4, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			var row *synergyRaw
			for i := range rows {
				r := rows[i]
				if r.Kind == tc.kind && r.ID == tc.id && r.TierCount == tc.k {
					row = &rows[i]
				}
			}
			if row == nil {
				t.Fatalf("%s %d at count %d missing", tc.kind, tc.id, tc.k)
			}
			if row.Lineups != tc.n {
				t.Errorf("lineups = %d, want %d", row.Lineups, tc.n)
			}
			if math.Abs(row.Lift-tc.lift) > 1e-9 {
				t.Errorf("lift = %v, want %v", row.Lift, tc.lift)
			}
		})
	}
	druid := synergyRaw{}
	for i := range rows {
		if rows[i].Kind == "class" && rows[i].ID == 3 && rows[i].TierCount == 2 {
			druid = rows[i]
		}
	}
	if druid.F != [8]int{1, 0, 0, 0, 0, 0, 0, 0} {
		t.Errorf("druid finishes = %v, want {1:1}", druid.F)
	}
}

func TestItemQuery_Lift(t *testing.T) {
	_, e := openAnalytics(t)
	rows, err := e.queryItems(context.Background(), Filter{Source: "pro"})
	if err != nil {
		t.Fatalf("queryItems: %v", err)
	}
	if len(rows) != 8 {
		t.Fatalf("got %d items, want 8", len(rows))
	}
	// Each hero's 8 pro slots carry items 1..8 exactly once, so every item sits
	// on 12 slots and the same-hero baseline has 84 slots.
	lifts := []float64{-4.0 / 3.0, -20.0 / 21.0, -4.0 / 7.0, -4.0 / 21.0, 4.0 / 21.0, 4.0 / 7.0, 20.0 / 21.0, 4.0 / 3.0}
	for i, r := range rows {
		if r.ID != int64(i+1) {
			t.Fatalf("row %d is item %d, want %d (ORDER BY id)", i, r.ID, i+1)
		}
		if r.SlotsWith != 12 {
			t.Errorf("item %d slots with = %d, want 12", r.ID, r.SlotsWith)
		}
		if r.LineupsInView != 32 {
			t.Errorf("item %d lineups in view = %d, want 32", r.ID, r.LineupsInView)
		}
		if math.Abs(r.Lift-lifts[i]) > 1e-9 {
			t.Errorf("item %d lift = %v, want %v", r.ID, r.Lift, lifts[i])
		}
	}
}

func TestRelicQuery_Lift(t *testing.T) {
	_, e := openAnalytics(t)
	rows, err := e.queryRelics(context.Background(), Filter{Source: "pro"})
	if err != nil {
		t.Fatalf("queryRelics: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d relics, want 4", len(rows))
	}
	// Relic r sits on the 8 lineups with (id-1) mod 4 + 1 = r; their placements
	// average r+2, the other 24 average (16-r)/3.
	lifts := []float64{-2, -2.0 / 3.0, 2.0 / 3.0, 2}
	for i, r := range rows {
		if r.ID != int64(i+1) {
			t.Fatalf("row %d is relic %d, want %d (ORDER BY id)", i, r.ID, i+1)
		}
		if r.Lineups != 8 {
			t.Errorf("relic %d lineups = %d, want 8", r.ID, r.Lineups)
		}
		if r.LineupsInView != 32 {
			t.Errorf("relic %d lineups in view = %d, want 32", r.ID, r.LineupsInView)
		}
		if math.Abs(r.Lift-lifts[i]) > 1e-9 {
			t.Errorf("relic %d lift = %v, want %v", r.ID, r.Lift, lifts[i])
		}
	}
}

func TestNetworthQuery_Curve(t *testing.T) {
	_, e := openAnalytics(t)
	rows, err := e.queryNetworth(context.Background(), Filter{Source: "pro"})
	if err != nil {
		t.Fatalf("queryNetworth: %v", err)
	}
	if len(rows) != 8 {
		t.Fatalf("got %d rows, want 8", len(rows))
	}
	for p, r := range rows {
		placement, n := p+1, 4
		want := float64(1000 + (8-placement)*50)
		if r.Placement != placement || r.N != n {
			t.Fatalf("row %d is placement %d with n=%d, want placement %d n=%d", p, r.Placement, r.N, placement, n)
		}
		if r.AvgNetworth != want {
			t.Errorf("placement %d avg networth = %v, want %v", placement, r.AvgNetworth, want)
		}
	}
}

func TestViewCountAndField(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()
	cases := []struct {
		name string
		f    Filter
		n    int
		avg  float64
	}{
		{"default view", Filter{}, 33, 145.0 / 33.0},
		{"pro only", Filter{Source: "pro"}, 32, 4.5},
		{"me only", Filter{Source: "me"}, 1, 1},
		{"patch 1", Filter{PatchID: 1}, 16, 4.5},
		{"patch 2", Filter{PatchID: 2}, 17, (16*4.5 + 1) / 17},
		{"patch 2 pro", Filter{PatchID: 2, Source: "pro"}, 16, 4.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := e.countView(ctx, tc.f)
			if err != nil {
				t.Fatalf("countView: %v", err)
			}
			if n != tc.n {
				t.Errorf("countView = %d, want %d", n, tc.n)
			}
			f, err := e.queryField(ctx, tc.f)
			if err != nil {
				t.Fatalf("queryField: %v", err)
			}
			if f.N != tc.n {
				t.Errorf("field n = %d, want %d", f.N, tc.n)
			}
			if math.Abs(f.AvgPlace-tc.avg) > 1e-9 {
				t.Errorf("field avg = %v, want %v", f.AvgPlace, tc.avg)
			}
		})
	}
	t.Run("empty view", func(t *testing.T) {
		f, err := e.queryField(ctx, Filter{PatchID: 999})
		if err != nil {
			t.Fatalf("queryField: %v", err)
		}
		if f.N != 0 || f.AvgPlace != 0 {
			t.Errorf("field = %+v, want n=0 avg=0", f)
		}
	})
}
