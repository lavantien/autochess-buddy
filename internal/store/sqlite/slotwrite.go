package sqlite

import (
	"context"
	"database/sql"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// AddSlot puts one hero cell at the lowest free board index.
func (s *Store) AddSlot(ctx context.Context, lineupID, heroID int64, stars int) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := lineupWritable(ctx, tx, lineupID); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx,
			`SELECT slot_index FROM lineup_slots WHERE lineup_id = ?`, lineupID)
		if err != nil {
			return err
		}
		var existing []domain.Slot
		for rows.Next() {
			var sl domain.Slot
			if err := rows.Scan(&sl.SlotIndex); err != nil {
				_ = rows.Close()
				return err
			}
			existing = append(existing, sl)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO lineup_slots (lineup_id, hero_id, slot_index, stars) VALUES (?, ?, ?, ?)`,
			lineupID, heroID, domain.NextSlotIndex(existing), stars)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, mapSlotTaken(mapConstraint(err))
}

// SetSlotStars rewrites one slot's star count.
func (s *Store) SetSlotStars(ctx context.Context, slotID int64, stars int) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := slotWritable(ctx, tx, slotID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE lineup_slots SET stars = ? WHERE id = ?`, stars, slotID)
		if err != nil {
			return err
		}
		return affected(res, "slot")
	})
}

// DeleteSlot removes one board cell; its items cascade.
func (s *Store) DeleteSlot(ctx context.Context, slotID int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := slotWritable(ctx, tx, slotID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM lineup_slots WHERE id = ?`, slotID)
		if err != nil {
			return err
		}
		return affected(res, "slot")
	})
}

// AddSlotItem attaches one item to one slot.
func (s *Store) AddSlotItem(ctx context.Context, slotID, itemID int64) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := slotWritable(ctx, tx, slotID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO slot_items (slot_id, item_id) VALUES (?, ?)`, slotID, itemID)
		return err
	}))
}

// RemoveSlotItem detaches one item from one slot.
func (s *Store) RemoveSlotItem(ctx context.Context, slotID, itemID int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := slotWritable(ctx, tx, slotID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM slot_items WHERE slot_id = ? AND item_id = ?`, slotID, itemID)
		if err != nil {
			return err
		}
		return affected(res, "slot item")
	})
}

// AddLineupRelic attaches one relic to one lineup.
func (s *Store) AddLineupRelic(ctx context.Context, lineupID, relicID int64) error {
	return mapConstraint(s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := lineupWritable(ctx, tx, lineupID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO lineup_relics (lineup_id, relic_id) VALUES (?, ?)`, lineupID, relicID)
		return err
	}))
}

// RemoveLineupRelic detaches one relic from one lineup.
func (s *Store) RemoveLineupRelic(ctx context.Context, lineupID, relicID int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := lineupWritable(ctx, tx, lineupID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM lineup_relics WHERE lineup_id = ? AND relic_id = ?`, lineupID, relicID)
		if err != nil {
			return err
		}
		return affected(res, "relic")
	})
}
