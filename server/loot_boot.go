package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/loot"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
	"github.com/maillunex-lang/NewMMO/server/internal/ws"
)

func attachLootEngine(ctx context.Context, store *auth.Store, socket *ws.Server, logger *slog.Logger) error {
	pool := store.Pool()
	if pool == nil {
		return fmt.Errorf("auth store pool unavailable")
	}
	buildID, err := quest.NewCatalog().ActiveBuildID(ctx, pool)
	if err != nil {
		return fmt.Errorf("active build: %w", err)
	}
	cat := loot.NewCatalog()
	if err := cat.LoadFromDB(ctx, pool, buildID); err != nil {
		return fmt.Errorf("load loot tables: %w", err)
	}
	socket.SetLoot(loot.Adapter{Engine: loot.NewEngine(pool, cat, loot.LoadConfig(os.Getenv))})
	logger.Info("loot engine ready", "build_id", buildID, "tables", cat.Len())
	return nil
}
