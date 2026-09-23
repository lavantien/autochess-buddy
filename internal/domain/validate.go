package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Error vocabulary, user-facing copy verbatim from frontend-design.md.
var (
	ErrInUse        = errors.New("existing matches keep their history.")
	ErrSlotCap      = errors.New("a lineup holds at most 12 heroes.")
	ErrItemCap      = errors.New("a slot holds at most 6 items.")
	ErrStarsRange   = errors.New("stars must be between 1 and 3.")
	ErrCostRange    = errors.New("cost must be between 1 and 5.")
	ErrLineageCount = errors.New("a hero carries 1 to 2 races and 1 to 2 classes.")
	ErrNotFound     = errors.New("not found.")
	// Copy for the me case is not in the spec, mirror of the pro wording, flagged in the stage report.
	ErrMeFinalize  = errors.New("my matches need exactly 1 lineup before they can be finalized.")
	ErrProFinalize = errors.New("a pro match needs 8 lineups with placements 1 through 8 before it can be finalized.")
)

// PlacementConflictError reports a UNIQUE(match_id, placement) hit, the store maps the
// sqlite violation onto it.
type PlacementConflictError struct {
	Placement int
}

func (e *PlacementConflictError) Error() string {
	return fmt.Sprintf("placement %d is already used by another lineup in this match.", e.Placement)
}

// ErrNoHeroNamed reports free-text hero input that misses the codex.
func ErrNoHeroNamed(name string) error {
	return fmt.Errorf("no hero named %s in the codex. add it first.", name)
}

// FieldError names one bad form field and the fix.
type FieldError struct {
	Field string
	Msg   string
}

// ValidationError is a set of field errors for one form.
type ValidationError []FieldError

func (v ValidationError) Error() string {
	parts := make([]string, len(v))
	for i, fe := range v {
		parts[i] = fe.Field + ": " + fe.Msg
	}
	return strings.Join(parts, ", ")
}

// ValidateHero checks cost and lineage counts.
func ValidateHero(h Hero) error {
	if h.Cost < 1 || h.Cost > 5 {
		return ErrCostRange
	}
	if len(h.Races) < 1 || len(h.Races) > 2 || len(h.Classes) < 1 || len(h.Classes) > 2 {
		return ErrLineageCount
	}
	return nil
}

// ValidateFinalize checks a match carries the lineups its source demands.
// Source must be "me" or "pro"; the schema CHECK guarantees it for stored rows.
func ValidateFinalize(m Match, lineups []Lineup) error {
	if m.Source != "me" && m.Source != "pro" {
		return fmt.Errorf("source must be me or pro, got %q", m.Source)
	}
	if m.Source == "me" {
		if len(lineups) != 1 {
			return ErrMeFinalize
		}
		return nil
	}
	if len(lineups) != 8 {
		return ErrProFinalize
	}
	seen := make(map[int]bool, 8)
	for _, l := range lineups {
		if l.Placement < 1 || l.Placement > 8 || seen[l.Placement] {
			return ErrProFinalize
		}
		seen[l.Placement] = true
	}
	return nil
}

// ValidateAddSlot rejects a hero add onto a full board.
func ValidateAddSlot(existing []Slot) error {
	if len(existing) >= 12 {
		return ErrSlotCap
	}
	return nil
}

// ValidateAddItem rejects an item add onto a full slot.
func ValidateAddItem(existing []int64) error {
	if len(existing) >= 6 {
		return ErrItemCap
	}
	return nil
}

// ValidateSlotStars checks the stars input range.
func ValidateSlotStars(stars int) error {
	if stars < 1 || stars > 3 {
		return ErrStarsRange
	}
	return nil
}
