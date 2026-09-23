package analytics

import (
	"context"
	"math"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"pgregory.net/rapid"
)

// Goldens over the shared seed fixture (internal/seed/fixture.sql). The pro
// view folds matches 1..4 into 32 lineups and every hero lands once on each
// placement 1..8, so the field average is exactly 4.5. The me view is lineup
// 33 alone. Ability, notes and effect carry empty-string schema defaults.

// nearEq compares doubles with a tolerance for the non-dyadic averages
// (37/9, 145/33, 9/33). Dyadic values assert with ==.
func nearEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func checkFinishes(t *testing.T, what string, got, want [8]int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: finishes %v, want %v", what, got, want)
	}
}

func TestHeroPerformance_Golden(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	names := []string{"sky breaker", "grim jaw", "lord of sand", "iron warden",
		"frost weaver", "night fang", "ember cub", "storm herald",
		"veil dancer", "anvil monk", "moss tender", "gale piercer"}
	costs := []int{5, 4, 2, 1, 3, 3, 1, 5, 2, 4, 1, 5}
	once := [8]int{1, 1, 1, 1, 1, 1, 1, 1}

	rows, err := e.HeroPerformance(ctx, domain.Filter{Source: "pro"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("pro: %d hero rows, want 12", len(rows))
	}
	for i, r := range rows {
		if r.Hero.ID != int64(i+1) || r.Hero.Name != names[i] || r.Hero.Cost != costs[i] ||
			r.Hero.Ability != "" || r.Hero.Notes != "" {
			t.Errorf("pro hero %d identity %+v", i+1, r.Hero)
		}
		if r.Picks != 8 || r.Top4 != 4 || r.LineupsInView != 32 ||
			r.AvgPlace != 4.5 || r.PickRate != 0.25 || r.Top4Rate != 0.5 ||
			r.VsField != 0 || r.Floor != domain.WilsonLB(4, 8) {
			t.Errorf("pro hero %d row %+v, want 8/4/32 at 4.5/0.25/0.5/0 with WilsonLB(4,8)", r.Hero.ID, r)
		}
		checkFinishes(t, "pro hero "+names[i], r.Finishes, once)
	}

	rows, err = e.HeroPerformance(ctx, domain.Filter{Source: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("me: %d hero rows, want 3", len(rows))
	}
	for i, r := range rows {
		if r.Hero.ID != int64(4*i+1) || r.Picks != 1 || r.Top4 != 1 ||
			r.LineupsInView != 1 || r.AvgPlace != 1 || r.PickRate != 1 ||
			r.Top4Rate != 1 || r.VsField != 0 || r.Floor != domain.WilsonLB(1, 1) {
			t.Errorf("me hero %d row %+v, want id %d at 1/1/1 all rates 1 with WilsonLB(1,1)", i, r, 4*i+1)
		}
		checkFinishes(t, "me hero", r.Finishes, [8]int{1, 0, 0, 0, 0, 0, 0, 0})
	}

	rows, err = e.HeroPerformance(ctx, domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("default: %d hero rows, want 12", len(rows))
	}
	// The me lineup (33, placement 1) holds heroes 1, 5 and 9, so exactly those
	// three carry 9 picks at 37/9 in the all-sources view; the rest stay pure pro.
	boosted := map[int64]bool{1: true, 5: true, 9: true}
	for _, r := range rows {
		if boosted[r.Hero.ID] {
			if r.Picks != 9 || r.Top4 != 5 || r.LineupsInView != 33 ||
				!nearEq(r.AvgPlace, 37.0/9.0) || !nearEq(r.PickRate, 9.0/33.0) ||
				!nearEq(r.Top4Rate, 5.0/9.0) || !nearEq(r.VsField, 37.0/9.0-145.0/33.0) ||
				r.Floor != domain.WilsonLB(5, 9) {
				t.Errorf("default boosted hero %d row %+v, want 9/5/33 at 37/9, 9/33, 5/9, vs 145/33 with WilsonLB(5,9)", r.Hero.ID, r)
			}
			checkFinishes(t, "default boosted hero", r.Finishes, [8]int{2, 1, 1, 1, 1, 1, 1, 1})
			continue
		}
		if r.Picks != 8 || r.Top4 != 4 || r.LineupsInView != 33 ||
			r.AvgPlace != 4.5 || !nearEq(r.PickRate, 8.0/33.0) || r.Top4Rate != 0.5 ||
			!nearEq(r.VsField, 4.5-145.0/33.0) || r.Floor != domain.WilsonLB(4, 8) {
			t.Errorf("default hero %d row %+v, want 8/4/33 at 4.5, 8/33, 0.5 with WilsonLB(4,8)", r.Hero.ID, r)
		}
		checkFinishes(t, "default hero", r.Finishes, once)
	}
}

func TestSynergyLift_Golden(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	type key struct {
		kind string
		id   int64
		k    int
	}
	rows, err := e.SynergyPerformance(ctx, domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 18 {
		t.Fatalf("default: %d synergy rows, want 18", len(rows))
	}
	if rows[0].Kind != "class" || rows[9].Kind != "race" {
		t.Errorf("default order: rows[0] kind %q rows[9] kind %q, want class then race blocks", rows[0].Kind, rows[9].Kind)
	}
	by := make(map[key]domain.SynergyRow, len(rows))
	for _, r := range rows {
		by[key{r.Kind, r.ID, r.TierCount}] = r
	}
	field := 145.0 / 33.0
	once := [8]int{1, 1, 1, 1, 1, 1, 1, 1}
	heads := []struct {
		kind string
		id   int64
		name string
		n    int
	}{
		{"class", 1, "knight", 8}, {"class", 2, "assassin", 0}, {"class", 3, "druid", 1},
		{"race", 1, "warrior", 8}, {"race", 2, "mage", 8}, {"race", 3, "beast", 8},
	}
	for _, h := range heads {
		for _, k := range []int{2, 4, 6} {
			r, ok := by[key{h.kind, h.id, k}]
			if !ok {
				t.Fatalf("default: missing %s %s tier %d", h.kind, h.name, k)
			}
			if r.Name != h.name {
				t.Errorf("%s tier %d name %q, want %q", h.name, k, r.Name, h.name)
			}
			if k != 2 && r.Lineups != 0 {
				t.Errorf("%s tier %d lineups %d, want 0 (only tier 2 is reachable)", h.name, k, r.Lineups)
			}
			if r.Lineups == 0 {
				if r.Lift != 0 {
					t.Errorf("%s tier %d empty-slice lift %v, want 0", h.name, k, r.Lift)
				}
				continue
			}
			if h.n == 0 {
				t.Errorf("%s tier %d lineups %d, want 0", h.name, k, r.Lineups)
				continue
			}
			want := 4.5 - field
			if h.n == 1 {
				want = 1 - field
			}
			if !nearEq(r.Lift, want) {
				t.Errorf("%s tier %d lift %v, want %v", h.name, k, r.Lift, want)
			}
			if h.n == 1 {
				checkFinishes(t, h.name, r.Finishes, [8]int{1, 0, 0, 0, 0, 0, 0, 0})
			} else {
				checkFinishes(t, h.name, r.Finishes, once)
			}
		}
	}

	rows, err = e.SynergyPerformance(ctx, domain.Filter{Source: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 18 {
		t.Fatalf("me: %d synergy rows, want the full 18 tier ladder", len(rows))
	}
	active := 0
	for _, r := range rows {
		if r.Lift != 0 {
			t.Errorf("me %s %d k%d lift %v, want 0 (me field avg is 1)", r.Kind, r.ID, r.TierCount, r.Lift)
		}
		if r.Lineups > 0 {
			active++
			if r.Kind != "class" || r.ID != 3 || r.TierCount != 2 || r.Lineups != 1 {
				t.Errorf("me active row %+v, want class druid tier 2 with 1 lineup", r)
			}
		}
	}
	if active != 1 {
		t.Errorf("me: %d rows with lineups, want exactly druid tier 2", active)
	}
}

func TestItemLift_Golden(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	rows, err := e.ItemPerformance(ctx, domain.Filter{Source: "pro"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("pro: %d item rows, want 8", len(rows))
	}
	lifts := []float64{-4.0 / 3.0, -20.0 / 21.0, -4.0 / 7.0, -4.0 / 21.0,
		4.0 / 21.0, 4.0 / 7.0, 20.0 / 21.0, 4.0 / 3.0}
	fins := [][8]int{
		{4, 0, 4, 0, 0, 4, 0, 0}, {4, 0, 0, 4, 0, 4, 0, 0},
		{4, 0, 0, 4, 0, 0, 4, 0}, {0, 4, 0, 4, 0, 0, 4, 0},
		{0, 4, 0, 0, 4, 0, 4, 0}, {0, 4, 0, 0, 4, 0, 0, 4},
		{0, 0, 4, 0, 4, 0, 0, 4}, {0, 0, 4, 0, 0, 4, 0, 4}}
	for i, r := range rows {
		if r.Item.ID != int64(i+1) || r.Item.Tier != i/2+1 || r.Item.Effect != "" ||
			r.SlotsWith != 12 || r.LineupsInView != 32 {
			t.Errorf("pro item %d row %+v, want id %d tier %d with 12 slots over 32 lineups", i+1, r.Item, i+1, i/2+1)
		}
		if !nearEq(r.Lift, lifts[i]) {
			t.Errorf("pro item %d lift %v, want %v", r.Item.ID, r.Lift, lifts[i])
		}
		checkFinishes(t, "pro item", r.Finishes, fins[i])
	}

	rows, err = e.ItemPerformance(ctx, domain.Filter{PatchID: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("empty view: %d item rows, want 0", len(rows))
	}
}

func TestRelicLift_Golden(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	rows, err := e.RelicPerformance(ctx, domain.Filter{Source: "pro"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("pro: %d relic rows, want 4", len(rows))
	}
	lifts := []float64{-2, -2.0 / 3.0, 2.0 / 3.0, 2}
	fins := [][8]int{
		{4, 0, 0, 0, 4, 0, 0, 0}, {0, 4, 0, 0, 0, 4, 0, 0},
		{0, 0, 4, 0, 0, 0, 4, 0}, {0, 0, 0, 4, 0, 0, 0, 4}}
	for i, r := range rows {
		if r.Relic.ID != int64(i+1) || r.Relic.Effect != "" || r.Lineups != 8 || r.LineupsInView != 32 {
			t.Errorf("pro relic %d row %+v, want id %d with 8 lineups over 32", i+1, r.Relic, i+1)
		}
		if !nearEq(r.Lift, lifts[i]) {
			t.Errorf("pro relic %d lift %v, want %v", r.Relic.ID, r.Lift, lifts[i])
		}
		checkFinishes(t, "pro relic", r.Finishes, fins[i])
	}

	rows, err = e.RelicPerformance(ctx, domain.Filter{PatchID: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("empty view: %d relic rows, want 0", len(rows))
	}
}

func TestNetworthByPlacement_Golden(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	rows, err := e.NetworthByPlacement(ctx, domain.Filter{Source: "pro"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("pro: %d rows, want 8", len(rows))
	}
	for i, r := range rows {
		if r.Placement != i+1 || r.N != 4 || r.AvgNetworth != float64(1000+(8-i-1)*50) {
			t.Errorf("pro row %d: %+v", i, r)
		}
	}

	rows, err = e.NetworthByPlacement(ctx, domain.Filter{Source: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Placement != 1 || rows[0].N != 1 || rows[0].AvgNetworth != 1200 {
		t.Errorf("me: %+v, want one row placement 1 n 1 avg 1200", rows)
	}

	rows, err = e.NetworthByPlacement(ctx, domain.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("default: %d rows, want 8", len(rows))
	}
	for i, r := range rows {
		n, avg := 4, float64(1000+(8-i-1)*50)
		if i == 0 {
			n, avg = 5, 1320
		}
		if r.Placement != i+1 || r.N != n || r.AvgNetworth != avg {
			t.Errorf("default row %d: %+v, want n %d avg %v", i, r, n, avg)
		}
	}

	rows, err = e.NetworthByPlacement(ctx, domain.Filter{PatchID: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("empty view: %d rows, want 0", len(rows))
	}
}

func TestFieldAvg_ProExactly45(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	for _, c := range []struct {
		f    domain.Filter
		want fieldRaw
	}{
		{domain.Filter{Source: "pro"}, fieldRaw{4.5, 32}},
		{domain.Filter{Source: "me"}, fieldRaw{1, 1}},
		{domain.Filter{PatchID: 999}, fieldRaw{}},
	} {
		f, err := e.queryField(ctx, c.f)
		if err != nil {
			t.Fatal(err)
		}
		if f != c.want {
			t.Errorf("%+v field %+v, want %+v (pro must be exactly 4.5 over 32)", c.f, f, c.want)
		}
	}
}

func TestLineupsInView_Filters(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()

	for _, c := range []struct {
		f    domain.Filter
		want int
	}{
		{domain.Filter{}, 33},
		{domain.Filter{Source: "pro"}, 32},
		{domain.Filter{Source: "me"}, 1},
		{domain.Filter{PatchID: 1}, 16},
		{domain.Filter{PatchID: 2}, 17},
		{domain.Filter{PatchID: 2, Source: "pro"}, 16},
		{domain.Filter{PatchID: 999}, 0},
	} {
		n, err := e.LineupsInView(ctx, c.f)
		if err != nil {
			t.Fatal(err)
		}
		if n != c.want {
			t.Errorf("%+v: %d lineups, want %d", c.f, n, c.want)
		}
	}
}

func TestLift_EmptySliceFallsBackToFieldAvg(t *testing.T) {
	_, e := openAnalytics(t)
	ctx := context.Background()
	rapid.Check(t, func(t *rapid.T) {
		f := domain.Filter{
			PatchID: rapid.SampledFrom([]int64{0, 1, 2, 999}).Draw(t, "patch"),
			Source:  rapid.SampledFrom([]string{"", "pro", "me"}).Draw(t, "source"),
		}
		rows, err := e.SynergyPerformance(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 18 {
			t.Fatalf("%+v: %d rows, want the full 18 tier ladder", f, len(rows))
		}
		field, err := e.queryField(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			sum, weighted := 0, 0
			for p, c := range r.Finishes {
				sum += c
				weighted += (p + 1) * c
			}
			if sum != r.Lineups {
				t.Errorf("%s %d k%d: finishes %v sum %d, lineups %d", r.Kind, r.ID, r.TierCount, r.Finishes, sum, r.Lineups)
			}
			if r.Lineups == 0 {
				if r.Lift != 0 {
					t.Errorf("%s %d k%d: empty-slice lift %v, want exactly 0", r.Kind, r.ID, r.TierCount, r.Lift)
				}
				continue
			}
			want := float64(weighted)/float64(r.Lineups) - field.AvgPlace
			if !nearEq(r.Lift, want) {
				t.Errorf("%s %d k%d: lift %v, want held avg minus field %v", r.Kind, r.ID, r.TierCount, r.Lift, want)
			}
		}
	})
}
