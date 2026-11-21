package dialogue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Engine struct {
	pool *pgxpool.Pool
	cat  *Catalog
}

func NewEngine(pool *pgxpool.Pool, cat *Catalog) *Engine {
	return &Engine{pool: pool, cat: cat}
}

type Option struct {
	OptionID string `json:"option_id"`
	LabelKey string `json:"label_key"`
}

type NodeState struct {
	DialogueID string   `json:"dialogue_id"`
	NodeID     string   `json:"node_id"`
	Speaker    string   `json:"speaker,omitempty"`
	Text       string   `json:"text,omitempty"`
	Options    []Option `json:"options"`
	Revision   int64    `json:"revision"`
}

func (n NodeState) GetRevision() int64 { return n.Revision }

type Envelope struct {
	Type, RequestID string
	Payload         map[string]any
}

func (e *Engine) Open(ctx context.Context, accountID string, characterID int64, env Envelope) (NodeState, error) {
	npcID, _ := env.Payload["npc_id"].(string)
	dialogueID, _ := env.Payload["dialogue_id"].(string)
	if npcID == "" || dialogueID == "" || env.RequestID == "" {
		return NodeState{}, ErrBadRequest
	}
	flow, ok := e.cat.Flow(dialogueID)
	if !ok {
		return NodeState{}, ErrNotFound
	}
	var out NodeState
	err := e.withCharacter(ctx, accountID, characterID, dialogueID, func(tx pgx.Tx, rev int64, questState, contact string) error {
		_ = tx
		mode, lines := pickMode(flow, questState)
		if len(lines) == 0 {
			return ErrPrerequisiteNotMet
		}
		if !npcAllowed(npcID, contact, mode) {
			return ErrPrerequisiteNotMet
		}
		nodeID := lines[0]
		line, _ := e.cat.Line(nodeID)
		out = NodeState{
			DialogueID: dialogueID,
			NodeID:     nodeID,
			Speaker:    line.Speaker,
			Text:       line.Text,
			Options:    filterOptions(mode, questState, npcID, contact),
			Revision:   rev,
		}
		return nil
	})
	return out, err
}

func (e *Engine) Choice(ctx context.Context, accountID string, characterID int64, env Envelope) (NodeState, error) {
	nodeID, _ := env.Payload["node_id"].(string)
	optionID, _ := env.Payload["option_id"].(string)
	dialogueID, _ := env.Payload["dialogue_id"].(string)
	npcID, _ := env.Payload["npc_id"].(string)
	if nodeID == "" || optionID == "" || env.RequestID == "" {
		return NodeState{}, ErrBadRequest
	}
	if dialogueID == "" {
		dialogueID = dialogueFromNode(nodeID)
	}
	if dialogueID == "" {
		return NodeState{}, ErrBadRequest
	}
	payloadHash := hashPayload(env.Payload)
	var out NodeState
	err := e.withCharacter(ctx, accountID, characterID, dialogueID, func(tx pgx.Tx, rev int64, questState, contact string) error {
		if replay, err := e.checkIdempotency(ctx, tx, characterID, env, payloadHash, &out); err != nil {
			return err
		} else if replay {
			return nil
		}
		flow, ok := e.cat.Flow(dialogueID)
		if !ok {
			return ErrNotFound
		}
		if npcID == "" {
			npcID = contact
		}
		mode, lines := pickMode(flow, questState)
		legal := filterOptions(mode, questState, npcID, contact)
		if !optionAllowed(legal, optionID) {
			return ErrPrerequisiteNotMet
		}
		buildID := e.cat.BuildID()
		tag, err := tx.Exec(ctx, `
			INSERT INTO public.character_dialogue_choices
				(character_id, dialogue_id, node_id, option_id, content_build_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING`, characterID, dialogueID, nodeID, optionID, buildID)
		if err != nil {
			return ErrInternal
		}
		if tag.RowsAffected() == 0 {
			var existing string
			if err := tx.QueryRow(ctx, `
				SELECT option_id FROM public.character_dialogue_choices
				WHERE character_id=$1 AND dialogue_id=$2 AND node_id=$3`,
				characterID, dialogueID, nodeID).Scan(&existing); err != nil {
				return ErrInternal
			}
			if existing != optionID {
				return ErrStateConflict
			}
		}
		newRev := rev + 1
		if _, err := tx.Exec(ctx, `UPDATE public.characters SET state_revision=$1 WHERE character_id=$2`, newRev, characterID); err != nil {
			return ErrInternal
		}
		nextMode, nextLines := mode, lines
		switch optionID {
		case "ACCEPT":
			nextMode, nextLines = "progress", flow.Progress
		case "TURN_IN":
			nextMode, nextLines = "turnin", flow.TurnIn
			if len(nextLines) > 1 {
				// After turn-in choice, show DONE line when present.
				for _, id := range nextLines {
					if strings.Contains(id, ".DONE.") {
						nextLines = []string{id}
						break
					}
				}
			}
		case "CONTINUE", "ASK", "LATER":
			nextLines = advanceLines(lines, nodeID)
		}
		nextID := nodeID
		if len(nextLines) > 0 {
			nextID = nextLines[0]
		}
		line, _ := e.cat.Line(nextID)
		out = NodeState{
			DialogueID: dialogueID,
			NodeID:     nextID,
			Speaker:    line.Speaker,
			Text:       line.Text,
			Options:    filterOptions(nextMode, questState, npcID, contact),
			Revision:   newRev,
		}
		return e.storeIdempotency(ctx, tx, characterID, env, payloadHash, out)
	})
	return out, err
}

func pickMode(flow FlowDef, questState string) (string, []string) {
	switch questState {
	case "", "AVAILABLE":
		return "start", flow.Start
	case "READY_TO_TURN_IN":
		return "turnin", flow.TurnIn
	case "COMPLETED":
		return "progress", flow.Progress
	default:
		return "progress", flow.Progress
	}
}

func advanceLines(lines []string, current string) []string {
	for i, id := range lines {
		if id == current && i+1 < len(lines) {
			return lines[i+1:]
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return []string{lines[len(lines)-1]}
}

func npcAllowed(npcID, contact, mode string) bool {
	if contact == "" {
		return true
	}
	switch mode {
	case "start", "turnin":
		return npcID == contact
	default:
		return true
	}
}

func filterOptions(mode, questState, npcID, contact string) []Option {
	var out []Option
	switch mode {
	case "start":
		if questState == "" || questState == "AVAILABLE" {
			if contact == "" || npcID == contact {
				out = append(out, Option{OptionID: "ACCEPT", LabelKey: "ui.dialogue.accept"})
			}
		}
		out = append(out, Option{OptionID: "ASK", LabelKey: "ui.dialogue.ask"})
		out = append(out, Option{OptionID: "LATER", LabelKey: "ui.dialogue.later"})
	case "turnin":
		if questState == "READY_TO_TURN_IN" && (contact == "" || npcID == contact) {
			out = append(out, Option{OptionID: "TURN_IN", LabelKey: "ui.dialogue.turn_in"})
		}
		out = append(out, Option{OptionID: "ASK", LabelKey: "ui.dialogue.ask"})
	default:
		out = append(out, Option{OptionID: "CONTINUE", LabelKey: "ui.dialogue.continue"})
		out = append(out, Option{OptionID: "ASK", LabelKey: "ui.dialogue.ask"})
	}
	return out
}

func optionAllowed(opts []Option, id string) bool {
	for _, o := range opts {
		if o.OptionID == id {
			return true
		}
	}
	return false
}

func dialogueFromNode(nodeID string) string {
	parts := strings.Split(nodeID, ".")
	if len(parts) >= 2 && (strings.HasPrefix(parts[1], "MQ-") || strings.HasPrefix(parts[1], "SQ-")) {
		return parts[1]
	}
	return ""
}

func (e *Engine) withCharacter(ctx context.Context, accountID string, characterID int64, dialogueID string, fn func(pgx.Tx, int64, string, string) error) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return ErrInternal
	}
	defer tx.Rollback(ctx)

	var rev int64
	err = tx.QueryRow(ctx, `
		SELECT state_revision FROM public.characters
		WHERE account_id=$1 AND character_id=$2 FOR UPDATE`, accountID, characterID).Scan(&rev)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return ErrInternal
	}

	questState := "AVAILABLE"
	var stored string
	err = tx.QueryRow(ctx, `
		SELECT state FROM public.character_quest_states
		WHERE character_id=$1 AND quest_id=$2`, characterID, dialogueID).Scan(&stored)
	if err == nil {
		questState = stored
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ErrInternal
	}

	contact := contactFor(dialogueID)
	if err := fn(tx, rev, questState, contact); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func contactFor(questID string) string {
	switch questID {
	case "MQ-01-03", "MQ-01-04":
		return "NPC-102"
	case "MQ-01-01", "MQ-01-02", "MQ-01-05":
		return "NPC-802"
	case "SQ-01-001":
		return "NPC-802"
	case "SQ-01-002":
		return "NPC-801"
	case "SQ-01-003", "SQ-01-004":
		return "NPC-102"
	case "SQ-01-005":
		return "NPC-906"
	default:
		if strings.HasPrefix(questID, "MQ-01-") {
			return "NPC-802"
		}
		return ""
	}
}

func (e *Engine) checkIdempotency(ctx context.Context, tx pgx.Tx, characterID int64, env Envelope, payloadHash string, dest any) (bool, error) {
	requestUUID, err := parseUUID(env.RequestID)
	if err != nil {
		return false, ErrBadRequest
	}
	var storedHash string
	var result []byte
	err = tx.QueryRow(ctx, `
		SELECT payload_sha256, result FROM public.idempotency_keys
		WHERE character_id=$1 AND request_id=$2`, characterID, requestUUID).Scan(&storedHash, &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, ErrInternal
	}
	if storedHash != payloadHash {
		return false, ErrIdempotencyConflict
	}
	if err := json.Unmarshal(result, dest); err != nil {
		return false, ErrInternal
	}
	return true, nil
}

func (e *Engine) storeIdempotency(ctx context.Context, tx pgx.Tx, characterID int64, env Envelope, payloadHash string, result any) error {
	requestUUID, err := parseUUID(env.RequestID)
	if err != nil {
		return ErrBadRequest
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return ErrInternal
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.idempotency_keys (character_id, request_id, message_type, payload_sha256, result)
		VALUES ($1,$2,$3,$4,$5::jsonb)`, characterID, requestUUID, env.Type, payloadHash, string(raw))
	if err != nil {
		return ErrInternal
	}
	return nil
}

func hashPayload(payload map[string]any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func parseUUID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return "", ErrBadRequest
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", ErrBadRequest
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return "", ErrBadRequest
			}
		}
	}
	return strings.ToLower(s), nil
}
