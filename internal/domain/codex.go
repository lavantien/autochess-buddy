package domain

// Patch is a game version.
type Patch struct {
	ID         int64
	Version    string
	ReleasedAt string
}

// Race is one hero race in the codex.
type Race struct {
	ID   int64
	Name string
}

// Class is one hero class in the codex.
type Class struct {
	ID   int64
	Name string
}

// Tier is one rung of a race or class tier ladder, LineageID targets the owning race or class.
type Tier struct {
	LineageID int64
	Count     int
	Effect    string
}

// Hero is a codex hero with its lineage attached.
type Hero struct {
	ID      int64
	Name    string
	Cost    int
	Ability string
	Notes   string
	Races   []Race
	Classes []Class
}

// Item is a codex item.
type Item struct {
	ID     int64
	Name   string
	Tier   int
	Effect string
}

// Relic is a codex relic.
type Relic struct {
	ID     int64
	Name   string
	Effect string
}

// Pro is a tracked professional player.
type Pro struct {
	ID       int64
	Name     string
	Handle   string
	PeakRank string
}
