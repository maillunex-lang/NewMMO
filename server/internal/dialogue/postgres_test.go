package dialogue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func TestDialogueOpenChoicePostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for dialogue integration")
	}
	for _, name := range []string{"NEWMMO_TEST_DATABASE_URL", "NEWMMO_TEST_MIGRATE_DATABASE_URL", "NEWMMO_TEST_APP_DATABASE_URL"} {
		if os.Getenv(name) == "" {
			t.Fatalf("required dialogue integration setting is missing: %s", name)
		}
	}

	migrator, err := pgxpool.New(ctx, os.Getenv("NEWMMO_TEST_MIGRATE_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(migrator.Close)

	pool, err := pgxpool.New(ctx, os.Getenv("NEWMMO_TEST_APP_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	cat := NewCatalog()
	flowsPath := ResolveFlowsPath("")
	if err := cat.LoadFlows(flowsPath); err != nil {
		t.Fatal(err)
	}
	buildID, err := cat.ActiveBuildID(ctx, pool)
	if err != nil || buildID == "" {
		t.Fatal("need published content build:", err)
	}
	if err := cat.LoadLinesFromDB(ctx, pool, buildID); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(pool, cat)

	store, err := auth.Open(ctx, auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	var accountID string
	if err := migrator.QueryRow(ctx, `INSERT INTO public.accounts (login, password_hash) VALUES ($1, 'fixture')
		RETURNING account_id::text`, "d_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
		t.Fatal("account fixture:", err)
	}

	var nameBytes [10]byte
	_, _ = rand.Read(nameBytes[:])
	for i := range nameBytes {
		nameBytes[i] = 'a' + nameBytes[i]%26
	}
	charIDStr, err := store.CreateCharacter(ctx, accountID, string(nameBytes[:]), "CLS-01")
	if err != nil {
		t.Fatal(err)
	}
	characterID, err := strconv.ParseInt(charIDStr, 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	ids := []string{
		"550e8400-e29b-41d4-a716-446655440101",
		"550e8400-e29b-41d4-a716-446655440102",
		"550e8400-e29b-41d4-a716-446655440103",
		"550e8400-e29b-41d4-a716-446655440104",
		"550e8400-e29b-41d4-a716-446655440105",
	}

	open, err := engine.Open(ctx, accountID, characterID, Envelope{
		Type: "dialogue.open.request", RequestID: ids[0],
		Payload: map[string]any{"npc_id": "NPC-802", "dialogue_id": "MQ-01-01"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if open.NodeID != "DIA.MQ-01-01.START.01" || !optionAllowed(open.Options, "ACCEPT") {
		t.Fatalf("open: %#v", open)
	}

	_, err = engine.Open(ctx, accountID, characterID, Envelope{
		Type: "dialogue.open.request", RequestID: ids[1],
		Payload: map[string]any{"npc_id": "NPC-999", "dialogue_id": "MQ-01-01"},
	})
	if err != ErrPrerequisiteNotMet {
		t.Fatalf("wrong npc open: %v", err)
	}

	_, err = engine.Choice(ctx, accountID, characterID, Envelope{
		Type: "dialogue.choice.request", RequestID: ids[2],
		Payload: map[string]any{
			"dialogue_id": "MQ-01-01", "node_id": open.NodeID,
			"option_id": "TURN_IN", "npc_id": "NPC-802",
		},
	})
	if err != ErrPrerequisiteNotMet {
		t.Fatalf("illegal TURN_IN: %v", err)
	}

	choice, err := engine.Choice(ctx, accountID, characterID, Envelope{
		Type: "dialogue.choice.request", RequestID: ids[3],
		Payload: map[string]any{
			"dialogue_id": "MQ-01-01", "node_id": open.NodeID,
			"option_id": "ACCEPT", "npc_id": "NPC-802",
		},
	})
	if err != nil || choice.Revision <= open.Revision {
		t.Fatalf("accept choice: %#v err=%v", choice, err)
	}

	again, err := engine.Choice(ctx, accountID, characterID, Envelope{
		Type: "dialogue.choice.request", RequestID: ids[3],
		Payload: map[string]any{
			"dialogue_id": "MQ-01-01", "node_id": open.NodeID,
			"option_id": "ACCEPT", "npc_id": "NPC-802",
		},
	})
	if err != nil || again.Revision != choice.Revision {
		t.Fatalf("idempotent choice: %#v err=%v", again, err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM public.character_dialogue_choices
		WHERE character_id=$1 AND dialogue_id='MQ-01-01' AND node_id=$2 AND option_id='ACCEPT'`,
		characterID, open.NodeID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("choices=%d err=%v", count, err)
	}

	// Concurrent conflicting option on same node → one conflict (already committed ACCEPT).
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		rid := ids[4]
		if i == 1 {
			rid = "550e8400-e29b-41d4-a716-446655440106"
		}
		go func(requestID, option string) {
			defer wg.Done()
			<-start
			_, e := engine.Choice(ctx, accountID, characterID, Envelope{
				Type: "dialogue.choice.request", RequestID: requestID,
				Payload: map[string]any{
					"dialogue_id": "MQ-01-01", "node_id": open.NodeID,
					"option_id": option, "npc_id": "NPC-802",
				},
			})
			results <- e
		}(rid, "LATER")
	}
	close(start)
	wg.Wait()
	close(results)
	var conflict int
	for e := range results {
		if e == ErrStateConflict || e == ErrPrerequisiteNotMet {
			conflict++
		} else if e != nil {
			t.Fatalf("unexpected: %v", e)
		}
	}
	if conflict < 1 {
		t.Fatalf("expected conflict on re-choice, got conflict=%d", conflict)
	}

	_ = filepath.Base(flowsPath)
}
