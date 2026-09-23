package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// writeHeroLineages rewrites both junction tables from the hero's lineage slices.
func writeHeroLineages(ctx context.Context, tx *sql.Tx, h domain.Hero, id int64) error {
	raceIDs := make([]int64, len(h.Races))
	for i, r := range h.Races {
		raceIDs[i] = r.ID
	}
	classIDs := make([]int64, len(h.Classes))
	for i, c := range h.Classes {
		classIDs[i] = c.ID
	}
	if err := insertChildInts(ctx, tx, "hero_races", "hero_id", "race_id", id, raceIDs); err != nil {
		return err
	}
	return insertChildInts(ctx, tx, "hero_classes", "hero_id", "class_id", id, classIDs)
}

// listHeroRaces hydrates one hero's races, id ordered.
func listHeroRaces(ctx context.Context, db *sql.DB, heroID int64) ([]domain.Race, error) {
	rows, err := db.QueryContext(ctx, `SELECT race_id, name FROM hero_races JOIN races ON races.id = race_id WHERE hero_id = ? ORDER BY race_id`, heroID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Race, error) {
		var v domain.Race
		return v, r.Scan(&v.ID, &v.Name)
	})
}

// listHeroClasses hydrates one hero's classes, id ordered.
func listHeroClasses(ctx context.Context, db *sql.DB, heroID int64) ([]domain.Class, error) {
	rows, err := db.QueryContext(ctx, `SELECT class_id, name FROM hero_classes JOIN classes ON classes.id = class_id WHERE hero_id = ? ORDER BY class_id`, heroID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Class, error) {
		var v domain.Class
		return v, r.Scan(&v.ID, &v.Name)
	})
}

// getHero scans one hero row by the query and hydrates its lineages.
func (s *Store) getHero(ctx context.Context, query string, args ...any) (domain.Hero, error) {
	var h domain.Hero
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&h.ID, &h.Name, &h.Cost, &h.Ability, &h.Notes)
	if errors.Is(err, sql.ErrNoRows) {
		return h, domain.ErrNotFound
	}
	if err != nil {
		return h, err
	}
	if h.Races, err = listHeroRaces(ctx, s.DB, h.ID); err != nil {
		return h, err
	}
	h.Classes, err = listHeroClasses(ctx, s.DB, h.ID)
	return h, err
}

// CreateHero inserts a hero and its junctions in one tx.
func (s *Store) CreateHero(ctx context.Context, h domain.Hero) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO heroes (name, cost, ability, notes) VALUES (?, ?, ?, ?)`, h.Name, h.Cost, h.Ability, h.Notes)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return writeHeroLineages(ctx, tx, h, id)
	})
	return id, mapConstraint(err)
}

// GetHero returns one hero with its lineages.
func (s *Store) GetHero(ctx context.Context, id int64) (domain.Hero, error) {
	return s.getHero(ctx, `SELECT id, name, cost, ability, notes FROM heroes WHERE id = ?`, id)
}

// GetHeroByName is the free-text datalist fallback lookup.
func (s *Store) GetHeroByName(ctx context.Context, name string) (domain.Hero, error) {
	return s.getHero(ctx, `SELECT id, name, cost, ability, notes FROM heroes WHERE name = ?`, name)
}

// ListHeroes returns every hero with lineages, ordered by id. The codex is small, the
// per-hero junction reads stay plain.
func (s *Store) ListHeroes(ctx context.Context) ([]domain.Hero, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM heroes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	ids, err := scanRows(rows, func(r *sql.Rows) (int64, error) {
		var id int64
		return id, r.Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Hero, len(ids))
	for i, id := range ids {
		if out[i], err = s.GetHero(ctx, id); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UpdateHero rewrites the row and both junction tables in one tx.
func (s *Store) UpdateHero(ctx context.Context, h domain.Hero) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE heroes SET name = ?, cost = ?, ability = ?, notes = ? WHERE id = ?`, h.Name, h.Cost, h.Ability, h.Notes, h.ID)
		if err != nil {
			return err
		}
		if err := affected(res, "hero"); err != nil {
			return err
		}
		return writeHeroLineages(ctx, tx, h, h.ID)
	}))
}

// DeleteHero removes one hero, junctions cascade; the lineup_slots foreign key
// refuses heroes sitting in match history.
func (s *Store) DeleteHero(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM heroes WHERE id = ?`, id)
	return mapConstraint(err)
}

// HeroStats is one codex hero with its all-time read-only columns.
type HeroStats struct {
	Hero     domain.Hero
	Lineups  int
	AvgPlace float64
}

// ListHeroesWithStats joins the all-time pick count and average placement onto every
// hero, one plain sqlite group-by over all lineups, drafts included.
func (s *Store) ListHeroesWithStats(ctx context.Context) ([]HeroStats, error) {
	heroes, err := s.ListHeroes(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT hero_id, COUNT(*), AVG(lineups.placement) FROM lineup_slots JOIN lineups ON lineups.id = lineup_id GROUP BY hero_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	tallies := map[int64]HeroStats{}
	for rows.Next() {
		var id int64
		var hs HeroStats
		if err := rows.Scan(&id, &hs.Lineups, &hs.AvgPlace); err != nil {
			return nil, err
		}
		tallies[id] = hs
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]HeroStats, len(heroes))
	for i, h := range heroes {
		out[i] = HeroStats{Hero: h, Lineups: tallies[h.ID].Lineups, AvgPlace: tallies[h.ID].AvgPlace}
	}
	return out, nil
}
