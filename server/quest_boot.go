package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
	"github.com/maillunex-lang/NewMMO/server/internal/ws"
)

func attachQuestEngine(ctx context.Context, store *auth.Store, socket *ws.Server, logger *slog.Logger) error {
	regions := quest.LoadRegionConfig(os.Getenv)
	path := quest.ResolveInteractablesPath(regions.InteractablesPath)
	cat := quest.NewCatalog()
	if err := cat.LoadInteractables(path); err != nil {
		return fmt.Errorf("interactables: %w", err)
	}
	pool := store.Pool()
	if pool == nil {
		return fmt.Errorf("auth store pool unavailable")
	}
	buildID, err := cat.ActiveBuildID(ctx, pool)
	if err != nil {
		return fmt.Errorf("active build: %w", err)
	}
	if err := cat.LoadFromDB(ctx, pool, buildID); err != nil {
		return fmt.Errorf("load quests: %w", err)
	}
	engine := quest.NewEngine(pool, cat, regions)
	socket.SetQuests(quest.Adapter{Engine: engine})
	logger.Info("quest engine ready", "build_id", buildID, "quests", len(cat.All()), "interactables", path)
	return nil
}
