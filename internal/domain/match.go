package domain

// Match is one played game. FinalizedAt is deviation 1: 0 keeps it in draft, > 0 marks finalized.
type Match struct {
	ID          int64
	PatchID     int64
	PlayedAt    int64  // utc unixepoch seconds
	Source      string // "me" or "pro"
	Notes       string
	CreatedAt   int64
	FinalizedAt int64
}

// Lineup is one of the 8 placements in a pro match or the single placement in a me match.
type Lineup struct {
	ID        int64
	MatchID   int64
	ProID     *int64
	Label     string
	Placement int
	Wins      int
	Draws     int
	Losses    int
	Networth  int
	CreatedAt int64
	Slots     []Slot
	Relics    []Relic
}

// Slot is one board position in a lineup, its hero, stars, and items.
type Slot struct {
	ID        int64
	LineupID  int64
	Hero      Hero
	SlotIndex int
	Stars     int
	Items     []Item
}

// AddLineupCmd carries a new lineup from the form into the store.
type AddLineupCmd struct {
	MatchID   int64
	ProID     *int64
	Label     string
	Placement int
	Wins      int
	Draws     int
	Losses    int
	Networth  int
	Slots     []Slot
	RelicIDs  []int64
}
