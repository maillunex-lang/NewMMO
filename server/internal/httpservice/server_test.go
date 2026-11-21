package httpservice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAddress(t *testing.T) {
	for _, tc := range []struct {
		name, addr, port, want string
	}{
		{"default", "", "", "127.0.0.1:18086"},
		{"legacy port stays on loopback", "", "9000", "127.0.0.1:9000"},
		{"explicit address", "0.0.0.0:8080", "", "0.0.0.0:8080"},
		{"explicit address overrides legacy port", "[::1]:9001", "9000", "[::1]:9001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NEWMMO_ADDR", tc.addr)
			t.Setenv("PORT", tc.port)
			if got := Address(os.Getenv); got != tc.want {
				t.Fatalf("address = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadyz(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := NewHandler(ctx, quietLogger())

	check := func(status int, ready bool) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("readiness response = %d %v, want status %d and no-store", response.Code, response.Header(), status)
		}
		var body map[string]bool
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if got, ok := body["ok"]; !ok || got != ready {
			t.Fatalf("readiness body = %v, want ok=%v", body, ready)
		}
	}

	check(http.StatusOK, true)
	cancel()
	check(http.StatusServiceUnavailable, false)
}

func TestShutdownDrainsActiveRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	shuttingDown := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	})}
	server.RegisterOnShutdown(func() { close(shuttingDown) })
	url, cancel, stopped := startServer(t, server, 5*time.Second)
	requestDone := getStatus(url)

	await(t, started)
	cancel()
	await(t, shuttingDown)
	select {
	case err := <-stopped:
		t.Fatalf("server returned before request completed: %v", err)
	default:
	}

	close(release)
	if err := await(t, requestDone); err != nil {
		t.Fatal(err)
	}
	if err := await(t, stopped); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownClosesActiveConnectionOnTimeout(t *testing.T) {
	started := make(chan struct{})
	disconnected := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(disconnected)
	})}
	url, cancel, stopped := startServer(t, server, 25*time.Millisecond)
	requestDone := getStatus(url)

	await(t, started)
	cancel()
	if err := await(t, stopped); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error = %v, want deadline exceeded", err)
	}
	await(t, disconnected)
	if err := await(t, requestDone); err == nil {
		t.Fatal("request completed after its connection was closed")
	}
}

func startServer(t *testing.T, server *http.Server, timeout time.Duration) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
	})
	stopped := make(chan error, 1)
	go func() { stopped <- serve(ctx, server, listener, timeout) }()
	return "http://" + listener.Addr().String(), cancel, stopped
}

func getStatus(url string) <-chan error {
	done := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				err = errors.New(response.Status)
			}
		}
		done <- err
	}()
	return done
}

func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case value := <-ch:
		return value
	case <-timer.C:
		t.Fatal("timed out waiting for shutdown test")
		var zero T
		return zero
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
