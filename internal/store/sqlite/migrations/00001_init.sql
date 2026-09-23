-- +goose Up
-- v1 schema
CREATE TABLE patches (
  id INTEGER PRIMARY KEY, version TEXT NOT NULL UNIQUE, released_at TEXT NOT NULL);
CREATE TABLE races (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE race_tiers (
  race_id INTEGER NOT NULL REFERENCES races(id),
  count INTEGER NOT NULL, effect TEXT NOT NULL,
  PRIMARY KEY (race_id, count));
CREATE TABLE classes (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE class_tiers (
  class_id INTEGER NOT NULL REFERENCES classes(id),
  count INTEGER NOT NULL, effect TEXT NOT NULL,
  PRIMARY KEY (class_id, count));
CREATE TABLE heroes (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  cost INTEGER NOT NULL CHECK (cost BETWEEN 1 AND 5),
  ability TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '');
CREATE TABLE hero_races (
  hero_id INTEGER NOT NULL REFERENCES heroes(id) ON DELETE CASCADE,
  race_id INTEGER NOT NULL REFERENCES races(id),
  PRIMARY KEY (hero_id, race_id));
CREATE TABLE hero_classes (
  hero_id INTEGER NOT NULL REFERENCES heroes(id) ON DELETE CASCADE,
  class_id INTEGER NOT NULL REFERENCES classes(id),
  PRIMARY KEY (hero_id, class_id));
CREATE TABLE items (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  tier INTEGER NOT NULL,
  effect TEXT NOT NULL DEFAULT '');
CREATE TABLE item_recipes (
  result_id INTEGER NOT NULL REFERENCES items(id),
  component_id INTEGER NOT NULL REFERENCES items(id),
  PRIMARY KEY (result_id, component_id));
CREATE TABLE relics (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  effect TEXT NOT NULL DEFAULT '');
CREATE TABLE pros (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL DEFAULT '', peak_rank TEXT NOT NULL DEFAULT '');
CREATE TABLE matches (
  id INTEGER PRIMARY KEY,
  patch_id INTEGER NOT NULL REFERENCES patches(id),
  played_at INTEGER NOT NULL,
  source TEXT NOT NULL CHECK (source IN ('me','pro')),
  notes TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  -- deviation 1: draft/final state needs its own marker, analytics filters finalized_at > 0
  finalized_at INTEGER NOT NULL DEFAULT 0);
CREATE TABLE lineups (
  id INTEGER PRIMARY KEY,
  match_id INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  pro_id INTEGER REFERENCES pros(id),
  label TEXT NOT NULL,
  placement INTEGER NOT NULL CHECK (placement BETWEEN 1 AND 8),
  wins INTEGER NOT NULL DEFAULT 0, draws INTEGER NOT NULL DEFAULT 0,
  losses INTEGER NOT NULL DEFAULT 0, networth INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  UNIQUE (match_id, placement));
CREATE TABLE lineup_slots (
  id INTEGER PRIMARY KEY,
  lineup_id INTEGER NOT NULL REFERENCES lineups(id) ON DELETE CASCADE,
  hero_id INTEGER NOT NULL REFERENCES heroes(id),
  slot_index INTEGER NOT NULL CHECK (slot_index >= 0),
  stars INTEGER NOT NULL CHECK (stars BETWEEN 1 AND 3),
  UNIQUE (lineup_id, slot_index));
CREATE TABLE slot_items (
  slot_id INTEGER NOT NULL REFERENCES lineup_slots(id) ON DELETE CASCADE,
  item_id INTEGER NOT NULL REFERENCES items(id));
CREATE TABLE lineup_relics (
  lineup_id INTEGER NOT NULL REFERENCES lineups(id) ON DELETE CASCADE,
  relic_id INTEGER NOT NULL REFERENCES relics(id),
  PRIMARY KEY (lineup_id, relic_id));
CREATE INDEX idx_slots_lineup ON lineup_slots(lineup_id);
CREATE INDEX idx_slots_hero ON lineup_slots(hero_id);
CREATE INDEX idx_lineups_match ON lineups(match_id);
CREATE INDEX idx_slot_items_item ON slot_items(item_id);
CREATE INDEX idx_hero_races_race ON hero_races(race_id);
CREATE INDEX idx_hero_classes_class ON hero_classes(class_id);
