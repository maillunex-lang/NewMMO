package inventory

import "context"

type Adapter struct{ *Engine }

func (a Adapter) SnapshotInventory(ctx context.Context, accountID string, characterID int64) (any, error) {
	return a.Engine.Snapshot(ctx, accountID, characterID)
}

func (a Adapter) Move(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Move(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}

func (a Adapter) Equip(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Equip(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}

func (a Adapter) Unequip(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Unequip(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}
