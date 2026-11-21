package dialogue

import "context"

// Adapter exposes Engine methods in the shape expected by ws.DialogueHandler.
type Adapter struct{ *Engine }

func (a Adapter) Open(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Open(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}

func (a Adapter) Choice(ctx context.Context, accountID string, characterID int64, typ, requestID string, payload map[string]any) (any, error) {
	return a.Engine.Choice(ctx, accountID, characterID, Envelope{Type: typ, RequestID: requestID, Payload: payload})
}
