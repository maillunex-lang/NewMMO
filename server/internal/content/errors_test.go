package content

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
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
