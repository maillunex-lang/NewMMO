package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/maillunex-lang/NewMMO/server/internal/auth"
	"github.com/maillunex-lang/NewMMO/server/internal/combatdata"
	"github.com/maillunex-lang/NewMMO/server/internal/equipment"
	"github.com/maillunex-lang/NewMMO/server/internal/loot"
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
	lootCat := loot.NewCatalog()
	if err := lootCat.LoadFromDB(ctx, pool, buildID); err != nil {
		return nil, fmt.Errorf("load loot tables: %w", err)
	}
	eqCat := equipment.NewCatalog()
	if err := eqCat.LoadFromDB(ctx, pool, buildID); err != nil {
		return nil, fmt.Errorf("load equipment templates: %w", err)
	}
	lootEngine := loot.NewEngine(pool, lootCat, loot.LoadConfig(os.Getenv)).WithEquipment(eqCat)
	runCtx, cancel := context.WithCancel(ctx)
	// Server-observed kills grant loot (closes R-KILL for the open world): one personal roll per contributor.
	killHook := func(k world.KillEvent) {
		for _, c := range k.Contributors {
			gctx, stop := context.WithTimeout(runCtx, 10*time.Second)
			res, err := lootEngine.GrantKill(gctx, c.AccountID, c.Character, k.EnemyID)
			stop()
			if err != nil {
				logger.Warn("kill loot not granted", "character_id", c.Character, "enemy_id", k.EnemyID, "error", err.Error())
				continue
			}
			if c.Push != nil {
				c.Push("combat.loot.result", res.Revision, res)
			}
		}
	}
	var zones []*world.Zone
	for _, d := range defs {
		z, err := world.NewZone(d, cat)
		if err != nil {
			cancel()
			return nil, err
		}
		z.SetKillHook(killHook)
		zones = append(zones, z)
		go z.Run(runCtx)
	}
	socket.SetWorld(world.NewHandler(zones, worldPlayerLoader(store)))
	logger.Info("world simulation ready", "build_id", buildID, "zones", len(zones))
	return cancel, nil
}

func worldPlayerLoader(store *auth.Store) world.PlayerLoader {
	return func(ctx context.Context, accountID string, characterID int64) (world.PlayerInfo, error) {
		st, err := store.LoadWSState(ctx, accountID, characterID)
		if err != nil {
			return world.PlayerInfo{}, err
		}
		return world.PlayerInfo{Name: st.Name, ClassID: st.ClassID, Level: st.Level}, nil
	}
}
