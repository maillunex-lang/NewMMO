package quest

import "context"

// Adapter exposes Engine methods in the shape expected by ws.QuestHandler.
type Adapter struct{ *Engine }

func (a Adapter) Accept(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Accept(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}

func (a Adapter) Interact(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Interact(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}

func (a Adapter) TurnIn(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.TurnIn(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}

func (a Adapter) SnapshotQuests(ctx context.Context, accountID string, characterID int64) (any, error) {
	return a.Engine.SnapshotQuests(ctx, accountID, characterID)
}
