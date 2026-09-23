package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// CreateItem inserts an item and its recipe rows in one tx.
func (s *Store) CreateItem(ctx context.Context, it domain.Item, components []int64) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO items (name, tier, effect) VALUES (?, ?, ?)`, it.Name, it.Tier, it.Effect)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return insertChildInts(ctx, tx, "item_recipes", "result_id", "component_id", id, components)
	})
	return id, mapConstraint(err)
}

// GetItem returns one item and its component ids.
func (s *Store) GetItem(ctx context.Context, id int64) (domain.Item, []int64, error) {
	var it domain.Item
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, tier, effect FROM items WHERE id = ?`, id).Scan(&it.ID, &it.Name, &it.Tier, &it.Effect)
	if errors.Is(err, sql.ErrNoRows) {
		return it, nil, domain.ErrNotFound
	}
	if err != nil {
		return it, nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT component_id FROM item_recipes WHERE result_id = ? ORDER BY component_id`, id)
	if err != nil {
		return it, nil, err
	}
	comps, err := scanRows(rows, func(r *sql.Rows) (int64, error) {
		var c int64
		return c, r.Scan(&c)
	})
	return it, comps, err
}

// ListItems returns every item ordered by id.
func (s *Store) ListItems(ctx context.Context) ([]domain.Item, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, tier, effect FROM items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Item, error) {
		var v domain.Item
		return v, r.Scan(&v.ID, &v.Name, &v.Tier, &v.Effect)
	})
}

// UpdateItem rewrites the row and the recipes in one tx.
func (s *Store) UpdateItem(ctx context.Context, it domain.Item, components []int64) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE items SET name = ?, tier = ?, effect = ? WHERE id = ?`, it.Name, it.Tier, it.Effect, it.ID); err != nil {
			return err
		}
		return insertChildInts(ctx, tx, "item_recipes", "result_id", "component_id", it.ID, components)
	}))
}

// DeleteItem clears the owned recipes, then the row. Component of another item
// refuses through that recipe's foreign key.
func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_recipes WHERE result_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
		return err
	}))
}

// CreateRelic inserts a relic.
func (s *Store) CreateRelic(ctx context.Context, r domain.Relic) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO relics (name, effect) VALUES (?, ?)`, r.Name, r.Effect)
	if err != nil {
		return 0, mapConstraint(err)
	}
	return res.LastInsertId()
}

// GetRelic returns one relic.
func (s *Store) GetRelic(ctx context.Context, id int64) (domain.Relic, error) {
	var r domain.Relic
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, effect FROM relics WHERE id = ?`, id).Scan(&r.ID, &r.Name, &r.Effect)
	if errors.Is(err, sql.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	return r, err
}

// ListRelics returns every relic ordered by id.
func (s *Store) ListRelics(ctx context.Context) ([]domain.Relic, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, effect FROM relics ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Relic, error) {
		var v domain.Relic
		return v, r.Scan(&v.ID, &v.Name, &v.Effect)
	})
}

// UpdateRelic rewrites one relic.
func (s *Store) UpdateRelic(ctx context.Context, r domain.Relic) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE relics SET name = ?, effect = ? WHERE id = ?`, r.Name, r.Effect, r.ID)
	return mapConstraint(err)
}

// DeleteRelic removes one relic; lineups keeping it in history refuse the delete.
func (s *Store) DeleteRelic(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM relics WHERE id = ?`, id)
	return mapConstraint(err)
}

// CreatePatch inserts a patch.
func (s *Store) CreatePatch(ctx context.Context, p domain.Patch) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO patches (version, released_at) VALUES (?, ?)`, p.Version, p.ReleasedAt)
	if err != nil {
		return 0, mapConstraint(err)
	}
	return res.LastInsertId()
}

// GetPatch returns one patch.
func (s *Store) GetPatch(ctx context.Context, id int64) (domain.Patch, error) {
	var p domain.Patch
	err := s.DB.QueryRowContext(ctx, `SELECT id, version, released_at FROM patches WHERE id = ?`, id).Scan(&p.ID, &p.Version, &p.ReleasedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}

// ListPatches returns every patch ordered by id.
func (s *Store) ListPatches(ctx context.Context) ([]domain.Patch, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, version, released_at FROM patches ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Patch, error) {
		var v domain.Patch
		return v, r.Scan(&v.ID, &v.Version, &v.ReleasedAt)
	})
}

// UpdatePatch rewrites one patch.
func (s *Store) UpdatePatch(ctx context.Context, p domain.Patch) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE patches SET version = ?, released_at = ? WHERE id = ?`, p.Version, p.ReleasedAt, p.ID)
	return mapConstraint(err)
}

// DeletePatch removes one patch; matches keep their history and refuse the delete.
func (s *Store) DeletePatch(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM patches WHERE id = ?`, id)
	return mapConstraint(err)
}

// CreatePro inserts a pro.
func (s *Store) CreatePro(ctx context.Context, p domain.Pro) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO pros (name, handle, peak_rank) VALUES (?, ?, ?)`, p.Name, p.Handle, p.PeakRank)
	if err != nil {
		return 0, mapConstraint(err)
	}
	return res.LastInsertId()
}

// GetPro returns one pro.
func (s *Store) GetPro(ctx context.Context, id int64) (domain.Pro, error) {
	var p domain.Pro
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, handle, peak_rank FROM pros WHERE id = ?`, id).Scan(&p.ID, &p.Name, &p.Handle, &p.PeakRank)
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}

// ListPros returns every pro ordered by id.
func (s *Store) ListPros(ctx context.Context) ([]domain.Pro, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, handle, peak_rank FROM pros ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.Pro, error) {
		var v domain.Pro
		return v, r.Scan(&v.ID, &v.Name, &v.Handle, &v.PeakRank)
	})
}

// UpdatePro rewrites one pro.
func (s *Store) UpdatePro(ctx context.Context, p domain.Pro) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE pros SET name = ?, handle = ?, peak_rank = ? WHERE id = ?`, p.Name, p.Handle, p.PeakRank, p.ID)
	return mapConstraint(err)
}

// DeletePro removes one pro; lineups keeping the credit refuse the delete.
func (s *Store) DeletePro(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM pros WHERE id = ?`, id)
	return mapConstraint(err)
}
