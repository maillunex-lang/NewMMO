package httpservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func Address(getenv func(string) string) string {
	if addr := getenv("NEWMMO_ADDR"); addr != "" {
		return addr
	}
	port := getenv("PORT")
	if port == "" {
		port = "18086"
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func Run(ctx context.Context, addr string, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	server := &http.Server{
		Handler:           NewHandler(ctx, logger),
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("HTTP server listening", "addr", listener.Addr().String())
	if err := serve(ctx, server, listener, 10*time.Second); err != nil {
		return err
	}
	logger.Info("HTTP server stopped")
	return nil
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, shutdownTimeout time.Duration) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := server.Shutdown(shutdownCtx)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		listenErr := <-serveErr
		if !errors.Is(listenErr, http.ErrServerClosed) {
			err = errors.Join(err, listenErr)
		}
		if err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}

func NewHandler(lifetime context.Context, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"service": "newmmo-dev",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		ready := lifetime.Err() == nil
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": ready})
	})
	return logRequests(logger, mux)
}
