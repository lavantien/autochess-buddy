package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lavantien/autochess-buddy/internal/domain"
	sqlite3 "github.com/mattn/go-sqlite3"
)

func now() int64 { return time.Now().UTC().Unix() }

// mapPlacement folds the lineups UNIQUE(match_id, placement) hit onto the conflict
// copy, placement supplied by the caller that knows what it attempted.
func mapPlacement(err error, placement int) error {
	var se sqlite3.Error
	if errors.As(err, &se) && se.ExtendedCode == sqlite3.ErrConstraintUnique {
		return &domain.PlacementConflictError{Placement: placement}
	}
	return err
}

// mapSlotTaken folds the lineup_slots UNIQUE(lineup_id, slot_index) hit onto
// friendly copy; it fires only when two adds race for the same board cell.
func mapSlotTaken(err error) error {
	var se sqlite3.Error
	if errors.As(err, &se) && se.ExtendedCode == sqlite3.ErrConstraintUnique {
		return domain.ErrSlotTaken
	}
	return err
}

// The writable guards refuse writes once a match is finalized: finalize locks
// counts and placements (readme route table), and every mutation path funnels
// through one of them inside its own tx.

func matchWritable(ctx context.Context, tx *sql.Tx, matchID int64) error {
	return finalizedAt(ctx, tx, `SELECT finalized_at FROM matches WHERE id = ?`, matchID)
}

func lineupWritable(ctx context.Context, tx *sql.Tx, lineupID int64) error {
	return finalizedAt(ctx, tx, `
		SELECT m.finalized_at FROM matches m
		JOIN lineups l ON l.match_id = m.id
		WHERE l.id = ?`, lineupID)
}

func slotWritable(ctx context.Context, tx *sql.Tx, slotID int64) error {
	return finalizedAt(ctx, tx, `
		SELECT m.finalized_at FROM matches m
		JOIN lineups l ON l.match_id = m.id
		JOIN lineup_slots s ON s.lineup_id = l.id
		WHERE s.id = ?`, slotID)
}

func finalizedAt(ctx context.Context, tx *sql.Tx, query string, id int64) error {
	var fin int64
	err := tx.QueryRowContext(ctx, query, id).Scan(&fin)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if fin > 0 {
		return domain.ErrFinalized
	}
	return nil
}

// CreateMatchShell inserts one draft match row and returns its id.
func (s *Store) CreateMatchShell(ctx context.Context, m domain.Match) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO matches (patch_id, played_at, source, notes, created_at, finalized_at)
			 VALUES (?, ?, ?, ?, ?, 0)`,
			m.PatchID, m.PlayedAt, m.Source, m.Notes, now())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

// AddLineup inserts a lineup with its starting slots and relics in one tx.
func (s *Store) AddLineup(ctx context.Context, cmd domain.AddLineupCmd) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := matchWritable(ctx, tx, cmd.MatchID); err != nil {
			return err
		}
		seen := make(map[int]bool, len(cmd.Slots))
		for _, sl := range cmd.Slots {
			if seen[sl.SlotIndex] {
				return domain.ErrSlotTaken
			}
			seen[sl.SlotIndex] = true
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO lineups (match_id, pro_id, label, placement, wins, draws, losses, networth, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cmd.MatchID, cmd.ProID, cmd.Label, cmd.Placement, cmd.Wins, cmd.Draws, cmd.Losses, cmd.Networth, now())
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		for _, sl := range cmd.Slots {
			sres, err := tx.ExecContext(ctx,
				`INSERT INTO lineup_slots (lineup_id, hero_id, slot_index, stars) VALUES (?, ?, ?, ?)`,
				id, sl.Hero.ID, sl.SlotIndex, sl.Stars)
			if err != nil {
				return err
			}
			slotID, err := sres.LastInsertId()
			if err != nil {
				return err
			}
			for _, it := range sl.Items {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO slot_items (slot_id, item_id) VALUES (?, ?)`, slotID, it.ID); err != nil {
					return err
				}
			}
		}
		for _, rid := range cmd.RelicIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO lineup_relics (lineup_id, relic_id) VALUES (?, ?)`, id, rid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, mapPlacement(mapConstraint(err), cmd.Placement)
	}
	return id, nil
}

// CopyLineup clones one lineup's board, keeps the label, frees the pro credit and
// the result scalars, and takes the first free placement. One tx.
func (s *Store) CopyLineup(ctx context.Context, lineupID int64) (int64, error) {
	var newID int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		var matchID int64
		var label string
		err := tx.QueryRowContext(ctx,
			`SELECT match_id, label FROM lineups WHERE id = ?`, lineupID).
			Scan(&matchID, &label)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := matchWritable(ctx, tx, matchID); err != nil {
			return err
		}
		var existing []domain.Lineup
		rows, err := tx.QueryContext(ctx, `SELECT placement FROM lineups WHERE match_id = ?`, matchID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var l domain.Lineup
			if err := rows.Scan(&l.Placement); err != nil {
				_ = rows.Close()
				return err
			}
			existing = append(existing, l)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		free := domain.FirstFreePlacement(existing)
		if free == 0 {
			return errors.New("all 8 placements in this match are already taken.") //nolint:staticcheck // user-facing copy
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO lineups (match_id, pro_id, label, placement, wins, draws, losses, networth, created_at)
			 VALUES (?, NULL, ?, ?, 0, 0, 0, 0, ?)`, matchID, label, free, now())
		if err != nil {
			return err
		}
		if newID, err = res.LastInsertId(); err != nil {
			return err
		}
		slotRows, err := tx.QueryContext(ctx,
			`SELECT id, hero_id, slot_index, stars FROM lineup_slots WHERE lineup_id = ? ORDER BY slot_index`, lineupID)
		if err != nil {
			return err
		}
		type boardSlot struct {
			id        int64
			heroID    int64
			slotIndex int
			stars     int
		}
		var board []boardSlot
		for slotRows.Next() {
			var b boardSlot
			if err := slotRows.Scan(&b.id, &b.heroID, &b.slotIndex, &b.stars); err != nil {
				_ = slotRows.Close()
				return err
			}
			board = append(board, b)
		}
		if err := slotRows.Err(); err != nil {
			_ = slotRows.Close()
			return err
		}
		for _, b := range board {
			sres, err := tx.ExecContext(ctx,
				`INSERT INTO lineup_slots (lineup_id, hero_id, slot_index, stars) VALUES (?, ?, ?, ?)`,
				newID, b.heroID, b.slotIndex, b.stars)
			if err != nil {
				return err
			}
			newSlot, err := sres.LastInsertId()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO slot_items (slot_id, item_id)
				 SELECT ?, item_id FROM slot_items WHERE slot_id = ?`, newSlot, b.id); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO lineup_relics (lineup_id, relic_id)
			 SELECT ?, relic_id FROM lineup_relics WHERE lineup_id = ?`, newID, lineupID)
		return err
	})
	return newID, err
}

// UpdateLineup rewrites one lineup's editable fields.
func (s *Store) UpdateLineup(ctx context.Context, l domain.Lineup) error {
	return mapPlacement(s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := lineupWritable(ctx, tx, l.ID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE lineups SET pro_id = ?, label = ?, placement = ?, wins = ?, draws = ?, losses = ?, networth = ?
			 WHERE id = ?`, l.ProID, l.Label, l.Placement, l.Wins, l.Draws, l.Losses, l.Networth, l.ID)
		if err != nil {
			return err
		}
		return affected(res, "lineup")
	}), l.Placement)
}

// DeleteLineup removes one lineup; slots, items and relics cascade.
func (s *Store) DeleteLineup(ctx context.Context, id int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := lineupWritable(ctx, tx, id); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM lineups WHERE id = ?`, id)
		if err != nil {
			return err
		}
		return affected(res, "lineup")
	})
}

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

// FinalizeMatch validates the source lineup rules and stamps the marker in one
// tx, so no lineup add can slip between validation and the stamp. A repeat
// finalize is idempotent and never moves the marker.
func (s *Store) FinalizeMatch(ctx context.Context, id int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var source string
		var fin int64
		err := tx.QueryRowContext(ctx,
			`SELECT source, finalized_at FROM matches WHERE id = ?`, id).Scan(&source, &fin)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if fin > 0 {
			return nil
		}
		var lineups []domain.Lineup
		rows, err := tx.QueryContext(ctx, `SELECT placement FROM lineups WHERE match_id = ?`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var l domain.Lineup
			if err := rows.Scan(&l.Placement); err != nil {
				_ = rows.Close()
				return err
			}
			lineups = append(lineups, l)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := domain.ValidateFinalize(domain.Match{Source: source}, lineups); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE matches SET finalized_at = ? WHERE id = ? AND finalized_at = 0`, now(), id)
		return err
	})
}

// affected turns a zero-row write into ErrNotFound.
func affected(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
	}
	return nil
}
