package content

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRejectAuditFailurePreservesErrorAndRedactsLog(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		if conn, err := listener.Accept(); err == nil {
			_ = conn.Close()
		}
	}()

	const dummySecret = "test-only-dsn-secret"
	url := "postgres://fixture:" + dummySecret + "@" + listener.Addr().String() + "/content?sslmode=disable&connect_timeout=1"
	store, err := Open(t.Context(), url, "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var log bytes.Buffer
	store.auditLogger = slog.New(slog.NewJSONHandler(&log, nil))
	_, err = store.Stage(t.Context(), t.TempDir())
	if ErrorCode(err) != BundleInvalid || !strings.Contains(err.Error(), "BUILD_ID") {
		t.Fatalf("audit failure did not preserve verification error: %v", err)
	}
	if strings.Contains(log.String(), dummySecret) || !strings.Contains(log.String(), `"audit_code":"DB_ERROR"`) ||
		!strings.Contains(log.String(), `"code":"BUNDLE_INVALID"`) {
		t.Fatalf("unexpected audit failure log %s", &log)
	}
}

func TestRejectAuditDeadline(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// The peer accepts TCP and stalls during PostgreSQL startup. Its
	// connection timeout is deliberately longer than the reject deadline.
	store, err := Open(t.Context(), "postgres://fixture@"+listener.Addr().String()+
		"/content?sslmode=disable&connect_timeout=6", "audit-deadline-test")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	release := make(chan struct{})
	defer close(release)
	go func() {
		if conn, err := listener.Accept(); err == nil {
			defer conn.Close()
			<-release
		}
	}()

	var log bytes.Buffer
	store.auditLogger = slog.New(slog.NewJSONHandler(&log, nil))
	dir := t.TempDir()
	start := time.Now()
	_, err = store.Stage(t.Context(), dir)
	store.Close() // The CLI closes the pool while the peer is still stalled.
	elapsed := time.Since(start)
	if err == nil || ErrorCode(err) != BundleInvalid || !strings.Contains(log.String(), `"audit_code":"DB_ERROR"`) {
		t.Fatalf("expected original validation error and audit failure log, got %v and %s", err, &log)
	}
	if elapsed < 1800*time.Millisecond || elapsed > 4*time.Second {
		t.Fatalf("reject audit finished in %s; want about 2 seconds", elapsed)
	}
	t.Logf("reject audit deadline elapsed in %s", elapsed)
}
