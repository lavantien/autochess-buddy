package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidateFinalize_MeNeedsExactlyOneLineup(t *testing.T) {
	me := Match{Source: "me"}
	if err := ValidateFinalize(me, nil); !errors.Is(err, ErrMeFinalize) {
		t.Errorf("zero lineups: err = %v, want ErrMeFinalize", err)
	}
	if err := ValidateFinalize(me, []Lineup{{Label: "only"}}); err != nil {
		t.Errorf("one lineup: err = %v, want nil", err)
	}
	if err := ValidateFinalize(me, []Lineup{{Label: "a"}, {Label: "b"}}); !errors.Is(err, ErrMeFinalize) {
		t.Errorf("two lineups: err = %v, want ErrMeFinalize", err)
	}
}

func TestValidateFinalize_ProNeedsEightDistinctPlacements(t *testing.T) {
	pro := Match{Source: "pro"}
	if err := ValidateFinalize(pro, nil); !errors.Is(err, ErrProFinalize) {
		t.Errorf("zero lineups: err = %v, want ErrProFinalize", err)
	}
	if err := ValidateFinalize(pro, lineupsPlaced(1, 2, 3, 4, 5, 6, 7)); !errors.Is(err, ErrProFinalize) {
		t.Errorf("seven lineups: err = %v, want ErrProFinalize", err)
	}
	if err := ValidateFinalize(pro, lineupsPlaced(1, 2, 3, 4, 6, 7, 8)); !errors.Is(err, ErrProFinalize) {
		t.Errorf("seven lineups with a gap: err = %v, want ErrProFinalize", err)
	}
	if err := ValidateFinalize(pro, lineupsPlaced(1, 2, 3, 4, 5, 6, 7, 8)); err != nil {
		t.Errorf("eight distinct: err = %v, want nil", err)
	}
}

func TestValidateFinalize_ProDuplicatePlacementsRejected(t *testing.T) {
	pro := Match{Source: "pro"}
	err := ValidateFinalize(pro, lineupsPlaced(1, 1, 2, 3, 4, 5, 6, 7))
	if !errors.Is(err, ErrProFinalize) {
		t.Errorf("duplicate placements: err = %v, want ErrProFinalize", err)
	}
}

// lineupsPlaced builds one lineup per given placement.
func lineupsPlaced(placements ...int) []Lineup {
	out := make([]Lineup, len(placements))
	for i, p := range placements {
		out[i] = Lineup{Label: "l", Placement: p}
	}
	return out
}

func TestValidateAddSlot_RejectsThirteenthSlot(t *testing.T) {
	if err := ValidateAddSlot(nil); err != nil {
		t.Errorf("empty board: err = %v, want nil", err)
	}
	eleven := make([]Slot, 11)
	if err := ValidateAddSlot(eleven); err != nil {
		t.Errorf("eleven slots: err = %v, want nil", err)
	}
	twelve := make([]Slot, 12)
	if err := ValidateAddSlot(twelve); !errors.Is(err, ErrSlotCap) {
		t.Errorf("twelve slots: err = %v, want ErrSlotCap", err)
	}
}

func TestValidateAddItem_RejectsSeventhItem(t *testing.T) {
	if err := ValidateAddItem(nil); err != nil {
		t.Errorf("no items: err = %v, want nil", err)
	}
	five := []int64{1, 2, 3, 4, 5}
	if err := ValidateAddItem(five); err != nil {
		t.Errorf("five items: err = %v, want nil", err)
	}
	six := []int64{1, 2, 3, 4, 5, 6}
	if err := ValidateAddItem(six); !errors.Is(err, ErrItemCap) {
		t.Errorf("six items: err = %v, want ErrItemCap", err)
	}
}

func TestValidateHero_CostRange(t *testing.T) {
	for cost := 1; cost <= 5; cost++ {
		h := Hero{Cost: cost, Races: []Race{{Name: "r"}}, Classes: []Class{{Name: "c"}}}
		if err := ValidateHero(h); err != nil {
			t.Errorf("cost %d: err = %v, want nil", cost, err)
		}
	}
	for _, cost := range []int{-1, 0, 6, 100} {
		h := Hero{Cost: cost, Races: []Race{{Name: "r"}}, Classes: []Class{{Name: "c"}}}
		if err := ValidateHero(h); !errors.Is(err, ErrCostRange) {
			t.Errorf("cost %d: err = %v, want ErrCostRange", cost, err)
		}
	}
}

func TestValidateHero_LineageOneToTwo(t *testing.T) {
	race := Race{Name: "r"}
	class := Class{Name: "c"}
	base := Hero{Cost: 1}
	if err := ValidateHero(base); !errors.Is(err, ErrLineageCount) {
		t.Errorf("no lineages: err = %v, want ErrLineageCount", err)
	}
	threeRaces := base
	threeRaces.Races = []Race{race, race, race}
	threeRaces.Classes = []Class{class}
	if err := ValidateHero(threeRaces); !errors.Is(err, ErrLineageCount) {
		t.Errorf("three races: err = %v, want ErrLineageCount", err)
	}
	threeClasses := base
	threeClasses.Races = []Race{race}
	threeClasses.Classes = []Class{class, class, class}
	if err := ValidateHero(threeClasses); !errors.Is(err, ErrLineageCount) {
		t.Errorf("three classes: err = %v, want ErrLineageCount", err)
	}
	dual := base
	dual.Races = []Race{race, race}
	dual.Classes = []Class{class, class}
	if err := ValidateHero(dual); err != nil {
		t.Errorf("dual dual: err = %v, want nil", err)
	}
}

func TestParsePlayedAt_RoundTrip(t *testing.T) {
	epoch, err := ParsePlayedAt("1970-01-01T00:00:00")
	if err != nil || epoch != 0 {
		t.Fatalf("epoch input: got (%d, %v), want (0, nil)", epoch, err)
	}
	in := "2026-09-23T14:30"
	got, err := ParsePlayedAt(in)
	if err != nil {
		t.Fatalf("ParsePlayedAt(%q): %v", in, err)
	}
	want := time.Date(2026, 9, 23, 14, 30, 0, 0, time.UTC).Unix()
	if got != want {
		t.Fatalf("ParsePlayedAt(%q) = %d, want %d (utc)", in, got, want)
	}
	if back := time.Unix(got, 0).UTC().Format("2006-01-02T15:04"); back != in {
		t.Errorf("round trip: %q -> %d -> %q, want %q", in, got, back, in)
	}
	if _, err := ParsePlayedAt("2026-09-23T14:30:45"); err != nil {
		t.Errorf("seconds layout: %v", err)
	}
}

func TestParsePlayedAt_RejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "not a time", "2026-13-45T99:99", "23/09/2026 14:30", "2026-09-23"} {
		if got, err := ParsePlayedAt(in); err == nil {
			t.Errorf("ParsePlayedAt(%q) = (%d, nil), want error", in, got)
		}
	}
}
