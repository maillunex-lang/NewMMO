package quest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func internalErr(err error) error {
	if err == nil {
		return ErrInternal
	}
	return fmt.Errorf("%w: %v", ErrInternal, err)
}

type Engine struct {
	pool   *pgxpool.Pool
	cat    *Catalog
	regions RegionConfig
	now    func() time.Time
}

func NewEngine(pool *pgxpool.Pool, cat *Catalog, regions RegionConfig) *Engine {
	return &Engine{pool: pool, cat: cat, regions: regions, now: time.Now}
}

type AcceptResult struct {
	QuestID  string `json:"quest_id"`
	State    string `json:"state"`
	Revision int64  `json:"revision"`
	OK       bool   `json:"ok"`
}

type ProgressDelta struct {
	QuestID     string `json:"quest_id"`
	ObjectiveID string `json:"objective_id"`
	Count       int    `json:"count"`
	Need        int    `json:"need"`
	Revision    int64  `json:"revision"`
	Ready       bool   `json:"ready,omitempty"`
}

type RewardGranted struct {
	QuestID           string         `json:"quest_id"`
	RewardClaimID     string         `json:"reward_claim_id"`
	InventoryRevision int64          `json:"inventory_revision"`
	XPTotal           int64          `json:"xp_total"`
	GoldTotal         int64          `json:"gold_total"`
	Level             int            `json:"level"`
	Reputation        map[string]int `json:"reputation"`
	Evidence          []string       `json:"evidence"`
}

type ActiveQuest struct {
	QuestID         string `json:"quest_id"`
	State           string `json:"state"`
	ContentBuildID  string `json:"content_build_id"`
	Objectives      []struct {
		ObjectiveID string `json:"objective_id"`
		Have        int    `json:"have"`
		Need        int    `json:"need"`
	} `json:"objectives"`
}

type QuestsScope struct {
	Active    []ActiveQuest `json:"active"`
	Completed []string      `json:"completed"`
	Available []string      `json:"available"`
}

type Envelope struct {
	Type      string
	RequestID string
	Payload   map[string]any
}

func (e *Engine) Accept(ctx context.Context, accountID string, characterID int64, env Envelope) (AcceptResult, error) {
	questID, _ := env.Payload["quest_id"].(string)
	buildID, _ := env.Payload["content_build_id"].(string)
	if questID == "" || buildID == "" || env.RequestID == "" {
		return AcceptResult{}, ErrBadRequest
	}
	payloadHash := hashPayload(env.Payload)
	var out AcceptResult
	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, char characterRow) error {
		if replay, err := e.checkIdempotency(ctx, tx, characterID, env, payloadHash, &out); err != nil {
			return err
		} else if replay {
			return nil
		}
		if buildID != e.cat.BuildID() {
			return ErrContentMismatch
		}
		def, ok := e.cat.Quest(questID)
		if !ok {
			return ErrNotFound
		}
		if !e.regions.RegionOpen(def.RegionID, accountID, e.now()) {
			return ErrRegionClosed
		}
		level := auth.Level(char.XP)
		if level < def.MinLevel {
			return ErrLevelTooLow
		}
		for _, pre := range def.Prereqs {
			st, err := e.questState(ctx, tx, characterID, pre)
			if err != nil {
				return err
			}
			if st != "COMPLETED" {
				return ErrPrerequisiteNotMet
			}
		}
		if len(def.Prereqs) == 0 && questID == "MQ-01-01" {
			if !char.Flags["TUTORIAL-COMPLETE"] {
				return ErrPrerequisiteNotMet
			}
		}
		existing, err := e.questState(ctx, tx, characterID, questID)
		if err != nil {
			return err
		}
		if existing != "" {
			return ErrStateConflict
		}
		if def.Kind == "quest_main" {
			n, err := e.countStates(ctx, tx, characterID, "quest_main", []string{"ACTIVE", "READY_TO_TURN_IN"})
			if err != nil {
				return err
			}
			if n > 0 {
				return ErrStateConflict
			}
		} else if def.Kind == "quest_side" {
			n, err := e.countStates(ctx, tx, characterID, "quest_side", []string{"ACTIVE", "READY_TO_TURN_IN"})
			if err != nil {
				return err
			}
			if n >= 5 {
				return ErrStateConflict
			}
		}
		rev := char.Revision + 1
		_, err = tx.Exec(ctx, `
			INSERT INTO public.character_quest_states
				(character_id, quest_id, state, objective_progress, content_build_id, revision)
			VALUES ($1, $2, 'ACTIVE', '{}'::jsonb, $3, $4)`,
			characterID, questID, buildID, rev)
		if err != nil {
			return internalErr(err)
		}
		flag := FlagName(questID, "START")
		if err := e.setFlag(ctx, tx, characterID, flag, true); err != nil {
			return err
		}
		if err := e.bumpRevision(ctx, tx, characterID, rev); err != nil {
			return err
		}
		out = AcceptResult{QuestID: questID, State: "ACTIVE", Revision: rev, OK: true}
		return e.storeIdempotency(ctx, tx, characterID, env, payloadHash, out)
	})
	return out, err
}

func (e *Engine) Interact(ctx context.Context, accountID string, characterID int64, env Envelope) (ProgressDelta, error) {
	event, _ := env.Payload["event"].(string)
	targetID, _ := env.Payload["target_id"].(string)
	instanceID, _ := env.Payload["instance_id"].(string)
	if event == "" || targetID == "" || env.RequestID == "" {
		return ProgressDelta{}, ErrBadRequest
	}
	switch event {
	case "npc_interact", "world_interact", "escort_checkpoint",
		// EX-P07 side-quest events (same progress path; registry supplies target).
		"inspect_object", "operate_mechanism", "npc_talk",
		"collect_quest_evidence", "reach_trigger", "choose_option":
	default:
		return ProgressDelta{}, ErrUnsupportedEvent
	}
	payloadHash := hashPayload(env.Payload)
	var out ProgressDelta
	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, char characterRow) error {
		if replay, err := e.checkIdempotency(ctx, tx, characterID, env, payloadHash, &out); err != nil {
			return err
		} else if replay {
			return nil
		}
		rows, err := tx.Query(ctx, `
			SELECT quest_id, objective_progress, content_build_id
			FROM public.character_quest_states
			WHERE character_id = $1 AND state = 'ACTIVE'`, characterID)
		if err != nil {
			return ErrInternal
		}
		defer rows.Close()
		type active struct {
			questID string
			progress map[string]map[string]int
			build   string
		}
		var actives []active
		for rows.Next() {
			var qid, build string
			var raw []byte
			if err := rows.Scan(&qid, &raw, &build); err != nil {
				return ErrInternal
			}
			prog := map[string]map[string]int{}
			_ = json.Unmarshal(raw, &prog)
			actives = append(actives, active{qid, prog, build})
		}
		if err := rows.Err(); err != nil {
			return ErrInternal
		}

		var matched *active
		var matchedDef QuestDef
		var obj ObjectiveDef
		var have int
		for i := range actives {
			a := &actives[i]
			def, err := e.cat.QuestForBuild(ctx, e.pool, a.build, a.questID)
			if err != nil {
				continue
			}
			cur, ok := currentObjective(def, a.progress)
			if !ok {
				continue
			}
			if cur.Event != event || cur.Target != targetID {
				continue
			}
			if matched != nil {
				return ErrInvalidTarget
			}
			matched = a
			matchedDef = def
			obj = cur
			have = progressHave(a.progress, cur.ID)
		}
		if matched == nil {
			return ErrInvalidTarget
		}
		if obj.Need > 1 {
			if instanceID == "" || !contains(obj.Instances, instanceID) {
				return ErrInvalidTarget
			}
			tag, err := tx.Exec(ctx, `
				INSERT INTO public.quest_objective_instances (character_id, objective_id, instance_id)
				VALUES ($1, $2, $3)
				ON CONFLICT DO NOTHING`, characterID, obj.ID, instanceID)
			if err != nil {
				return ErrInternal
			}
			if tag.RowsAffected() == 0 {
				return ErrInvalidTarget
			}
			have++
		} else {
			if have >= obj.Need {
				return ErrInvalidTarget
			}
			have++
		}
		if matched.progress == nil {
			matched.progress = map[string]map[string]int{}
		}
		matched.progress[obj.ID] = map[string]int{"have": have}
		progJSON, _ := json.Marshal(matched.progress)
		ready := allObjectivesCompleteDef(matchedDef, matched.progress)
		newState := "ACTIVE"
		if ready {
			newState = "READY_TO_TURN_IN"
		}
		rev := char.Revision + 1
		_, err = tx.Exec(ctx, `
			UPDATE public.character_quest_states
			SET objective_progress = $1::jsonb, state = $2, revision = $3
			WHERE character_id = $4 AND quest_id = $5 AND state = 'ACTIVE'`,
			string(progJSON), newState, rev, characterID, matched.questID)
		if err != nil {
			return ErrInternal
		}
		if ready {
			if err := e.setFlag(ctx, tx, characterID, FlagName(matched.questID, "DONE"), true); err != nil {
				return err
			}
		}
		if err := e.bumpRevision(ctx, tx, characterID, rev); err != nil {
			return err
		}
		out = ProgressDelta{
			QuestID: matched.questID, ObjectiveID: obj.ID,
			Count: have, Need: obj.Need, Revision: rev, Ready: ready,
		}
		return e.storeIdempotency(ctx, tx, characterID, env, payloadHash, out)
	})
	return out, err
}

func (e *Engine) TurnIn(ctx context.Context, accountID string, characterID int64, env Envelope) (RewardGranted, error) {
	questID, _ := env.Payload["quest_id"].(string)
	if questID == "" || env.RequestID == "" {
		return RewardGranted{}, ErrBadRequest
	}
	payloadHash := hashPayload(env.Payload)
	var out RewardGranted
	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, char characterRow) error {
		if replay, err := e.checkIdempotency(ctx, tx, characterID, env, payloadHash, &out); err != nil {
			return err
		} else if replay {
			return nil
		}
		var state, pinnedBuild string
		err := tx.QueryRow(ctx, `
			SELECT state, content_build_id FROM public.character_quest_states
			WHERE character_id = $1 AND quest_id = $2`, characterID, questID).Scan(&state, &pinnedBuild)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return ErrInternal
		}
		if state != "READY_TO_TURN_IN" {
			return ErrStateConflict
		}
		def, err := e.cat.QuestForBuild(ctx, e.pool, pinnedBuild, questID)
		if err != nil {
			return err
		}

		var claimID string
		err = tx.QueryRow(ctx, `
			INSERT INTO public.reward_claims
				(claim_id, character_id, source_kind, source_instance_id, reward_bundle_id, content_build_id)
			VALUES (gen_random_uuid(), $1, 'quest', $2, $3, $4)
			ON CONFLICT (character_id, source_kind, source_instance_id) DO NOTHING
			RETURNING claim_id::text`,
			characterID, questID, "REWARD-"+questID, pinnedBuild).Scan(&claimID)
		if errors.Is(err, pgx.ErrNoRows) {
			// Already claimed — fail closed if quest row was not completed (S-06).
			var existing, claimState string
			if err := tx.QueryRow(ctx, `
				SELECT c.claim_id::text, q.state FROM public.reward_claims c
				JOIN public.character_quest_states q
					ON q.character_id = c.character_id AND q.quest_id = c.source_instance_id
				WHERE c.character_id = $1 AND c.source_kind = 'quest' AND c.source_instance_id = $2`,
				characterID, questID).Scan(&existing, &claimState); err != nil {
				return ErrInternal
			}
			if claimState != "COMPLETED" {
				return ErrStateConflict
			}
			out, err = e.loadRewardResult(ctx, tx, characterID, questID, existing, char)
			return err
		}
		if err != nil {
			return ErrInternal
		}

		xp := char.XP + def.Reward.XP
		gold := char.Gold + def.Reward.Gold
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.character_ledger (character_id, claim_id, kind, delta, balance_after)
			VALUES ($1, $2::uuid, 'xp', $3, $4)`, characterID, claimID, def.Reward.XP, xp); err != nil {
			return ErrInternal
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.character_ledger (character_id, claim_id, kind, delta, balance_after)
			VALUES ($1, $2::uuid, 'gold', $3, $4)`, characterID, claimID, def.Reward.Gold, gold); err != nil {
			return ErrInternal
		}
		rep := map[string]int{}
		for _, r := range def.Reward.Reputation {
			var cur int
			_ = tx.QueryRow(ctx, `
				SELECT value FROM public.character_reputation
				WHERE character_id = $1 AND faction_id = $2`, characterID, r.Faction).Scan(&cur)
			next := cur + r.Delta
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.character_reputation (character_id, faction_id, value)
				VALUES ($1, $2, $3)
				ON CONFLICT (character_id, faction_id) DO UPDATE SET value = EXCLUDED.value`,
				characterID, r.Faction, next); err != nil {
				return ErrInternal
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.character_ledger (character_id, claim_id, kind, ref, delta, balance_after)
				VALUES ($1, $2::uuid, 'reputation', $3, $4, $5)`,
				characterID, claimID, r.Faction, r.Delta, next); err != nil {
				return ErrInternal
			}
			rep[r.Faction] = next
		}
		evidence := append([]string{}, def.Reward.Evidence...)
		for _, item := range evidence {
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.character_quest_items (character_id, item_id, claim_id)
				VALUES ($1, $2, $3::uuid)
				ON CONFLICT DO NOTHING`, characterID, item, claimID); err != nil {
				return ErrInternal
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.character_ledger (character_id, claim_id, kind, ref, delta)
				VALUES ($1, $2::uuid, 'quest_item', $3, 1)`, characterID, claimID, item); err != nil {
				return ErrInternal
			}
		}
		rev := char.Revision + 1
		if _, err := tx.Exec(ctx, `
			UPDATE public.characters SET xp = $1, gold = $2, state_revision = $3 WHERE character_id = $4`,
			xp, gold, rev, characterID); err != nil {
			return ErrInternal
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.character_quest_states
			SET state = 'COMPLETED', completed_at = now(), revision = $1
			WHERE character_id = $2 AND quest_id = $3`, rev, characterID, questID); err != nil {
			return ErrInternal
		}
		if err := e.setFlag(ctx, tx, characterID, FlagName(questID, "REWARD"), true); err != nil {
			return err
		}
		out = RewardGranted{
			QuestID: questID, RewardClaimID: claimID, InventoryRevision: rev,
			XPTotal: xp, GoldTotal: gold, Level: auth.Level(xp),
			Reputation: rep, Evidence: evidence,
		}
		return e.storeIdempotency(ctx, tx, characterID, env, payloadHash, out)
	})
	return out, err
}

func (e *Engine) SnapshotQuests(ctx context.Context, accountID string, characterID int64) (QuestsScope, error) {
	var scope QuestsScope
	scope.Active = []ActiveQuest{}
	scope.Completed = []string{}
	scope.Available = []string{}

	err := e.withCharacter(ctx, accountID, characterID, func(tx pgx.Tx, char characterRow) error {
		rows, err := tx.Query(ctx, `
			SELECT quest_id, state, objective_progress, content_build_id
			FROM public.character_quest_states WHERE character_id = $1`, characterID)
		if err != nil {
			return ErrInternal
		}
		defer rows.Close()
		stored := map[string]string{}
		progress := map[string]map[string]map[string]int{}
		builds := map[string]string{}
		for rows.Next() {
			var qid, state, build string
			var raw []byte
			if err := rows.Scan(&qid, &state, &raw, &build); err != nil {
				return ErrInternal
			}
			stored[qid] = state
			builds[qid] = build
			p := map[string]map[string]int{}
			_ = json.Unmarshal(raw, &p)
			progress[qid] = p
			if state == "COMPLETED" {
				scope.Completed = append(scope.Completed, qid)
				continue
			}
			def, ok := e.cat.Quest(qid)
			aq := ActiveQuest{QuestID: qid, State: state, ContentBuildID: build}
			if ok {
				for _, obj := range def.Objectives {
					have := progressHave(p, obj.ID)
					need := obj.Need
					if need < 1 {
						need = 1
					}
					aq.Objectives = append(aq.Objectives, struct {
						ObjectiveID string `json:"objective_id"`
						Have        int    `json:"have"`
						Need        int    `json:"need"`
					}{obj.ID, have, need})
				}
			}
			scope.Active = append(scope.Active, aq)
		}
		if err := rows.Err(); err != nil {
			return ErrInternal
		}

		level := auth.Level(char.XP)
		for _, def := range e.cat.All() {
			if _, exists := stored[def.ID]; exists {
				continue
			}
			if !e.regions.RegionOpen(def.RegionID, accountID, e.now()) {
				continue
			}
			if level < def.MinLevel {
				continue
			}
			okPrereq := true
			for _, pre := range def.Prereqs {
				if stored[pre] != "COMPLETED" {
					okPrereq = false
					break
				}
			}
			if len(def.Prereqs) == 0 && def.ID == "MQ-01-01" && !char.Flags["TUTORIAL-COMPLETE"] {
				okPrereq = false
			}
			if !okPrereq {
				continue
			}
			if def.Kind == "quest_main" {
				busy := false
				for id, st := range stored {
					if st != "ACTIVE" && st != "READY_TO_TURN_IN" {
						continue
					}
					if q, ok := e.cat.Quest(id); ok && q.Kind == "quest_main" {
						busy = true
						break
					}
				}
				if busy {
					continue
				}
			}
			scope.Available = append(scope.Available, def.ID)
		}
		return nil
	})
	return scope, err
}

type characterRow struct {
	XP, Gold, Revision int64
	Flags              map[string]bool
}

func (e *Engine) withCharacter(ctx context.Context, accountID string, characterID int64, fn func(pgx.Tx, characterRow) error) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return ErrInternal
	}
	defer tx.Rollback(ctx)

	var char characterRow
	var flagsRaw []byte
	err = tx.QueryRow(ctx, `
		SELECT xp, gold, state_revision, flags
		FROM public.characters
		WHERE account_id = $1 AND character_id = $2
		FOR UPDATE`, accountID, characterID).Scan(&char.XP, &char.Gold, &char.Revision, &flagsRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return ErrInternal
	}
	char.Flags = map[string]bool{}
	_ = json.Unmarshal(flagsRaw, &char.Flags)

	if err := fn(tx, char); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrInternal
	}
	return nil
}

func (e *Engine) questState(ctx context.Context, tx pgx.Tx, characterID int64, questID string) (string, error) {
	var state string
	err := tx.QueryRow(ctx, `
		SELECT state FROM public.character_quest_states
		WHERE character_id = $1 AND quest_id = $2`, characterID, questID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", ErrInternal
	}
	return state, nil
}

func (e *Engine) countStates(ctx context.Context, tx pgx.Tx, characterID int64, kind string, states []string) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT quest_id FROM public.character_quest_states
		WHERE character_id = $1 AND state = ANY($2)`, characterID, states)
	if err != nil {
		return 0, ErrInternal
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, ErrInternal
		}
		if q, ok := e.cat.Quest(id); ok && q.Kind == kind {
			n++
		}
	}
	return n, rows.Err()
}

func (e *Engine) setFlag(ctx context.Context, tx pgx.Tx, characterID int64, flag string, value bool) error {
	_, err := tx.Exec(ctx, `
		UPDATE public.characters
		SET flags = jsonb_set(flags, ARRAY[$1]::text[], to_jsonb($2::boolean), true)
		WHERE character_id = $3`, flag, value, characterID)
	if err != nil {
		return ErrInternal
	}
	return nil
}

func (e *Engine) bumpRevision(ctx context.Context, tx pgx.Tx, characterID int64, rev int64) error {
	_, err := tx.Exec(ctx, `UPDATE public.characters SET state_revision = $1 WHERE character_id = $2`, rev, characterID)
	if err != nil {
		return ErrInternal
	}
	return nil
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
		WHERE character_id = $1 AND request_id = $2`, characterID, requestUUID).Scan(&storedHash, &result)
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
		VALUES ($1, $2, $3, $4, $5::jsonb)`, characterID, requestUUID, env.Type, payloadHash, string(raw))
	if err != nil {
		return ErrInternal
	}
	return nil
}

func (e *Engine) loadRewardResult(ctx context.Context, tx pgx.Tx, characterID int64, questID, claimID string, char characterRow) (RewardGranted, error) {
	rep := map[string]int{}
	rows, err := tx.Query(ctx, `SELECT faction_id, value FROM public.character_reputation WHERE character_id = $1`, characterID)
	if err != nil {
		return RewardGranted{}, ErrInternal
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		var v int
		if err := rows.Scan(&f, &v); err != nil {
			return RewardGranted{}, ErrInternal
		}
		rep[f] = v
	}
	var evidence []string
	erows, err := tx.Query(ctx, `SELECT item_id FROM public.character_quest_items WHERE character_id = $1 AND claim_id = $2::uuid`, characterID, claimID)
	if err != nil {
		return RewardGranted{}, ErrInternal
	}
	defer erows.Close()
	for erows.Next() {
		var id string
		if err := erows.Scan(&id); err != nil {
			return RewardGranted{}, ErrInternal
		}
		evidence = append(evidence, id)
	}
	return RewardGranted{
		QuestID: questID, RewardClaimID: claimID, InventoryRevision: char.Revision,
		XPTotal: char.XP, GoldTotal: char.Gold, Level: auth.Level(char.XP),
		Reputation: rep, Evidence: evidence,
	}, nil
}

func currentObjective(def QuestDef, progress map[string]map[string]int) (ObjectiveDef, bool) {
	for _, obj := range def.Objectives {
		need := obj.Need
		if need < 1 {
			need = 1
		}
		if progressHave(progress, obj.ID) < need {
			return obj, true
		}
	}
	return ObjectiveDef{}, false
}

func progressHave(progress map[string]map[string]int, objectiveID string) int {
	if progress == nil {
		return 0
	}
	if m, ok := progress[objectiveID]; ok {
		return m["have"]
	}
	return 0
}

func allObjectivesCompleteDef(def QuestDef, progress map[string]map[string]int) bool {
	for _, obj := range def.Objectives {
		need := obj.Need
		if need < 1 {
			need = 1
		}
		if progressHave(progress, obj.ID) < need {
			return false
		}
	}
	return true
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
	// Validate hex+hyphen shape without new deps.
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

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
