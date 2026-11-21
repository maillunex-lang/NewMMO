package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	return httpservice.Run(ctx, httpservice.Address(os.Getenv), logger)
}
