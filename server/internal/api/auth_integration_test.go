package api

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/httpservice"
)

func TestAuthRESTPostgres(t *testing.T) {
	ctx := t.Context()
	publisher, migrator, reader := gameTestPools(t)
	store, err := auth.Open(ctx, auth.Config{
		DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL"),
		Limits:      auth.Limits{LoginPerPair: 5, LoginPerIP: 20, RegistrationsPerIP: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	const wsURL = "ws://127.0.0.1:18086/v1/ws"
	handler := httpservice.NewHandler(ctx, logger, httpservice.Application{API: New(store, wsURL), Ready: store.Ready})

	peer := randomPeer()
	request := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return callFrom(handler, method, path, bearer, body, peer)
	}
	suffix := randomSuffix()
	loginName := "acct_" + strings.ToLower(suffix)
	otherLogin := "other_" + strings.ToLower(suffix)
	const password = "Café-phrase-12345"
	const decomposed = "Cafe\u0301-phrase-12345"
	const wrongPassword = "different-phrase-12345"

	register := func(name, password string) string {
		response := request("POST", "/v1/accounts", "", map[string]string{"login": name, "password": password})
		return getString(t, response, 201, "account_id")
	}
	accountID := register(loginName, decomposed)
	otherID := register(otherLogin, password)
	if accountID == otherID {
		t.Fatal("two accounts have the same ID")
	}
	expectError(t, request("POST", "/v1/accounts", "", map[string]string{"login": loginName, "password": password}), 409, "LOGIN_TAKEN")

	var knownTime, unknownTime time.Duration
	for range 3 {
		start := time.Now()
		wrong := request("POST", "/v1/auth/login", "", map[string]string{"login": loginName, "password": wrongPassword})
		knownTime += time.Since(start)
		start = time.Now()
		unknown := request("POST", "/v1/auth/login", "", map[string]string{"login": "unknown_" + strings.ToLower(suffix), "password": wrongPassword})
		unknownTime += time.Since(start)
		expectError(t, wrong, 401, "INVALID_CREDENTIALS")
		if unknown.Code != wrong.Code || unknown.Body.String() != wrong.Body.String() {
			t.Fatal("unknown login and wrong password expose different responses")
		}
	}
	if unknownTime < knownTime/4 || knownTime < unknownTime/4 {
		t.Fatalf("login failure timings diverge: known %v, unknown %v", knownTime, unknownTime)
	}
	t.Logf("failed login KDF timing, known %v and unknown %v over 3 trials each", knownTime, unknownTime)

	login := func(loginName, password string) auth.Session {
		r := request("POST", "/v1/auth/login", "", map[string]string{"login": loginName, "password": password})
		var result auth.Session
		decode(t, r, 200, &result)
		if !auth.ValidSecret(result.AccessToken) || time.Until(result.ExpiresAt) < 12*time.Hour-15*time.Second ||
			time.Until(result.ExpiresAt) > 12*time.Hour+time.Second {
			t.Fatal("invalid access token or TTL")
		}
		return result
	}
	session := login(loginName, password)
	other := login(otherLogin, decomposed)
	if session.AccountID != accountID || other.AccountID != otherID {
		t.Fatal("login returned the wrong account")
	}
	var storedTokenHash, passwordHash string
	if err := reader.QueryRow(ctx, `SELECT token_hash FROM public.auth_tokens WHERE account_id=$1`, accountID).Scan(&storedTokenHash); err != nil {
		t.Fatal(err)
	}
	if err := reader.QueryRow(ctx, `SELECT password_hash FROM public.accounts WHERE account_id=$1`, accountID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if storedTokenHash != secretHash(session.AccessToken) || !strings.HasPrefix(passwordHash, "pbkdf2-sha256$600000$") ||
		strings.Contains(passwordHash, password) {
		t.Fatal("database did not store only the expected hashes")
	}

	legacyName := "legacy_" + strings.ToLower(suffix)
	legacySalt := []byte("test-only-salt!!")
	legacyKey, err := pbkdf2.Key(sha256.New, password, legacySalt, 1234, 32)
	if err != nil {
		t.Fatal(err)
	}
	legacyHash := "pbkdf2-sha256$1234$" + base64.RawURLEncoding.EncodeToString(legacySalt) + "$" + base64.RawURLEncoding.EncodeToString(legacyKey)
	if _, err := migrator.Exec(ctx, `INSERT INTO public.accounts(login,password_hash) VALUES($1,$2)`, legacyName, legacyHash); err != nil {
		t.Fatal(err)
	}
	legacy := login(legacyName, decomposed)
	var updatedHash string
	if err := reader.QueryRow(ctx, `SELECT password_hash FROM public.accounts WHERE login=$1`, legacyName).Scan(&updatedHash); err != nil {
		t.Fatal(err)
	}
	if updatedHash == legacyHash || !strings.HasPrefix(updatedHash, "pbkdf2-sha256$600000$") {
		t.Fatal("legacy password hash was not upgraded on login")
	}
	if _, err := migrator.Exec(ctx, `UPDATE public.auth_tokens SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, secretHash(legacy.AccessToken)); err != nil {
		t.Fatal(err)
	}
	expectError(t, request("GET", "/v1/characters", legacy.AccessToken, nil), 401, "UNAUTHENTICATED")

	expectError(t, request("GET", "/v1/characters", "", nil), 401, "UNAUTHENTICATED")
	expectError(t, request("POST", "/v1/characters", "", map[string]string{"name": "Руна", "class_id": "CLS-01"}), 401, "UNAUTHENTICATED")
	empty := request("GET", "/v1/characters", session.AccessToken, nil)
	if empty.Code != 200 || empty.Body.String() != "{\"characters\":[]}\n" {
		t.Fatalf("empty character list returned %d", empty.Code)
	}
	activeSnapshot := func() (string, time.Time) {
		var active string
		var switched time.Time
		if err := reader.QueryRow(ctx, `SELECT coalesce(build_id,''), switched_at FROM public.content_active WHERE id=1`).
			Scan(&active, &switched); err != nil {
			t.Fatal(err)
		}
		return active, switched
	}
	beforeBuild, beforeSwitch := activeSnapshot()
	name := "Руна" + suffix
	character := getString(t, request("POST", "/v1/characters", session.AccessToken, map[string]string{
		"name": name, "class_id": "CLS-01",
	}), 201, "character_id")
	id, err := strconv.ParseInt(character, 10, 64)
	if err != nil || id <= 0 {
		t.Fatal("character ID is not a positive decimal string")
	}
	expectError(t, request("POST", "/v1/characters", other.AccessToken, map[string]string{
		"name": strings.ToUpper(name), "class_id": "CLS-02",
	}), 409, "NAME_TAKEN")
	var initialized bool
	var createdBuild string
	if err := reader.QueryRow(ctx, `SELECT xp=0 AND gold=0 AND state_revision=0 AND flags @> '{"TUTORIAL-COMPLETE":true}',
		coalesce(content_build_id,'') FROM public.characters WHERE character_id=$1`, id).Scan(&initialized, &createdBuild); err != nil || !initialized {
		t.Fatalf("character state was not initialized: %v", err)
	}
	afterBuild, afterSwitch := activeSnapshot()
	if beforeBuild == afterBuild && beforeSwitch.Equal(afterSwitch) && createdBuild != beforeBuild {
		t.Fatal("character creation did not record the active build snapshot")
	}
	var listed struct {
		Characters []auth.Character `json:"characters"`
	}
	decode(t, request("GET", "/v1/characters", session.AccessToken, nil), 200, &listed)
	if len(listed.Characters) != 1 || listed.Characters[0] != (auth.Character{ID: character, Name: name, ClassID: "CLS-01", Level: 1}) {
		t.Fatal("character list did not return the authoritative initial state")
	}

	body := map[string]string{"character_id": character}
	expectError(t, request("POST", "/v1/session/ticket", "", body), 401, "UNAUTHENTICATED")
	expectError(t, request("POST", "/v1/session/ticket", other.AccessToken, body), 403, "FORBIDDEN")
	expectError(t, request("POST", "/v1/session/ticket", session.AccessToken, map[string]string{
		"character_id": "9223372036854775807"}), 404, "NOT_FOUND")
	issue := func() string {
		var result struct {
			auth.Ticket
			WSURL string `json:"ws_url"`
		}
		decode(t, request("POST", "/v1/session/ticket", session.AccessToken, body), 200, &result)
		if !auth.ValidSecret(result.Value) || result.ExpiresInMS != 30000 || result.WSURL != wsURL {
			t.Fatal("invalid WS ticket contract")
		}
		return result.Value
	}
	ticket := issue()
	var storedTicketHash string
	var ticketTTL time.Duration
	var ttlSeconds float64
	if err := reader.QueryRow(ctx, `SELECT ticket_hash, extract(epoch FROM expires_at-created_at)::float8 FROM public.ws_tickets WHERE character_id=$1`,
		id).Scan(&storedTicketHash, &ttlSeconds); err != nil {
		t.Fatal(err)
	}
	ticketTTL = time.Duration(ttlSeconds * float64(time.Second))
	if storedTicketHash != secretHash(ticket) || ticketTTL != 30*time.Second {
		t.Fatal("ticket hash or TTL did not match")
	}
	if gotAccount, gotID, err := store.ConsumeTicket(ctx, ticket); err != nil || gotAccount != accountID || gotID != id {
		t.Fatal("ticket did not resolve the correct account and character")
	}
	if _, _, err := store.ConsumeTicket(ctx, ticket); !errors.Is(err, auth.ErrInvalidTicket) {
		t.Fatal("ticket was usable twice")
	}
	expired := issue()
	if _, err := migrator.Exec(ctx, `UPDATE public.ws_tickets SET expires_at=now()-interval '1 second', created_at=now()-interval '2 hours' WHERE ticket_hash=$1`, secretHash(expired)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ConsumeTicket(ctx, expired); !errors.Is(err, auth.ErrInvalidTicket) {
		t.Fatal("expired ticket was accepted")
	}
	var oldAttempt int64
	if err := migrator.QueryRow(ctx, `INSERT INTO public.login_attempts(login,ip,ok,at)
		VALUES($1,'192.0.2.8',false,now()-interval '25 hours') RETURNING attempt_id`, loginName).Scan(&oldAttempt); err != nil {
		t.Fatal(err)
	}
	store.Prune(ctx)
	var oldAttemptExists bool
	if err := reader.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.login_attempts WHERE attempt_id=$1)`, oldAttempt).
		Scan(&oldAttemptExists); err != nil || oldAttemptExists {
		t.Fatalf("login attempt older than 24 hours was not pruned: %v", err)
	}
	var expiredExists bool
	if err := reader.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.ws_tickets WHERE ticket_hash=$1)`, secretHash(expired)).Scan(&expiredExists); err != nil || expiredExists {
		t.Fatalf("ticket older than one hour was not pruned: %v", err)
	}

	raced := issue()
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			_, _, err := store.ConsumeTicket(ctx, raced)
			results <- err
		}()
	}
	close(start)
	var successes int
	for range 2 {
		if err := <-results; err == nil {
			successes++
		} else if !errors.Is(err, auth.ErrInvalidTicket) {
			t.Fatal("unexpected concurrent ticket error")
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent ticket successes = %d, want 1", successes)
	}

	checkCharacterLimit(t, handler, session.AccessToken, suffix)

	expectError(t, request("POST", "/v1/auth/logout", "", map[string]string{}), 401, "UNAUTHENTICATED")
	if response := request("POST", "/v1/auth/logout", session.AccessToken, map[string]string{}); response.Code != 204 || response.Body.Len() != 0 {
		t.Fatalf("logout returned %d", response.Code)
	}
	expectError(t, request("GET", "/v1/characters", session.AccessToken, nil), 401, "UNAUTHENTICATED")
	expectError(t, request("POST", "/v1/auth/logout", session.AccessToken, map[string]string{}), 401, "UNAUTHENTICATED")
	if _, err := reader.Exec(ctx, `CREATE TABLE public.auth_ddl_must_fail (id integer)`); !sqlState(err, "42501") {
		t.Fatal("newmmo_app unexpectedly has DDL privileges")
	}
	if _, err := auth.Open(ctx, auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_DATABASE_URL")}); err == nil {
		t.Fatal("server accepted the publisher database role")
	}
	if err := publisher.Ping(ctx); err != nil {
		t.Fatal("publisher role check connection became unavailable")
	}

	for _, secret := range []string{password, decomposed, wrongPassword, session.AccessToken, other.AccessToken, legacy.AccessToken, ticket, expired, raced, passwordHash, updatedHash} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("HTTP logs contain credential material")
		}
	}
}

func checkCharacterLimit(t *testing.T, handler http.Handler, bearer, suffix string) {
	t.Helper()
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 5)
	for i := range 5 {
		go func() {
			<-start
			responses <- call(handler, "POST", "/v1/characters", bearer, map[string]string{
				"name": "Герой" + suffix + string(rune('A'+i)), "class_id": "CLS-01",
			})
		}()
	}
	close(start)
	var created int
	for range 5 {
		response := <-responses
		if response.Code == 201 {
			created++
		} else {
			expectError(t, response, 409, "LIMIT_REACHED")
		}
	}
	if created != 2 {
		t.Fatalf("concurrent creation made %d additional characters, want 2", created)
	}
}

func TestAuthRateLimitsPostgres(t *testing.T) {
	_, _, _ = gameTestPools(t)
	store, err := auth.Open(t.Context(), auth.Config{DatabaseURL: os.Getenv("NEWMMO_TEST_APP_DATABASE_URL"),
		Limits: auth.Limits{LoginPerPair: 2, LoginPerIP: 3, RegistrationsPerIP: 2}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, "ws://127.0.0.1:18086/v1/ws")
	suffix := strings.ToLower(randomSuffix())
	rateIP := randomPeer()
	request := func(path, login string) *httptest.ResponseRecorder {
		return callFrom(handler, "POST", path, "", map[string]string{"login": login, "password": "fixture-password-long"}, rateIP)
	}
	for _, login := range []string{"pair_" + suffix, "pair_" + suffix} {
		expectError(t, request("/v1/auth/login", login), 401, "INVALID_CREDENTIALS")
	}
	expectRateLimit(t, request("/v1/auth/login", "pair_"+suffix), 15*time.Minute)
	expectError(t, request("/v1/auth/login", "different_"+suffix), 401, "INVALID_CREDENTIALS")
	expectRateLimit(t, request("/v1/auth/login", "another_"+suffix), 15*time.Minute)
	for _, login := range []string{"newa_" + suffix, "newb_" + suffix} {
		_ = getString(t, request("/v1/accounts", login), 201, "account_id")
	}
	expectRateLimit(t, request("/v1/accounts", "newc_"+suffix), time.Hour)
}

func gameTestPools(t *testing.T) (publisher, migrator, app *pgxpool.Pool) {
	t.Helper()
	strict := os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") == "1"
	roles := []struct{ env, name string }{
		{"NEWMMO_TEST_DATABASE_URL", "newmmo_content_publisher"},
		{"NEWMMO_TEST_MIGRATE_DATABASE_URL", "newmmo_migrator"},
		{"NEWMMO_TEST_APP_DATABASE_URL", "newmmo_app"},
	}
	for _, role := range roles {
		if os.Getenv(role.env) == "" {
			if strict {
				t.Fatalf("%s is required in strict mode", role.env)
			}
			t.Skipf("%s unavailable for PostgreSQL auth tests", role.env)
		}
	}
	var pools []*pgxpool.Pool
	for _, role := range roles {
		pool, err := pgxpool.New(t.Context(), os.Getenv(role.env))
		if err != nil {
			t.Fatalf("cannot configure test role %s", role.name)
		}
		t.Cleanup(pool.Close)
		var user string
		if err := pool.QueryRow(t.Context(), `SELECT current_user`).Scan(&user); err != nil || user != role.name {
			t.Fatalf("expected test role %s, got %q", role.name, user)
		}
		pools = append(pools, pool)
	}
	return pools[0], pools[1], pools[2]
}

func randomPeer() string {
	digest := secretHash(rand.Text())
	return "[2001:db8:" + digest[:4] + ":" + digest[4:8] + ":" + digest[8:12] + "::1]:41921"
}

func randomSuffix() string {
	word := rand.Text()[:6]
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r
		}
		return 'A' + (r - '2')
	}, word)
}

func secretHash(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(digest[:])
}

func sqlState(err error, expected string) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == expected
}

func call(handler http.Handler, method, path, bearer string, body any) *httptest.ResponseRecorder {
	return callFrom(handler, method, path, bearer, body, "192.0.2.18:41921")
}

func callFrom(handler http.Handler, method, path, bearer string, body any, peer string) *httptest.ResponseRecorder {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request.RemoteAddr = peer
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decode(t *testing.T, r *httptest.ResponseRecorder, status int, out any) {
	t.Helper()
	if r.Code != status {
		t.Fatalf("HTTP status %d, want %d", r.Code, status)
	}
	if err := json.Unmarshal(r.Body.Bytes(), out); err != nil {
		t.Fatal("invalid JSON response")
	}
}

func getString(t *testing.T, r *httptest.ResponseRecorder, status int, field string) string {
	t.Helper()
	var payload map[string]string
	decode(t, r, status, &payload)
	if payload[field] == "" {
		t.Fatalf("missing field %q", field)
	}
	return payload[field]
}

func expectError(t *testing.T, r *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var reply struct {
		Error map[string]string `json:"error"`
	}
	decode(t, r, status, &reply)
	if len(reply.Error) != 2 || reply.Error["code"] != code || reply.Error["message_key"] != "error."+strings.ToLower(code) {
		t.Fatalf("wrong error envelope for %s", code)
	}
}

func expectRateLimit(t *testing.T, r *httptest.ResponseRecorder, window time.Duration) {
	t.Helper()
	var payload struct {
		Error struct {
			Code         string `json:"code"`
			MessageKey   string `json:"message_key"`
			RetryAfterMS int64  `json:"retry_after_ms"`
		} `json:"error"`
	}
	decode(t, r, 429, &payload)
	if payload.Error.Code != "RATE_LIMITED" || payload.Error.MessageKey != "error.rate_limited" ||
		payload.Error.RetryAfterMS <= 0 || payload.Error.RetryAfterMS > window.Milliseconds() || r.Header().Get("Retry-After") == "" {
		t.Fatal("invalid rate-limit envelope or retry delay")
	}
}
