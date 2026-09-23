package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lavantien/autochess-buddy/internal/domain"
	sqlite3 "github.com/mattn/go-sqlite3"
)

// mapConstraint folds sqlite foreign key violations onto the delete confirm copy so
// handlers render exact spec copy without knowing the schema.
func mapConstraint(err error) error {
	var se sqlite3.Error
	if errors.As(err, &se) && se.ExtendedCode == sqlite3.ErrConstraintForeignKey {
		return fmt.Errorf("%w: %v", domain.ErrInUse, se)
	}
	return err
}

// scanRows folds each row through scan until the result set drains.
func scanRows[T any](rows *sql.Rows, scan func(*sql.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// insertTiers rewrites the tier ladder of one race or class.
func insertTiers(ctx context.Context, tx *sql.Tx, table, parentCol string, parent int64, tiers []domain.Tier) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+parentCol+` = ?`, parent); err != nil {
		return err
	}
	for _, t := range tiers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+parentCol+`, count, effect) VALUES (?, ?, ?)`, parent, t.Count, t.Effect); err != nil {
			return err
		}
	}
	return nil
}

// listTiers reads one tier ladder, count ascending.
func listTiers(ctx context.Context, db *sql.DB, table, parentCol string, parent int64) ([]domain.Tier, error) {
	rows, err := db.QueryContext(ctx, `SELECT count, effect FROM `+table+` WHERE `+parentCol+` = ? ORDER BY count`, parent)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Tier, error) {
		var t domain.Tier
		t.LineageID = parent
		return t, r.Scan(&t.Count, &t.Effect)
	})
}

// insertChildInts rewrites the child id rows of one parent, shared by the hero
// junctions and the item recipes.
func insertChildInts(ctx context.Context, tx *sql.Tx, table, parentCol, childCol string, parent int64, children []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+parentCol+` = ?`, parent); err != nil {
		return err
	}
	for _, c := range children {
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+parentCol+`, `+childCol+`) VALUES (?, ?)`, parent, c); err != nil {
			return err
		}
	}
	return nil
}

// CreateRace inserts a race and its tier ladder in one tx.
func (s *Store) CreateRace(ctx context.Context, r domain.Race, tiers []domain.Tier) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO races (name) VALUES (?)`, r.Name)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return insertTiers(ctx, tx, "race_tiers", "race_id", id, tiers)
	})
	return id, mapConstraint(err)
}

// GetRace returns one race with its tier ladder.
func (s *Store) GetRace(ctx context.Context, id int64) (domain.Race, []domain.Tier, error) {
	var r domain.Race
	err := s.DB.QueryRowContext(ctx, `SELECT id, name FROM races WHERE id = ?`, id).Scan(&r.ID, &r.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return r, nil, domain.ErrNotFound
	}
	if err != nil {
		return r, nil, err
	}
	tiers, err := listTiers(ctx, s.DB, "race_tiers", "race_id", id)
	return r, tiers, err
}

// ListRaces returns every race ordered by id.
func (s *Store) ListRaces(ctx context.Context) ([]domain.Race, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name FROM races ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Race, error) {
		var v domain.Race
		return v, r.Scan(&v.ID, &v.Name)
	})
}

// UpdateRace rewrites the name and the tier ladder in one tx.
func (s *Store) UpdateRace(ctx context.Context, r domain.Race, tiers []domain.Tier) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE races SET name = ? WHERE id = ?`, r.Name, r.ID); err != nil {
			return err
		}
		return insertTiers(ctx, tx, "race_tiers", "race_id", r.ID, tiers)
	}))
}

// DeleteRace clears the owned tier ladder, then the row. Lineage in use by a hero
// still refuses through the junction foreign key.
func (s *Store) DeleteRace(ctx context.Context, id int64) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM race_tiers WHERE race_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM races WHERE id = ?`, id)
		return err
	}))
}

// CreateClass inserts a class and its tier ladder in one tx.
func (s *Store) CreateClass(ctx context.Context, c domain.Class, tiers []domain.Tier) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO classes (name) VALUES (?)`, c.Name)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return insertTiers(ctx, tx, "class_tiers", "class_id", id, tiers)
	})
	return id, mapConstraint(err)
}

// GetClass returns one class with its tier ladder.
func (s *Store) GetClass(ctx context.Context, id int64) (domain.Class, []domain.Tier, error) {
	var c domain.Class
	err := s.DB.QueryRowContext(ctx, `SELECT id, name FROM classes WHERE id = ?`, id).Scan(&c.ID, &c.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil, domain.ErrNotFound
	}
	if err != nil {
		return c, nil, err
	}
	tiers, err := listTiers(ctx, s.DB, "class_tiers", "class_id", id)
	return c, tiers, err
}

// ListClasses returns every class ordered by id.
func (s *Store) ListClasses(ctx context.Context) ([]domain.Class, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name FROM classes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Class, error) {
		var v domain.Class
		return v, r.Scan(&v.ID, &v.Name)
	})
}

// UpdateClass rewrites the name and the tier ladder in one tx.
func (s *Store) UpdateClass(ctx context.Context, c domain.Class, tiers []domain.Tier) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE classes SET name = ? WHERE id = ?`, c.Name, c.ID); err != nil {
			return err
		}
		return insertTiers(ctx, tx, "class_tiers", "class_id", c.ID, tiers)
	}))
}

// DeleteClass clears the owned tier ladder, then the row.
func (s *Store) DeleteClass(ctx context.Context, id int64) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM class_tiers WHERE class_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM classes WHERE id = ?`, id)
		return err
	}))
}
