package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/api"
	"github.com/maillunex-lang/NewMMO/server/internal/auth"

	"github.com/maillunex-lang/NewMMO/server/internal/httpservice"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	address := httpservice.Address(os.Getenv)
	config, err := auth.LoadConfig(os.Getenv, address)
	if err != nil {
		return err
	}
	store, err := auth.Open(ctx, config)
	if err != nil {
		return err
	}
	defer store.Close()

	cleanupCtx, stopCleanup := context.WithCancel(ctx)
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for cleanupCtx.Err() == nil {
			pruneCtx, cancel := context.WithTimeout(cleanupCtx, 5*time.Second)
			store.Prune(pruneCtx)
			cancel()
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	defer func() {
		stopCleanup()
		<-cleanupDone
	}()

	return httpservice.Run(ctx, address, logger, httpservice.Application{
		API: api.New(store, config.WSURL), Ready: store.Ready,
	})
}
