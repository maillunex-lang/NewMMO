package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/httpservice"
)

func TestHealthz(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := httpservice.NewHandler(ctx, logger, httpservice.Application{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("health response = %d %v", response.Code, response.Header())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 3 || body["ok"] != true || body["service"] != "newmmo-dev" {
		t.Fatalf("unexpected health body: %v", body)
	}
	stamp, ok := body["time"].(string)
	parsed, err := time.Parse(time.RFC3339, stamp)
	if !ok || err != nil || parsed.Location() != time.UTC {
		t.Fatalf("health time = %v, want UTC RFC3339", body["time"])
	}
}
