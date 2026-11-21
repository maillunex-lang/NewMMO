package loot

import "context"

type Adapter struct{ *Engine }

func (a Adapter) Claim(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Claim(ctx, accountID, characterID, typ, requestID, payload)
}
