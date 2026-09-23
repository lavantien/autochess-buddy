package sqlite

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/seed"
)

func TestCodexCRUD_RoundTrip(t *testing.T) {
	ctx := context.Background()

	t.Run("race", func(t *testing.T) {
		s := openTemp(t)
		tiers := []domain.Tier{{Count: 1, Effect: "+2 armor"}, {Count: 4, Effect: "+9 armor"}}
		id, err := s.CreateRace(ctx, domain.Race{Name: "elf"}, tiers)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, gotTiers, err := s.GetRace(ctx, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Name != "elf" {
			t.Fatalf("name = %q, want elf", got.Name)
		}
		wantTiers := []domain.Tier{
			{LineageID: id, Count: 1, Effect: "+2 armor"},
			{LineageID: id, Count: 4, Effect: "+9 armor"},
		}
		if !reflect.DeepEqual(gotTiers, wantTiers) {
			t.Fatalf("tiers = %+v, want %+v", gotTiers, wantTiers)
		}
		races, err := s.ListRaces(ctx)
		if err != nil || len(races) != 1 || races[0].ID != id || races[0].Name != "elf" {
			t.Fatalf("list = %+v, err %v", races, err)
		}
		if err := s.UpdateRace(ctx, domain.Race{ID: id, Name: "high elf"}, tiers[:1]); err != nil {
			t.Fatalf("update: %v", err)
		}
		_, gotTiers, err = s.GetRace(ctx, id)
		if err != nil {
			t.Fatalf("get after update: %v", err)
		}
		if len(gotTiers) != 1 || gotTiers[0].Count != 1 || gotTiers[0].Effect != "+2 armor" {
			t.Fatalf("tiers after update = %+v", gotTiers)
		}
		if err := s.DeleteRace(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, _, err := s.GetRace(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("class", func(t *testing.T) {
		s := openTemp(t)
		tiers := []domain.Tier{{Count: 2, Effect: "block one spell"}}
		id, err := s.CreateClass(ctx, domain.Class{Name: "warden"}, tiers)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, gotTiers, err := s.GetClass(ctx, id)
		if err != nil || got.Name != "warden" || len(gotTiers) != 1 || gotTiers[0].Effect != "block one spell" {
			t.Fatalf("get = (%+v, %+v), err %v", got, gotTiers, err)
		}
		classes, err := s.ListClasses(ctx)
		if err != nil || len(classes) != 1 || classes[0].ID != id {
			t.Fatalf("list = %+v, err %v", classes, err)
		}
		if err := s.UpdateClass(ctx, domain.Class{ID: id, Name: "high warden"}, nil); err != nil {
			t.Fatalf("update: %v", err)
		}
		_, gotTiers, err = s.GetClass(ctx, id)
		if err != nil || len(gotTiers) != 0 {
			t.Fatalf("tiers after update = %+v, err %v", gotTiers, err)
		}
		if err := s.DeleteClass(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, _, err := s.GetClass(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("hero", func(t *testing.T) {
		s := openTemp(t)
		beast, err := s.CreateRace(ctx, domain.Race{Name: "beast"}, nil)
		if err != nil {
			t.Fatalf("create race: %v", err)
		}
		mage, err := s.CreateRace(ctx, domain.Race{Name: "mage"}, nil)
		if err != nil {
			t.Fatalf("create race: %v", err)
		}
		assassin, err := s.CreateClass(ctx, domain.Class{Name: "assassin"}, nil)
		if err != nil {
			t.Fatalf("create class: %v", err)
		}
		druid, err := s.CreateClass(ctx, domain.Class{Name: "druid"}, nil)
		if err != nil {
			t.Fatalf("create class: %v", err)
		}
		id, err := s.CreateHero(ctx, domain.Hero{
			Name: "shade", Cost: 3, Ability: "vanish", Notes: "tech vs summon",
			Races:   []domain.Race{{ID: beast}},
			Classes: []domain.Class{{ID: assassin}, {ID: druid}},
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := s.GetHero(ctx, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Name != "shade" || got.Cost != 3 || got.Ability != "vanish" || got.Notes != "tech vs summon" {
			t.Fatalf("get = %+v", got)
		}
		if len(got.Races) != 1 || got.Races[0].ID != beast || got.Races[0].Name != "beast" {
			t.Fatalf("races = %+v", got.Races)
		}
		if len(got.Classes) != 2 || got.Classes[0].ID != assassin || got.Classes[1].ID != druid {
			t.Fatalf("classes = %+v", got.Classes)
		}
		byName, err := s.GetHeroByName(ctx, "shade")
		if err != nil || byName.ID != id {
			t.Fatalf("get by name = %+v, err %v", byName, err)
		}
		if _, err := s.GetHeroByName(ctx, "ghost"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("missing name err = %v, want ErrNotFound", err)
		}
		heroes, err := s.ListHeroes(ctx)
		if err != nil || len(heroes) != 1 || len(heroes[0].Classes) != 2 {
			t.Fatalf("list = %+v, err %v", heroes, err)
		}
		if err := s.UpdateHero(ctx, domain.Hero{
			ID: id, Name: "shade", Cost: 4,
			Races:   []domain.Race{{ID: mage}},
			Classes: []domain.Class{{ID: assassin}},
		}); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err = s.GetHero(ctx, id)
		if err != nil || got.Cost != 4 {
			t.Fatalf("get after update = %+v, err %v", got, err)
		}
		if len(got.Races) != 1 || got.Races[0].ID != mage {
			t.Fatalf("races after update = %+v", got.Races)
		}
		if len(got.Classes) != 1 || got.Classes[0].ID != assassin {
			t.Fatalf("classes after update = %+v", got.Classes)
		}
		if err := s.DeleteHero(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.GetHero(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("item", func(t *testing.T) {
		s := openTemp(t)
		storm, err := s.CreateItem(ctx, domain.Item{Name: "storm core", Tier: 3}, nil)
		if err != nil {
			t.Fatalf("create component: %v", err)
		}
		id, err := s.CreateItem(ctx, domain.Item{Name: "orb", Tier: 2, Effect: "+hit"}, []int64{storm})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, comps, err := s.GetItem(ctx, id)
		if err != nil || got.Name != "orb" || got.Tier != 2 || got.Effect != "+hit" {
			t.Fatalf("get = %+v, err %v", got, err)
		}
		if len(comps) != 1 || comps[0] != storm {
			t.Fatalf("components = %v, want [%d]", comps, storm)
		}
		items, err := s.ListItems(ctx)
		if err != nil || len(items) != 2 {
			t.Fatalf("list = %+v, err %v", items, err)
		}
		if err := s.UpdateItem(ctx, domain.Item{ID: id, Name: "orb", Tier: 4, Effect: "+hit"}, nil); err != nil {
			t.Fatalf("update: %v", err)
		}
		_, comps, err = s.GetItem(ctx, id)
		if err != nil || len(comps) != 0 {
			t.Fatalf("components after update = %v, err %v", comps, err)
		}
		if err := s.DeleteItem(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, _, err := s.GetItem(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("relic", func(t *testing.T) {
		s := openTemp(t)
		id, err := s.CreateRelic(ctx, domain.Relic{Name: "tide bell", Effect: "heal on kill"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := s.GetRelic(ctx, id)
		if err != nil || got.Name != "tide bell" || got.Effect != "heal on kill" {
			t.Fatalf("get = %+v, err %v", got, err)
		}
		relics, err := s.ListRelics(ctx)
		if err != nil || len(relics) != 1 || relics[0].ID != id {
			t.Fatalf("list = %+v, err %v", relics, err)
		}
		if err := s.UpdateRelic(ctx, domain.Relic{ID: id, Name: "tide bell", Effect: "heal twice"}); err != nil {
			t.Fatalf("update: %v", err)
		}
		if got, err = s.GetRelic(ctx, id); err != nil || got.Effect != "heal twice" {
			t.Fatalf("get after update = %+v, err %v", got, err)
		}
		if err := s.DeleteRelic(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.GetRelic(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("patch", func(t *testing.T) {
		s := openTemp(t)
		id, err := s.CreatePatch(ctx, domain.Patch{Version: "8.0", ReleasedAt: "2026-10-01"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := s.GetPatch(ctx, id)
		if err != nil || got.Version != "8.0" || got.ReleasedAt != "2026-10-01" {
			t.Fatalf("get = %+v, err %v", got, err)
		}
		patches, err := s.ListPatches(ctx)
		if err != nil || len(patches) != 1 || patches[0].ID != id {
			t.Fatalf("list = %+v, err %v", patches, err)
		}
		if err := s.UpdatePatch(ctx, domain.Patch{ID: id, Version: "8.0", ReleasedAt: "2026-10-02"}); err != nil {
			t.Fatalf("update: %v", err)
		}
		if got, err = s.GetPatch(ctx, id); err != nil || got.ReleasedAt != "2026-10-02" {
			t.Fatalf("get after update = %+v, err %v", got, err)
		}
		if err := s.DeletePatch(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.GetPatch(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("pro", func(t *testing.T) {
		s := openTemp(t)
		id, err := s.CreatePro(ctx, domain.Pro{Name: "drift", Handle: "drifttt", PeakRank: "challenger"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := s.GetPro(ctx, id)
		if err != nil || got.Name != "drift" || got.Handle != "drifttt" || got.PeakRank != "challenger" {
			t.Fatalf("get = %+v, err %v", got, err)
		}
		pros, err := s.ListPros(ctx)
		if err != nil || len(pros) != 1 || pros[0].ID != id {
			t.Fatalf("list = %+v, err %v", pros, err)
		}
		if err := s.UpdatePro(ctx, domain.Pro{ID: id, Name: "drift", Handle: "drift2", PeakRank: "challenger"}); err != nil {
			t.Fatalf("update: %v", err)
		}
		if got, err = s.GetPro(ctx, id); err != nil || got.Handle != "drift2" {
			t.Fatalf("get after update = %+v, err %v", got, err)
		}
		if err := s.DeletePro(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.GetPro(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})
}

func TestUpdateCodex_StaleIdMapsToNotFound(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	err := s.UpdateRace(ctx, domain.Race{ID: 99, Name: "ghost race"}, nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update stale race err = %v, want ErrNotFound", err)
	}
	err = s.UpdatePro(ctx, domain.Pro{ID: 99, Name: "ghost"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update stale pro err = %v, want ErrNotFound", err)
	}
}

func TestUpdateHero_RewritesJunctions(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	warrior, err := s.CreateRace(ctx, domain.Race{Name: "warrior"}, nil)
	if err != nil {
		t.Fatalf("create race: %v", err)
	}
	beast, err := s.CreateRace(ctx, domain.Race{Name: "beast"}, nil)
	if err != nil {
		t.Fatalf("create race: %v", err)
	}
	knight, err := s.CreateClass(ctx, domain.Class{Name: "knight"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	druid, err := s.CreateClass(ctx, domain.Class{Name: "druid"}, nil)
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	id, err := s.CreateHero(ctx, domain.Hero{
		Name: "shade", Cost: 3,
		Races:   []domain.Race{{ID: warrior}},
		Classes: []domain.Class{{ID: knight}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	err = s.UpdateHero(ctx, domain.Hero{
		ID: id, Name: "shade", Cost: 3,
		Races:   []domain.Race{{ID: beast}},
		Classes: []domain.Class{{ID: druid}},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM hero_races WHERE hero_id = ?`, id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("hero_races rows = %d, err %v, want only the rewritten row", n, err)
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM hero_races WHERE hero_id = ? AND race_id = ?`, id, beast).Scan(&n); err != nil || n != 1 {
		t.Fatalf("beast junction rows = %d, err %v", n, err)
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM hero_classes WHERE hero_id = ? AND class_id = ?`, id, druid).Scan(&n); err != nil || n != 1 {
		t.Fatalf("druid junction rows = %d, err %v", n, err)
	}
}

func TestDeleteHero_InUseMapsToErrInUse(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if err := seed.Load(s.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// grim jaw sits in finalized lineup slots, so the lineup_slots foreign key fires
	// and the store maps it onto the delete confirm copy.
	h, err := s.GetHeroByName(ctx, "grim jaw")
	if err != nil {
		t.Fatalf("lookup grim jaw: %v", err)
	}
	err = s.DeleteHero(ctx, h.ID)
	if !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete hero in use err = %v, want ErrInUse", err)
	}
	if _, err := s.GetHero(ctx, h.ID); err != nil {
		t.Fatalf("hero must survive the refused delete: %v", err)
	}
}

func TestListHeroesWithStats_MatchesFixtureGolden(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if err := seed.Load(s.DB); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// All-time picks over the whole fixture. Finalized pro play gives every hero 8
	// picks at sum 36 (avg 4.5). The tail lineups add: 33 p1 heroes 1,5,9; 34 p1
	// heroes 1-6; 35 p2 heroes 7,8; 36 p3 heroes 9,10.
	want := map[string]struct {
		lineups int
		avg     float64
	}{
		"sky breaker": {10, 3.8}, "grim jaw": {9, 37.0 / 9}, "lord of sand": {9, 37.0 / 9},
		"iron warden": {9, 37.0 / 9}, "frost weaver": {10, 3.8}, "night fang": {9, 37.0 / 9},
		"ember cub": {9, 38.0 / 9}, "storm herald": {9, 38.0 / 9}, "veil dancer": {10, 4},
		"anvil monk": {9, 39.0 / 9}, "moss tender": {8, 4.5}, "gale piercer": {8, 4.5},
	}
	rows, err := s.ListHeroesWithStats(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(rows), len(want))
	}
	for _, r := range rows {
		w, ok := want[r.Hero.Name]
		if !ok {
			t.Fatalf("unexpected hero %q", r.Hero.Name)
		}
		if r.Lineups != w.lineups || math.Abs(r.AvgPlace-w.avg) > 1e-9 {
			t.Fatalf("%s = (%d, %v), want (%d, %v)", r.Hero.Name, r.Lineups, r.AvgPlace, w.lineups, w.avg)
		}
		if r.Hero.Name == "storm herald" && len(r.Hero.Races) != 2 {
			t.Fatalf("storm herald races = %+v, want mage and beast", r.Hero.Races)
		}
	}
}
