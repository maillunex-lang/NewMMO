package httpservice

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogging(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := logRequests(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))

	for _, tc := range []struct {
		method string
		status int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodPost, http.StatusTeapot},
	} {
		request := httptest.NewRequest(tc.method, "/private-path?key=query-secret", strings.NewReader("body-secret"))
		request.Header.Set("Authorization", "Bearer header-secret")
		request.Header.Set("Cookie", "session=cookie-secret")
		request.Header.Set(requestIDHeader, "caller-secret")

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("status = %d, want %d", response.Code, tc.status)
		}
		id := response.Header().Get(requestIDHeader)
		if decoded, err := hex.DecodeString(id); err != nil || len(decoded) != 16 {
			t.Fatalf("response request ID = %q, want 16 hex-encoded bytes", id)
		}
		log := logs.String()
		if strings.Contains(log, "secret") || strings.Contains(log, "private-path") {
			t.Fatalf("request data leaked into log: %s", log)
		}
		var fields map[string]any
		if err := json.Unmarshal(logs.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		duration, ok := fields["duration_ns"].(float64)
		if !ok || duration < 0 || fields["request_id"] != id || fields["status"] != float64(tc.status) {
			t.Fatalf("unexpected request log: %v", fields)
		}
		logs.Reset()
	}
}
