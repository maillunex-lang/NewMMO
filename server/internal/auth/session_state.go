package auth

import (
	"context"
	"strconv"
)

// WSState is the authoritative character snapshot and active content metadata
// for the account and character returned by ConsumeTicket.
type WSState struct {
	Character
	XP, Gold, Revision int64
	BuildID            string
	MinClientVersion   string
}

func (s *Store) LoadWSState(ctx context.Context, accountID string, characterID int64) (WSState, error) {
	var state WSState
	err := s.pool.QueryRow(ctx, `
		SELECT c.name, c.class_id, c.xp, c.gold, c.state_revision,
			coalesce(b.build_id, ''), coalesce(b.minimum_client_version, '')
		FROM public.characters c
		CROSS JOIN public.content_active a
		LEFT JOIN public.content_builds b ON b.build_id = a.build_id AND b.status = 'published'
		WHERE a.id = 1 AND c.account_id = $1 AND c.character_id = $2`, accountID, characterID).
		Scan(&state.Name, &state.ClassID, &state.XP, &state.Gold, &state.Revision,
			&state.BuildID, &state.MinClientVersion)
	if err != nil {
		return WSState{}, ErrInternal
	}
	state.ID = strconv.FormatInt(characterID, 10)
	state.Level = Level(state.XP)
	return state, nil
}
