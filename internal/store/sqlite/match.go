package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// ListMatches returns matches newest first with lineup counts and patch versions.
func (s *Store) ListMatches(ctx context.Context, f domain.MatchFilter) ([]domain.MatchListRow, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.PatchID != 0 {
		where = append(where, "m.patch_id = ?")
		args = append(args, f.PatchID)
	}
	if f.Source != "" {
		where = append(where, "m.source = ?")
		args = append(args, f.Source)
	}
	switch f.State {
	case "draft":
		where = append(where, "m.finalized_at = 0")
	case "final":
		where = append(where, "m.finalized_at > 0")
	case "":
	default:
		return nil, errors.New("state filter must be draft or final")
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT m.id, m.patch_id, m.played_at, m.source, m.notes, m.created_at, m.finalized_at,
		        p.version, COUNT(l.id)
		 FROM matches m
		 JOIN patches p ON p.id = m.patch_id
		 LEFT JOIN lineups l ON l.match_id = m.id
		 WHERE `+strings.Join(where, " AND ")+`
		 GROUP BY m.id
		 ORDER BY m.played_at DESC, m.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (domain.MatchListRow, error) {
		var v domain.MatchListRow
		return v, r.Scan(&v.Match.ID, &v.Match.PatchID, &v.Match.PlayedAt, &v.Match.Source,
			&v.Match.Notes, &v.Match.CreatedAt, &v.Match.FinalizedAt, &v.PatchVersion, &v.LineupCount)
	})
}

// GetMatch returns one match with its lineups deep-assembled: slots in board order
// with their heroes and items, plus lineup relics.
func (s *Store) GetMatch(ctx context.Context, id int64) (domain.Match, []domain.Lineup, error) {
	var m domain.Match
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, patch_id, played_at, source, notes, created_at, finalized_at FROM matches WHERE id = ?`, id).
		Scan(&m.ID, &m.PatchID, &m.PlayedAt, &m.Source, &m.Notes, &m.CreatedAt, &m.FinalizedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return m, nil, domain.ErrNotFound
	}
	if err != nil {
		return m, nil, err
	}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, pro_id, label, placement, wins, draws, losses, networth, created_at
		 FROM lineups WHERE match_id = ? ORDER BY placement`, id)
	if err != nil {
		return m, nil, err
	}
	lineups, err := scanRows(rows, func(r *sql.Rows) (domain.Lineup, error) {
		var l domain.Lineup
		var pro sql.NullInt64
		err := r.Scan(&l.ID, &pro, &l.Label, &l.Placement, &l.Wins, &l.Draws, &l.Losses, &l.Networth, &l.CreatedAt)
		if err == nil && pro.Valid {
			p := pro.Int64
			l.ProID = &p
		}
		return l, err
	})
	if err != nil {
		return m, nil, err
	}
	if err := s.attachSlots(ctx, lineups); err != nil {
		return m, nil, err
	}
	return m, lineups, s.attachRelics(ctx, lineups)
}

// attachSlots loads every slot of every lineup, board order, with heroes and items.
func (s *Store) attachSlots(ctx context.Context, lineups []domain.Lineup) error {
	ids := make([]int64, len(lineups))
	byID := make(map[int64]*domain.Lineup, len(lineups))
	for i := range lineups {
		ids[i] = lineups[i].ID
		byID[lineups[i].ID] = &lineups[i]
	}
	slots, err := s.querySlots(ctx, ids)
	if err != nil {
		return err
	}
	slotIDs := make([]int64, len(slots))
	for i := range slots {
		slotIDs[i] = slots[i].slot.ID
	}
	items, err := s.querySlotItems(ctx, slotIDs)
	if err != nil {
		return err
	}
	itemsBySlot := map[int64][]domain.Item{}
	for _, it := range items {
		itemsBySlot[it.slotID] = append(itemsBySlot[it.slotID], it.item)
	}
	for i := range slots {
		l := byID[slots[i].lineupID]
		slots[i].slot.Items = itemsBySlot[slots[i].slot.ID]
		l.Slots = append(l.Slots, slots[i].slot)
	}
	return nil
}

type slotRow struct {
	lineupID int64
	slot     domain.Slot
}

func (s *Store) querySlots(ctx context.Context, lineupIDs []int64) ([]slotRow, error) {
	if len(lineupIDs) == 0 {
		return nil, nil
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT ls.id, ls.lineup_id, ls.slot_index, ls.stars, h.id, h.name, h.cost
		 FROM lineup_slots ls JOIN heroes h ON h.id = ls.hero_id
		 WHERE ls.lineup_id IN (`+placeholders(len(lineupIDs))+`)
		 ORDER BY ls.lineup_id, ls.slot_index`, int64s(lineupIDs)...)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (slotRow, error) {
		var sr slotRow
		return sr, r.Scan(&sr.slot.ID, &sr.lineupID, &sr.slot.SlotIndex, &sr.slot.Stars,
			&sr.slot.Hero.ID, &sr.slot.Hero.Name, &sr.slot.Hero.Cost)
	})
}

type itemRow2 struct {
	slotID int64
	item   domain.Item
}

func (s *Store) querySlotItems(ctx context.Context, slotIDs []int64) ([]itemRow2, error) {
	if len(slotIDs) == 0 {
		return nil, nil
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT si.slot_id, i.id, i.name, i.tier, i.effect
		 FROM slot_items si JOIN items i ON i.id = si.item_id
		 WHERE si.slot_id IN (`+placeholders(len(slotIDs))+`)
		 ORDER BY si.slot_id, i.id`, int64s(slotIDs)...)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(r *sql.Rows) (itemRow2, error) {
		var ir itemRow2
		return ir, r.Scan(&ir.slotID, &ir.item.ID, &ir.item.Name, &ir.item.Tier, &ir.item.Effect)
	})
}

// attachRelics loads every lineup relic, id ordered.
func (s *Store) attachRelics(ctx context.Context, lineups []domain.Lineup) error {
	ids := make([]int64, len(lineups))
	for i := range lineups {
		ids[i] = lineups[i].ID
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT lr.lineup_id, r.id, r.name, r.effect
		 FROM lineup_relics lr JOIN relics r ON r.id = lr.relic_id
		 WHERE lr.lineup_id IN (`+placeholders(len(ids))+`)
		 ORDER BY lr.lineup_id, r.id`, int64s(ids)...)
	if err != nil {
		return err
	}
	pairs, err := scanRows(rows, func(r *sql.Rows) (struct {
		lineupID int64
		relic    domain.Relic
	}, error) {
		var p struct {
			lineupID int64
			relic    domain.Relic
		}
		return p, r.Scan(&p.lineupID, &p.relic.ID, &p.relic.Name, &p.relic.Effect)
	})
	if err != nil {
		return err
	}
	byLineup := map[int64][]domain.Relic{}
	for _, p := range pairs {
		byLineup[p.lineupID] = append(byLineup[p.lineupID], p.relic)
	}
	for i := range lineups {
		lineups[i].Relics = byLineup[lineups[i].ID]
	}
	return nil
}

// placeholders builds one ?, ?, ? list of n marks.
func placeholders(n int) string {
	marks := make([]string, n)
	for i := range marks {
		marks[i] = "?"
	}
	return strings.Join(marks, ", ")
}

// int64s widens ids into scan args.
func int64s(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
