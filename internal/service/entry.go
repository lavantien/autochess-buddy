package service

import (
	"context"

	"github.com/lavantien/autochess-buddy/internal/domain"
	"github.com/lavantien/autochess-buddy/internal/store/sqlite"
)

// EditorView is everything the match editor screen renders: the match, its lineups
// deep-assembled, and the codex option lists the forms offer.
type EditorView struct {
	Match        domain.Match
	PatchVersion string
	Lineups      []domain.Lineup
	Heroes       []domain.Hero
	Items        []domain.Item
	Relics       []domain.Relic
	Pros         []domain.Pro
}

// EntryService owns the match entry flows. Codex stays handler->store direct
// (readme:207); everything touching a match goes through here so validation and
// friendly errors happen once.
type EntryService struct {
	St *sqlite.Store
}

// CreateMatch validates the shell and inserts the draft row.
func (s EntryService) CreateMatch(ctx context.Context, m domain.Match) (int64, error) {
	if m.Source != "me" && m.Source != "pro" {
		return 0, domain.ValidationError{{Field: "source", Msg: "source must be me or pro."}}
	}
	if _, err := s.St.GetPatch(ctx, m.PatchID); err != nil {
		return 0, err
	}
	return s.St.CreateMatchShell(ctx, m)
}

// AddLineup validates and stores a new lineup, or when copyFrom is nonzero copies
// that lineup instead. It returns the reloaded editor view and the new lineup's
// id (0 on error) so the editor can focus the fresh card.
func (s EntryService) AddLineup(ctx context.Context, matchID int64, cmd domain.AddLineupCmd, copyFrom int64) (EditorView, int64, error) {
	m, _, err := s.St.GetMatch(ctx, matchID)
	if err != nil {
		return EditorView{}, 0, err
	}
	if m.FinalizedAt > 0 {
		return EditorView{}, 0, domain.ValidationError{{Field: "match", Msg: "this match is finalized and can no longer be edited."}}
	}
	if copyFrom != 0 {
		newID, err := s.copyLineup(ctx, matchID, copyFrom)
		if err != nil {
			return EditorView{}, 0, err
		}
		view, verr := s.Editor(ctx, matchID)
		return view, newID, verr
	}
	cmd.MatchID = matchID
	if err := s.validateLineup(ctx, matchID, cmd); err != nil {
		return EditorView{}, 0, err
	}
	newID, err := s.St.AddLineup(ctx, cmd)
	if err != nil {
		return EditorView{}, 0, err
	}
	view, verr := s.Editor(ctx, matchID)
	return view, newID, verr
}

// validateLineup runs the friendly pre-checks: placement free, heroes resolvable,
// board and item caps, star range, scalars non-negative.
func (s EntryService) validateLineup(ctx context.Context, matchID int64, cmd domain.AddLineupCmd) error {
	var errs domain.ValidationError
	if cmd.Placement < 1 || cmd.Placement > 8 {
		errs = append(errs, domain.FieldError{Field: "placement", Msg: "placement must be between 1 and 8."})
	}
	if cmd.Wins < 0 || cmd.Draws < 0 || cmd.Losses < 0 || cmd.Networth < 0 {
		errs = append(errs, domain.FieldError{Field: "record", Msg: "wins, draws, losses and networth cannot be negative."})
	}
	if len(cmd.Slots) > 12 {
		errs = append(errs, domain.FieldError{Field: "board", Msg: domain.ErrSlotCap.Error()})
	}
	if cmd.ProID != nil {
		if _, err := s.St.GetPro(ctx, *cmd.ProID); err != nil {
			errs = append(errs, domain.FieldError{Field: "pro", Msg: "pick a pro from the list."})
		}
	}
	slots := make([]domain.Slot, len(cmd.Slots))
	for i := range cmd.Slots {
		sl := &cmd.Slots[i]
		if err := domain.ValidateSlotStars(sl.Stars); err != nil {
			errs = append(errs, domain.FieldError{Field: "stars", Msg: err.Error()})
		}
		if len(sl.Items) > 6 {
			errs = append(errs, domain.FieldError{Field: "items", Msg: domain.ErrItemCap.Error()})
		}
		if sl.Hero.ID == 0 && sl.Hero.Name != "" {
			h, err := s.St.GetHeroByName(ctx, sl.Hero.Name)
			if err != nil {
				// Spec copy must surface bare for the editor's inline error.
				return domain.ErrNoHeroNamed(sl.Hero.Name)
			}
			sl.Hero = h
		}
		if sl.Hero.ID != 0 {
			if _, err := s.St.GetHero(ctx, sl.Hero.ID); err != nil {
				return domain.ErrNoHeroNamed(sl.Hero.Name)
			}
		}
		slots[i] = *sl
	}
	for _, rid := range cmd.RelicIDs {
		if _, err := s.St.GetRelic(ctx, rid); err != nil {
			errs = append(errs, domain.FieldError{Field: "relic", Msg: "pick a relic from the list."})
		}
	}
	if len(errs) > 0 {
		return errs
	}
	_, lineups, err := s.St.GetMatch(ctx, matchID)
	if err != nil {
		return err
	}
	for _, l := range lineups {
		if l.Placement == cmd.Placement {
			return &domain.PlacementConflictError{Placement: cmd.Placement}
		}
	}
	return nil
}

// copyLineup guards the full-match case with friendly copy before delegating.
func (s EntryService) copyLineup(ctx context.Context, matchID, lineupID int64) (int64, error) {
	_, lineups, err := s.St.GetMatch(ctx, matchID)
	if err != nil {
		return 0, err
	}
	for _, l := range lineups {
		if l.ID == lineupID {
			return s.St.CopyLineup(ctx, lineupID)
		}
	}
	return 0, domain.ErrNotFound
}

// FinalizeMatch validates the lineup rules then stamps the marker.
func (s EntryService) FinalizeMatch(ctx context.Context, id int64) error {
	m, lineups, err := s.St.GetMatch(ctx, id)
	if err != nil {
		return err
	}
	if err := domain.ValidateFinalize(m, lineups); err != nil {
		return err
	}
	return s.St.FinalizeMatch(ctx, id)
}

// Editor reloads the full editor view.
func (s EntryService) Editor(ctx context.Context, matchID int64) (EditorView, error) {
	m, lineups, err := s.St.GetMatch(ctx, matchID)
	if err != nil {
		return EditorView{}, err
	}
	p, err := s.St.GetPatch(ctx, m.PatchID)
	if err != nil {
		return EditorView{}, err
	}
	heroes, err := s.St.ListHeroes(ctx)
	if err != nil {
		return EditorView{}, err
	}
	items, err := s.St.ListItems(ctx)
	if err != nil {
		return EditorView{}, err
	}
	relics, err := s.St.ListRelics(ctx)
	if err != nil {
		return EditorView{}, err
	}
	pros, err := s.St.ListPros(ctx)
	if err != nil {
		return EditorView{}, err
	}
	return EditorView{Match: m, PatchVersion: p.Version, Lineups: lineups, Heroes: heroes, Items: items, Relics: relics, Pros: pros}, nil
}
