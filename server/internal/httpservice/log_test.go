package httpservice

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestRequestLoggingHijack(t *testing.T) {
	for _, tc := range []struct {
		name   string
		hijack func(http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error)
	}{
		{"Hijacker", func(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error) {
			return w.(http.Hijacker).Hijack()
		}},
		{"ResponseController", func(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error) {
			return http.NewResponseController(w).Hijack()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, logs := newLoggedTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				requestID := w.Header().Get(requestIDHeader)
				conn, rw, err := tc.hijack(w)
				if err != nil {
					t.Errorf("hijack: %v", err)
					return
				}
				defer conn.Close()
				_, err = fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: test\r\n%s: %s\r\n\r\nraw payload\n", requestIDHeader, requestID)
				if err != nil {
					t.Errorf("write hijacked response: %v", err)
					return
				}
				if err := rw.Flush(); err != nil {
					t.Errorf("flush hijacked response: %v", err)
				}
			})

			conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			request, err := http.NewRequest(http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Connection", "Upgrade")
			request.Header.Set("Upgrade", "test")
			if err := request.Write(conn); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(conn)
			response, err := http.ReadResponse(reader, request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusSwitchingProtocols || response.Header.Get("Upgrade") != "test" {
				t.Fatalf("unexpected upgrade response: %s %v", response.Status, response.Header)
			}
			if payload, err := reader.ReadString('\n'); payload != "raw payload\n" || err != nil {
				t.Fatalf("hijacked payload = %q, error = %v", payload, err)
			}
			checkLoggedResponse(t, logs, http.StatusSwitchingProtocols, response.Header.Get(requestIDHeader))
		})
	}
}

func TestRequestLoggingFlush(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		flush  func(http.ResponseWriter) error
	}{
		{"Flusher", http.StatusOK, func(w http.ResponseWriter) error {
			w.(http.Flusher).Flush()
			return nil
		}},
		{"ResponseController", http.StatusCreated, func(w http.ResponseWriter) error {
			return http.NewResponseController(w).Flush()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			finish := make(chan struct{})
			server, logs := newLoggedTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.status != http.StatusOK {
					w.WriteHeader(tc.status)
				}
				if _, err := io.WriteString(w, "first\n"); err != nil {
					t.Errorf("write first chunk: %v", err)
					return
				}
				if err := tc.flush(w); err != nil {
					t.Errorf("flush first chunk: %v", err)
					return
				}
				select {
				case <-finish:
					if _, err := io.WriteString(w, "second\n"); err != nil {
						t.Errorf("write second chunk: %v", err)
					}
				case <-r.Context().Done():
				}
			})
			client := server.Client()
			client.Timeout = 5 * time.Second
			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.status)
			}
			reader := bufio.NewReader(response.Body)
			if chunk, err := reader.ReadString('\n'); chunk != "first\n" || err != nil {
				t.Fatalf("first chunk = %q, error = %v", chunk, err)
			}
			close(finish)
			if rest, err := io.ReadAll(reader); string(rest) != "second\n" || err != nil {
				t.Fatalf("remaining stream = %q, error = %v", rest, err)
			}
			checkLoggedResponse(t, logs, tc.status, response.Header.Get(requestIDHeader))
		})
	}
}

func TestStatusWriterFlushCommitsImplicitStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := &statusWriter{ResponseWriter: recorder}
	if err := http.NewResponseController(w).Flush(); err != nil {
		t.Fatal(err)
	}
	w.WriteHeader(http.StatusCreated)
	if !recorder.Flushed || recorder.Code != http.StatusOK || w.status != http.StatusOK {
		t.Fatalf("flushed = %v, HTTP status = %d, recorded status = %d", recorder.Flushed, recorder.Code, w.status)
	}
}

func TestStatusWriterUnsupportedOperations(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := &statusWriter{ResponseWriter: struct{ http.ResponseWriter }{recorder}}
	controller := http.NewResponseController(w)
	if conn, rw, err := controller.Hijack(); conn != nil || rw != nil || !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported hijack returned %v, %v, %v", conn, rw, err)
	}
	if err := controller.Flush(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported flush returned %v", err)
	}
	w.Flush()
	w.WriteHeader(http.StatusTeapot)
	if recorder.Flushed || w.status != http.StatusTeapot {
		t.Fatalf("flushed = %v, recorded status = %d after unsupported operations", recorder.Flushed, w.status)
	}
}

func newLoggedTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, <-chan []byte) {
	t.Helper()
	logs := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var log bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&log, nil))
		logRequests(logger, handler).ServeHTTP(w, r)
		logs <- log.Bytes()
	}))
	t.Cleanup(server.Close)
	return server, logs
}

func checkLoggedResponse(t *testing.T, logs <-chan []byte, status int, requestID string) {
	t.Helper()
	select {
	case log := <-logs:
		var entry struct {
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(log, &entry); err != nil {
			t.Fatalf("want one request log, got %q: %v", log, err)
		}
		if entry.Status != status || requestID == "" || entry.RequestID != requestID {
			t.Fatalf("request log = %s, want status %d and request ID %q", log, status, requestID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for request log")
	}
}
