package domain

// Read-model rows and filters shared by every layer. They live here, next to
// the entity types they embed, so the ui templates can render against them
// without importing the data layers that produce them (readme:100: each layer
// only talks to the layer directly below).

// Filter scopes an analytics view. Zero values mean all.
type Filter struct {
	PatchID int64
	Source  string
}

// HeroRow is one hero's performance over the filtered view.
type HeroRow struct {
	Hero                       Hero
	Picks, Top4, LineupsInView int
	AvgPlace                   float64
	Finishes                   [8]int // placement 1..8 histogram, index 0 unused
	// Rates, floor and vs-field are assembled by the analytics engine on top
	// of the counts.
	PickRate, Top4Rate, Floor, VsField float64
}

// SynergyRow is one race or class tier ladder rung over the filtered view.
type SynergyRow struct {
	Kind      string // "race" or "class"
	ID        int64
	Name      string
	TierCount int
	Lineups   int
	Lift      float64
	Finishes  [8]int
}

// ItemRow is one item's lift against its same-hero baseline.
type ItemRow struct {
	Item          Item
	SlotsWith     int
	LineupsInView int
	Lift          float64
	Finishes      [8]int
}

// RelicRow is one relic's lift over the view.
type RelicRow struct {
	Relic         Relic
	Lineups       int
	LineupsInView int
	Lift          float64
	Finishes      [8]int
}

// PlaceRow is the average networth at one placement.
type PlaceRow struct {
	Placement   int
	N           int
	AvgNetworth float64
}

// MatchFilter narrows the matches list. Zero values pass everything.
type MatchFilter struct {
	PatchID int64
	Source  string // "" | me | pro
	State   string // "" | draft | final
}

// MatchListRow is one matches-list row with its pip count and patch column.
type MatchListRow struct {
	Match        Match
	PatchVersion string
	LineupCount  int
}

// HeroStats is one codex hero with its all-time read-only columns.
type HeroStats struct {
	Hero     Hero
	Lineups  int
	AvgPlace float64
}
