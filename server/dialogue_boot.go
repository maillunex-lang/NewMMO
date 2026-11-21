package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/dialogue"
	"github.com/maillunex-lang/NewMMO/server/internal/ws"
)

func attachDialogueEngine(ctx context.Context, store *auth.Store, socket *ws.Server, logger *slog.Logger) error {
	path := dialogue.ResolveFlowsPath(os.Getenv("NEWMMO_DIALOGUE_FLOWS_PATH"))
	cat := dialogue.NewCatalog()
	if err := cat.LoadFlows(path); err != nil {
		return fmt.Errorf("dialogue flows: %w", err)
	}
	pool := store.Pool()
	if pool == nil {
		return fmt.Errorf("auth store pool unavailable")
	}
	buildID, err := cat.ActiveBuildID(ctx, pool)
	if err != nil {
		return fmt.Errorf("active build: %w", err)
	}
	if err := cat.LoadLinesFromDB(ctx, pool, buildID); err != nil {
		return fmt.Errorf("load dialogue lines: %w", err)
	}
	engine := dialogue.NewEngine(pool, cat)
	socket.SetDialogue(dialogue.Adapter{Engine: engine})
	logger.Info("dialogue engine ready", "build_id", buildID, "flows", path)
	return nil
}
