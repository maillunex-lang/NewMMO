package quest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func TestSideQuestsREG01Postgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for quest integration")
	}
	for _, name := range []string{"NEWMMO_TEST_DATABASE_URL", "NEWMMO_TEST_MIGRATE_DATABASE_URL", "NEWMMO_TEST_APP_DATABASE_URL"} {
		if os.Getenv(name) == "" {
			t.Fatalf("required setting missing: %s", name)
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
	if err := cat.LoadInteractables(ResolveInteractablesPath("")); err != nil {
		t.Fatal(err)
	}
	buildID, err := cat.ActiveBuildID(ctx, pool)
	if err != nil || buildID == "" {
		t.Fatal(err)
	}
	if err := cat.LoadFromDB(ctx, pool, buildID); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(pool, cat, RegionConfig{})
	store, err := auth.Open(ctx, auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	cases := []struct {
		questID, prereq string
		minXP, minGold  int64
	}{
		{"SQ-01-002", "MQ-01-05", 135, 27},
		{"SQ-01-003", "MQ-01-07", 160, 32},
		{"SQ-01-004", "MQ-01-10", 195, 40},
		{"SQ-01-005", "MQ-01-04", 95, 19},
	}

	for _, tc := range cases {
		t.Run(tc.questID, func(t *testing.T) {
			var rnd [8]byte
			_, _ = rand.Read(rnd[:])
			var accountID string
			if err := migrator.QueryRow(ctx, `INSERT INTO public.accounts (login, password_hash) VALUES ($1, 'fixture')
				RETURNING account_id::text`, "sqx_"+hex.EncodeToString(rnd[:])).Scan(&accountID); err != nil {
				t.Fatal(err)
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
			if _, err := migrator.Exec(ctx, `
				INSERT INTO public.character_quest_states
					(character_id, quest_id, state, content_build_id, objective_progress, accepted_at, completed_at)
				VALUES ($1, $2, 'COMPLETED', $3, '{}'::jsonb, now(), now())`,
				characterID, tc.prereq, buildID); err != nil {
				t.Fatal("seed prereq:", err)
			}

			def, ok := cat.Quest(tc.questID)
			if !ok {
				t.Fatal("quest missing from catalog")
			}
			mkID := func(n int) string {
				return "550e8400-e29b-41d4-a716-" + hex.EncodeToString([]byte{
					0x44, 0x66, 0x55, 0x44, byte(n >> 8), byte(n),
				})
			}

			accept, err := engine.Accept(ctx, accountID, characterID, Envelope{
				Type: "quest.accept.request", RequestID: mkID(1),
				Payload: map[string]any{"quest_id": tc.questID, "content_build_id": buildID},
			})
			if err != nil || accept.State != "ACTIVE" {
				t.Fatalf("accept: %#v err=%v", accept, err)
			}

			step := 2
			var last ProgressDelta
			for _, obj := range def.Objectives {
				if obj.Event == "" || obj.Target == "" {
					t.Fatalf("objective %s missing registry binding", obj.ID)
				}
				need := obj.Need
				if need < 1 {
					need = 1
				}
				for i := 0; i < need; i++ {
					payload := map[string]any{"event": obj.Event, "target_id": obj.Target}
					if need > 1 {
						if i >= len(obj.Instances) {
							t.Fatalf("%s need=%d instances=%d", obj.ID, need, len(obj.Instances))
						}
						payload["instance_id"] = obj.Instances[i]
					}
					last, err = engine.Interact(ctx, accountID, characterID, Envelope{
						Type: "world.interact.request", RequestID: mkID(step), Payload: payload,
					})
					step++
					if err != nil {
						t.Fatalf("interact %s#%d: %v", obj.ID, i, err)
					}
				}
			}
			if !last.Ready {
				t.Fatalf("not ready: %#v", last)
			}
			reward, err := engine.TurnIn(ctx, accountID, characterID, Envelope{
				Type: "quest.turnin.request", RequestID: mkID(step),
				Payload: map[string]any{"quest_id": tc.questID},
			})
			if err != nil {
				t.Fatal(err)
			}
			if reward.XPTotal < tc.minXP || reward.GoldTotal < tc.minGold {
				t.Fatalf("reward %#v want xp>=%d gold>=%d", reward, tc.minXP, tc.minGold)
			}
		})
	}
}
