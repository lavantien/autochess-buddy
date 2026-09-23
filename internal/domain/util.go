package domain

import (
	"fmt"
	"time"
)

// NextSlotIndex returns the smallest free slot index, duplicate heroes and gaps included.
func NextSlotIndex(existing []Slot) int {
	used := make(map[int]bool, len(existing))
	for _, s := range existing {
		used[s.SlotIndex] = true
	}
	for i := 0; ; i++ {
		if !used[i] {
			return i
		}
	}
}

// FirstFreePlacement returns the smallest free placement in 1..8, 0 when the match is full.
func FirstFreePlacement(existing []Lineup) int {
	used := make(map[int]bool, len(existing))
	for _, l := range existing {
		used[l.Placement] = true
	}
	for p := 1; p <= 8; p++ {
		if !used[p] {
			return p
		}
	}
	return 0
}

var playedAtLayouts = []string{"2006-01-02T15:04:05", "2006-01-02T15:04"}

// ParsePlayedAt parses a datetime-local input as utc unixepoch seconds.
func ParsePlayedAt(in string) (int64, error) {
	for _, layout := range playedAtLayouts {
		t, err := time.Parse(layout, in)
		if err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("cannot parse played at %q", in)
}
