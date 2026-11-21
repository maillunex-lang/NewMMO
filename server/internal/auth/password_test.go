package auth

import (
	"strings"
	"testing"
)

func TestPasswordFormatAndNormalization(t *testing.T) {
	composed, ok := NormalizePassword("Café-пример-123")
	decomposed, otherOK := NormalizePassword("Cafe\u0301-пример-123")
	if !ok || !otherOK || composed != decomposed {
		t.Fatal("canonically equivalent passwords must normalize alike")
	}

	hash, err := hashPassword(composed, passwordIterations)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "600000" ||
		len(parts[2]) != rawURL.EncodedLen(16) || len(parts[3]) != rawURL.EncodedLen(32) {
		t.Fatal("wrong password hash format")
	}
	if match, n, err := verifyPassword(decomposed, hash); err != nil || !match || n != passwordIterations {
		t.Fatalf("valid password verification failed (iterations=%d, err=%v)", n, err)
	}
	if match, _, err := verifyPassword("different password", hash); err != nil || match {
		t.Fatal("incorrect password verified")
	}
	for _, raw := range []string{"short", strings.Repeat("Ж", 129), "\xff"} {
		if _, ok := NormalizePassword(raw); ok {
			t.Fatal("invalid password accepted")
		}
	}
}

func TestSecretEncodingAndLevel(t *testing.T) {
	token, err := newSecret()
	if err != nil || !ValidSecret(token) || len(hashSecret(token)) != 64 || ValidSecret(token+"=") {
		t.Fatal("invalid secret generation or digest")
	}
	for _, sample := range []struct {
		xp   int64
		want int
	}{{0, 1}, {99, 1}, {100, 2}, {402, 2}, {403, 3}, {1_000_000_000, 60}} {
		if got := Level(sample.xp); got != sample.want {
			t.Errorf("Level(%d) = %d, want %d", sample.xp, got, sample.want)
		}
	}
}

func TestConfig(t *testing.T) {
	env := map[string]string{"NEWMMO_DATABASE_URL": "example"}
	getenv := func(name string) string { return env[name] }
	cfg, err := LoadConfig(getenv, "127.0.0.1:18086")
	if err != nil || cfg.WSURL != "ws://127.0.0.1:18086/v1/ws" || cfg.Limits != (Limits{5, 20, 5}) {
		t.Fatalf("invalid dev defaults: %v", err)
	}
	env["NEWMMO_LOGIN_LIMIT_IP"] = "1"
	env["NEWMMO_PUBLIC_WS_URL"] = "wss://example.test/v1/ws"
	if cfg, err := LoadConfig(getenv, "127.0.0.1:18086"); err != nil || cfg.Limits.LoginPerIP != 1 {
		t.Fatalf("override not applied: %v", err)
	}
	env["NEWMMO_PUBLIC_WS_URL"] = "ws://example.test/v1/ws"
	if _, err := LoadConfig(getenv, "127.0.0.1:18086"); err == nil {
		t.Fatal("remote plaintext WebSocket URL was accepted")
	}
	env["NEWMMO_PUBLIC_WS_URL"] = "wss://example.test/v1/ws?ticket=fixture"
	if _, err := LoadConfig(getenv, "127.0.0.1:18086"); err == nil || strings.Contains(err.Error(), "fixture") {
		t.Fatal("expected redacted WebSocket configuration error")
	}
}
