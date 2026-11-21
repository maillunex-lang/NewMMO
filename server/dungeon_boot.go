package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/dungeon"
	"github.com/maillunex-lang/NewMMO/server/internal/loot"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
	"github.com/maillunex-lang/NewMMO/server/internal/ws"
)

func attachDungeonEngine(ctx context.Context, store *auth.Store, socket *ws.Server, logger *slog.Logger) error {
	pool := store.Pool()
	if pool == nil {
		return fmt.Errorf("auth store pool unavailable")
	}
	buildID, err := quest.NewCatalog().ActiveBuildID(ctx, pool)
	if err != nil {
		return fmt.Errorf("active build: %w", err)
	}
	lootCat := loot.NewCatalog()
	if err := lootCat.LoadFromDB(ctx, pool, buildID); err != nil {
		return fmt.Errorf("load loot tables: %w", err)
	}
	defs, err := dungeon.LoadDir(dungeon.ResolveDir(os.Getenv("NEWMMO_DUNGEONS_DIR")))
	if err != nil {
		return fmt.Errorf("dungeon definitions: %w", err)
	}
	lcfg := loot.LoadConfig(os.Getenv)
	engine := dungeon.NewEngine(pool, defs, lootCat, dungeon.Config{Regions: lcfg.Regions, RegionCoef: lcfg.RegionCoef})
	if err := engine.ValidateAgainstContent(); err != nil {
		return fmt.Errorf("dungeon definitions vs content: %w", err)
	}
	socket.SetDungeon(engine)
	logger.Info("dungeon engine ready", "build_id", buildID, "dungeons", len(defs.IDs()))
	return nil
}
