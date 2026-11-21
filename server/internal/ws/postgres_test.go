package ws

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
)

func TestPostgresWebSocketTickets(t *testing.T) {
	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL to enable WebSocket database integration")
	}
	for _, name := range []string{"NEWMMO_TEST_DATABASE_URL", "NEWMMO_TEST_MIGRATE_DATABASE_URL", "NEWMMO_TEST_APP_DATABASE_URL"} {
		if os.Getenv(name) == "" {
			t.Fatalf("required WebSocket integration setting is missing: %s", name)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	publisher := checkRole(t, ctx, "NEWMMO_TEST_DATABASE_URL", "newmmo_content_publisher")
	migrator := checkRole(t, ctx, "NEWMMO_TEST_MIGRATE_DATABASE_URL", "newmmo_migrator")
	app := checkRole(t, ctx, "NEWMMO_TEST_APP_DATABASE_URL", "newmmo_app")
	_ = publisher.Close(ctx)

	account := func() string {
		t.Helper()
		var id string
		var random [8]byte
		_, _ = rand.Read(random[:])
		if err := migrator.QueryRow(ctx, `INSERT INTO public.accounts (login,password_hash) VALUES ($1,'fixture')
			RETURNING account_id::text`, "ws_"+hex.EncodeToString(random[:])).Scan(&id); err != nil {
			t.Fatal("could not create test account")
		}
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			for _, query := range []string{
				"DELETE FROM public.ws_tickets WHERE account_id = $1",
				"DELETE FROM public.characters WHERE account_id = $1",
				"DELETE FROM public.accounts WHERE account_id = $1",
			} {
				if _, err := migrator.Exec(cleanupCtx, query, id); err != nil {
					t.Error("could not clean up WebSocket account fixture")
				}
			}
		})
		return id
	}

	owner, foreign := account(), account()
	store, err := auth.Open(ctx, auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL")})
	if err != nil {
		t.Fatal("could not open app store")
	}
	t.Cleanup(store.Close)
	var characterName [12]byte
	_, _ = rand.Read(characterName[:])
	for i := range characterName {
		characterName[i] = 'a' + characterName[i]%26
	}
	char, err := store.CreateCharacter(ctx, owner, string(characterName[:]), "CLS-01")
	if err != nil {
		t.Fatalf("character fixture: %v", err)
	}
	charID, err := strconv.ParseInt(char, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.IssueTicket(ctx, foreign, charID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("foreign account issued a ticket")
	}
	if _, err := store.LoadWSState(ctx, foreign, charID); err == nil {
		t.Fatal("foreign account loaded a snapshot")
	}
	state, err := store.LoadWSState(ctx, owner, charID)
	if err != nil || state.ID != char || state.Name != string(characterName[:]) || state.ClassID != "CLS-01" {
		t.Fatal("database snapshot did not match authenticated character")
	}
	_, err = app.Exec(ctx, `INSERT INTO public.ws_tickets (ticket_hash,account_id,character_id,expires_at)
		VALUES (repeat('a',64),$1,$2,now()+interval '1 minute')`, foreign, charID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatal("database allowed a ticket with mismatched ownership")
	}

	_, url, log := testServer(t, DefaultConfig(), store, nil)
	issue := func() string {
		t.Helper()
		ticket, err := store.IssueTicket(ctx, owner, charID)
		if err != nil {
			t.Fatalf("issue fixture ticket: %v", err)
		}
		return ticket.Value
	}

	shared, expired := issue(), issue()
	hash := sha256.Sum256([]byte(expired))
	if _, err := migrator.Exec(ctx, "UPDATE public.ws_tickets SET expires_at = now()-interval '1 second' WHERE ticket_hash = $1", hex.EncodeToString(hash[:])); err != nil {
		t.Fatal("could not expire fixture ticket")
	}
	reuse := dial(t, url, nil)
	login(t, reuse, expired, "100.0.0", "")
	expectClose(t, reuse, 4001)

	first, second := dial(t, url, nil), dial(t, url, nil)
	var sends [2]chan error
	start := make(chan struct{})
	for i, c := range []*websocket.Conn{first, second} {
		sends[i] = make(chan error, 1)
		go func(result chan<- error) {
			<-start
			result <- c.Write(ctx, websocket.MessageText, []byte(`{"protocol_version":1,"type":"session.auth","request_id":"`+requestID+
				`","payload":{"ticket":"`+shared+`","client_version":"100.0.0","protocol_versions":[1],"character_id":"999"}}`))
		}(sends[i])
	}
	close(start)
	for _, result := range sends {
		if err := <-result; err != nil {
			t.Fatal("concurrent auth write failed")
		}
	}
	accepted := 0
	for _, c := range []*websocket.Conn{first, second} {
		msg, err := read(t, c)
		if websocket.CloseStatus(err) == 4001 {
			continue
		}
		if err != nil {
			t.Fatalf("unexpected ticket result: %v", err)
		}
		switch msg.Type {
		case "session.ready":
			if string(msg.Payload["character_id"]) != strconv.Quote(char) {
				t.Fatal("ready used the injected character")
			}
			expect(t, c, "content.version")
			expect(t, c, "state.snapshot")
			if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
				t.Fatal(err)
			}
		case "content.version":
			// A standalone DB run can start before any content is published.
			// The winning ticket is still consumed and receives incompatibility.
			if string(msg.Payload["compatibility"]) != `"incompatible"` {
				t.Fatal("missing published content was not handled")
			}
			expectClose(t, c, 4004)
		default:
			t.Fatalf("unexpected ticket result: %s", msg.Type)
		}
		accepted++
	}
	if accepted != 1 {
		t.Fatalf("%d sockets consumed the same ticket", accepted)
	}
	replay := dial(t, url, nil)
	login(t, replay, shared, "100.0.0", "")
	expectClose(t, replay, 4001)
	if content := log.String(); strings.Contains(content, shared) || strings.Contains(content, expired) {
		t.Fatal("credential leaked into log")
	}
}

func checkRole(t *testing.T, ctx context.Context, setting, want string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, os.Getenv(setting))
	if err != nil {
		t.Fatalf("could not connect using %s", setting)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	var role string
	if err := conn.QueryRow(ctx, "SELECT current_user").Scan(&role); err != nil || role != want {
		t.Fatalf("%s must connect as %s", setting, want)
	}
	return conn
}
