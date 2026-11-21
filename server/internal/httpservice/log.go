package httpservice

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const requestIDHeader = "X-Request-ID"

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var idBytes [16]byte
		if _, err := rand.Read(idBytes[:]); err != nil {
			logger.Error("HTTP request ID generation failed")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		id := hex.EncodeToString(idBytes[:])
		w.Header().Set(requestIDHeader, id)

		response := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.Info("HTTP request", "request_id", id, "status", status, "duration_ns", time.Since(start).Nanoseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	if w.status == 0 && (status == http.StatusSwitchingProtocols || status >= 200) {
		w.status = status
	}
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		// The handler writes an upgrade response directly to the connection.
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

func (w *statusWriter) Flush() {
	_ = w.FlushError()
}

func (w *statusWriter) FlushError() error {
	if err := http.NewResponseController(w.ResponseWriter).Flush(); err != nil {
		return err
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return nil
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
