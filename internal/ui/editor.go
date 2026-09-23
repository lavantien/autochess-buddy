package ui

import (
	"strconv"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/service"
)

// AddFormState carries the add-lineup form's raw values so 422 rerenders preserve
// exactly what was typed.
type AddFormState struct {
	Placement, ProID, Label, Wins, Draws, Losses, Networth string
	Errs                                                   []domain.FieldError
}

// ResetAddForm prefills the placement with the first free one and clears the rest.
func ResetAddForm(view service.EditorView) AddFormState {
	return AddFormState{Placement: strconv.Itoa(domain.FirstFreePlacement(view.Lineups))}
}

func (st AddFormState) err(field string) string {
	for _, fe := range st.Errs {
		if fe.Field == field {
			return fe.Msg
		}
	}
	return ""
}

func (st AddFormState) firstErrField() string {
	if len(st.Errs) == 0 {
		return ""
	}
	return st.Errs[0].Field
}

// HeroFormState carries the add-hero row's raw values.
type HeroFormState struct {
	Hero, Stars string
	Err         string
	Reset       bool
}

// EditFormState carries the edit disclosure's raw values.
type EditFormState struct {
	Placement, ProID, Label, Wins, Draws, Losses, Networth string
	Errs                                                   []domain.FieldError
}

func (st EditFormState) err(field string) string {
	for _, fe := range st.Errs {
		if fe.Field == field {
			return fe.Msg
		}
	}
	return ""
}

func (st EditFormState) firstErrField() string {
	if len(st.Errs) == 0 {
		return ""
	}
	return st.Errs[0].Field
}

// SlotFormState tells the grid which slot details render open and which slot
// control carries an inline error.
type SlotFormState struct {
	OpenSlotID int64
	BadSlotID  int64
	BadField   string // "stars" or "item"
	BadMsg     string
}

// RelicState carries the relic disclosure's render state.
type RelicState struct {
	Open bool
	Err  string
}

// requiredTotal is 8 lineups for pro matches, 1 for mine.
func requiredTotal(source string) int {
	if source == "me" {
		return 1
	}
	return 8
}

// displayName is the pro name when a pro is credited, otherwise the label.
func displayName(view service.EditorView, l domain.Lineup) string {
	if l.ProID != nil {
		for _, p := range view.Pros {
			if p.ID == *l.ProID {
				return p.Name
			}
		}
	}
	return l.Label
}

// peakLabel renders "peak: rank" when the lineup credits a pro.
func peakLabel(view service.EditorView, l domain.Lineup) string {
	if l.ProID == nil {
		return ""
	}
	for _, p := range view.Pros {
		if p.ID == *l.ProID {
			return "peak: " + p.PeakRank
		}
	}
	return ""
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// slotAt returns the slot occupying board cell i, or nil for the placeholder.
func slotAt(l domain.Lineup, i int) *domain.Slot {
	for j := range l.Slots {
		if l.Slots[j].SlotIndex == i {
			return &l.Slots[j]
		}
	}
	return nil
}
