package main

import (
	"fmt"
	"log/slog"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/inventory"
	"github.com/maillunex-lang/NewMMO/server/internal/ws"
)

func attachInventoryEngine(store *auth.Store, socket *ws.Server, logger *slog.Logger) error {
	pool := store.Pool()
	if pool == nil {
		return fmt.Errorf("auth store pool unavailable")
	}
	engine := inventory.NewEngine(pool)
	socket.SetInventory(inventory.Adapter{Engine: engine})
	logger.Info("inventory engine ready")
	return nil
}
