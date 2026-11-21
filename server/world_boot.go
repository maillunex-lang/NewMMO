package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
	"github.com/maillunex-lang/NewMMO/server/internal/world"
	"github.com/maillunex-lang/NewMMO/server/internal/ws"
)

// attachWorld starts the zone simulations (T-038b). It returns the cancel func that stops the tick loops.
func attachWorld(ctx context.Context, store *auth.Store, socket *ws.Server, logger *slog.Logger) (context.CancelFunc, error) {
	pool := store.Pool()
	if pool == nil {
		return nil, fmt.Errorf("auth store pool unavailable")
	}
	buildID, err := quest.NewCatalog().ActiveBuildID(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("active build: %w", err)
	}
	cat, err := combatdata.LoadFromDB(ctx, pool, buildID)
	if err != nil {
		return nil, fmt.Errorf("load combat data: %w", err)
	}
	if err := cat.Validate(); err != nil {
		return nil, fmt.Errorf("combat data: %w", err)
	}
	defs, err := world.LoadZoneDefs(world.ResolveZonesDir(os.Getenv("NEWMMO_ZONES_DIR")))
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	var zones []*world.Zone
	for _, d := range defs {
		z, err := world.NewZone(d, cat)
		if err != nil {
			cancel()
			return nil, err
		}
		zones = append(zones, z)
		go z.Run(runCtx)
	}
	loader := func(ctx context.Context, accountID string, characterID int64) (world.PlayerInfo, error) {
		st, err := store.LoadWSState(ctx, accountID, characterID)
		if err != nil {
			return world.PlayerInfo{}, err
		}
		return world.PlayerInfo{Name: st.Name, ClassID: st.ClassID, Level: st.Level}, nil
	}
	socket.SetWorld(world.NewHandler(zones, loader))
	logger.Info("world simulation ready", "build_id", buildID, "zones", len(zones))
	return cancel, nil
}
