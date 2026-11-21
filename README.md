# Lunex MMORPG (NewMMO)

Landscape Android online fantasy MMORPG with a 2.5D isometric world, prerendered animated characters, and editable Godot 4 UI.

## Status

Work happens on `main` only. Done: Stage 1 visual slice (Dawn Valley, local combat, editable props, themed HUD, World Studio dock), MQ-01-01…05 as a **local client stub**, full content canon `docs/content/01…14`, machine-checked canon (EX-P01/EX-P02: parsers, JSON Schema, crossrefs, quest graph, CI lint). **Not done:** authoritative server (Go `/healthz` stub only), PostgreSQL, SQ/DNG/EVT gameplay, Android build. Execution order and status: `docs/ROADMAP_EXECUTION.md`.

Documentation: GDD-01…11 approved; GDD-12 architecture is a baseline proposal. See `docs/AUDIT.md` and `docs/IMPLEMENTATION_PLAN.md`.

## Non-negotiable constraints

- Godot 4 2D client; no runtime 3D characters.
- Animated directional spritesheets exported offline from Blender (source models may come from Meshy). Stage 1 ships **procedural original** interim sheets — see `client/assets/LICENSES.md`.
- All major UI screens built as editable Godot Control scenes (`.tscn`), with reusable components and Theme resources. Do not generate the primary UI entirely in GDScript.
- Android landscape, responsive safe areas and customizable HUD, target 6 GB RAM devices.
- Authoritative Go server, PostgreSQL, WebSocket, React/TypeScript admin.
- Never trust the client for combat outcomes, currency, inventory or movement authorization.
- Keep development assets, credentials and production services separate.

## Documentation

- [Agent standing rules](AGENTS.md)
- [Execution roadmap (AI checklist)](docs/ROADMAP_EXECUTION.md)
- [AI director protocol (multi-agent lanes, tasks)](docs/director/DIRECTOR.md)
- [Content tooling (lint, schemas, sync checks)](tools/content/README.md)
- [Approved design decisions](docs/GDD.md)
- [Content canon](docs/content/README.md) (start: `14_CONTENT_INTEGRATION.md`)
- [Technical architecture and vertical slice](docs/ARCHITECTURE.md)
- [Implementation roadmap](docs/ROADMAP.md)
- [World editor](docs/WORLD_EDITOR.md)
- [Audit](docs/AUDIT.md)
- [Implementation plan](docs/IMPLEMENTATION_PLAN.md)
- [Research notes](docs/research/)

## Run locally

```bash
# Client (Godot 4.3+)
godot --path client

# Dev API stub
cd server && go run .
curl http://127.0.0.1:18086/healthz

# Smoke
./tools/smoke_stage1.sh

# Content canon checks (Python 3, stdlib only)
python3 tools/content/lint.py
python3 tools/content/check_client_sync.py
python3 tools/content/check_docs_links.py
python3 -m unittest discover -s tools/content/tests
```

## First milestone

One region, one playable class stand-in, movement, one NPC, quests, a few enemies, skill combat, loot — then live server synchronization (Stage 2). Other classes and MMO systems are planned, not cut from the game.

## License

No license has been selected for the project source. Public source visibility does not by itself grant reuse rights. Asset licenses are tracked in `client/assets/LICENSES.md`.
