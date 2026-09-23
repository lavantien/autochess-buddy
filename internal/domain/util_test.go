package domain

import (
	"testing"

	"pgregory.net/rapid"
)

func TestNextSlotIndex_FillsLowestGap_PermutationInvariant(t *testing.T) {
	usedGen := rapid.SliceOfDistinct(rapid.IntRange(0, 13), func(i int) int { return i })
	rapid.Check(t, func(t *rapid.T) {
		used := usedGen.Draw(t, "used slot indices")
		slots := make([]Slot, len(used))
		for i, idx := range used {
			slots[i] = Slot{SlotIndex: idx}
		}
		want := NextSlotIndex(slots)
		perm := rapid.Permutation(slots).Draw(t, "slot order")
		if got := NextSlotIndex(perm); got != want {
			t.Fatalf("NextSlotIndex order dependent: %d in input order vs %d permuted", want, got)
		}
		seen := make(map[int]bool, len(used))
		for _, idx := range used {
			seen[idx] = true
		}
		for gap := 0; ; gap++ {
			if !seen[gap] {
				if want != gap {
					t.Fatalf("NextSlotIndex = %d, want lowest free gap %d", want, gap)
				}
				break
			}
		}
	})
}

func TestFirstFreePlacement_SkipsUsed_PermutationInvariant(t *testing.T) {
	usedGen := rapid.SliceOfDistinct(rapid.IntRange(1, 8), func(i int) int { return i })
	rapid.Check(t, func(t *rapid.T) {
		used := usedGen.Draw(t, "used placements")
		lineups := make([]Lineup, len(used))
		for i, p := range used {
			lineups[i] = Lineup{Placement: p}
		}
		want := FirstFreePlacement(lineups)
		perm := rapid.Permutation(lineups).Draw(t, "lineup order")
		if got := FirstFreePlacement(perm); got != want {
			t.Fatalf("FirstFreePlacement order dependent: %d in input order vs %d permuted", want, got)
		}
		seen := make(map[int]bool, len(used))
		for _, p := range used {
			seen[p] = true
		}
		if len(seen) == 8 {
			if want != 0 {
				t.Fatalf("full match: FirstFreePlacement = %d, want 0", want)
			}
			return
		}
		if want < 1 || want > 8 || seen[want] {
			t.Fatalf("FirstFreePlacement = %d, want smallest free placement in 1..8", want)
		}
		for p := 1; p < want; p++ {
			if !seen[p] {
				t.Fatalf("FirstFreePlacement = %d skips free placement %d", want, p)
			}
		}
	})
}
