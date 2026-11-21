# Research: isometric 2.5D location procedural generation (Godot 4)

**Date:** 2026-11-21  
**Repo:** `maillunex-lang/NewMMO`  
**Status:** Research only — **nothing installed or vendored**  
**Awaiting:** owner confirmation before any integration

## 1. Problem statement

NewMMO needs a **hybrid** location generator (WORLD-GEN-001) that:

- Creates biomes/terrain, paths, vegetation groups, decorative clusters with a deterministic seed
- Writes into **editable `.tscn`** scenes (Godot Inspector / World Studio)
- Never overwrites **Manual** edits (stable IDs, Manual vs Generated layers, preview/diff)
- Supports **isometric 2.5D** presentation (2D sprites / TileMapLayer — **no runtime 3D**)
- Later exports validated collision/spawns/navigation for the **authoritative Go server**

Current pin: Godot **4.3** (`client/project.godot`). Planning docs mention **4.7.x** as a future target; Godot 4.7.2 was not available as a verified binary in this environment. Compatibility notes below use **4.3 APIs (`TileMapLayer`)** and flag version drift risk.

## 2. Search sources

| Source | Query / path | Notes |
|--------|----------------|-------|
| GitHub | procedural generation, WFC, isometric, TileMapLayer, Godot 4 | Primary candidates below |
| Godot Asset Library | Gaea (#3272), WFC (#1951), Procedural-Map-Generator (#2979), GenMap Lite, Dingo | Verified listing pages |
| Hysen Labs | GDQuest procedural generation write-up | https://hysenlabs.com/projects/gdquest-demos-godot-4-procedural-generation |
| Official docs | Godot 4 TileMapLayer / isometric TileSet | Built-in iso grid + Y-sort (no addon required for basics) |

## 3. Constraint filter (applied to all candidates)

- Commercial-friendly license preferred (MIT/Apache/BSD). Avoid unclear / course-only licenses for production code.
- Must not force **runtime 3D** client worlds (rejects GridMap/3D dungeon generators as primary path).
- Must allow **visual editing** after generation, or generate into standard Godot nodes that remain editable.
- Prefer small addons over replacing Lunex World Studio / Go server.

## 4. Shortlisted candidates (≥3)

### A. Gaea (`gaea-godot/gaea`)

| Field | Finding |
|-------|---------|
| URL | https://github.com/gaea-godot/gaea |
| Asset Library | https://godotengine.org/asset-library/asset/3272 (listed for 4.3 / v1.x era) |
| Docs | https://gaea-docs.readthedocs.io/ |
| License | **MIT** (commercial OK) |
| Stars / activity | ~1666★; `pushed_at` 2026-10-18; default branch `2.0` |
| Working code | Yes — maintained addon with docs and demos |
| Godot | **2.0 targets Godot 4.4+**; **v1.X for ≤4.3** (explicit upstream note) |
| Android | Pure GDScript addon path looks exportable; verify no editor-only deps before adopt. Generation should stay **editor/offline** for MMO authored zones |
| Fit | Graph-based generators → renderers can paint **TileMap** / textures / custom outputs. Strong for biomes/caves/terrain. Does **not** by itself implement Lunex Manual-layer protection or prop scene instancing |
| Risks | 2.0 still “early”; version split vs our 4.3 pin; easy to over-adopt as runtime generator (we need authored `.tscn` permanence) |

### B. Wave Function Collapse — `AlexeyBond/godot-constraint-solving`

| Field | Finding |
|-------|---------|
| URL | https://github.com/AlexeyBond/godot-constraint-solving |
| Asset Library | https://godotengine.org/asset-library/asset/1951 (Godot 4.3 listing) |
| License | **MIT** |
| Stars / activity | ~466★; last push **2025-05-13** (slower than Gaea; still usable) |
| Working code | Yes — WFC2DGenerator, samples, TileMapLayer support |
| Godot | Godot 4; supports **`TileMapLayer`**, legacy TileMap, optional GridMap slices |
| Android | Algorithm is GDScript; fine if used offline/editor. Runtime WFC on mobile for large maps is a perf risk — not recommended for MMO zones |
| Fit | Excellent for **rule-consistent terrain/path tiles** learned from a hand-authored sample map. Preserves ability to leave pre-placed cells. Closest match to “generate then keep editing TileMap” |
| Risks | Not a full open-world biome + prop scatter system; hex/iso need careful tileset rules; multithreading/backtracking cost |

### C. TileMapDual (`pablogila/TileMapDual`)

| Field | Finding |
|-------|---------|
| URL | https://github.com/pablogila/TileMapDual |
| License | **MIT** |
| Stars / activity | ~1324★; push 2026-09-03; very active community interest |
| Working code | Yes — dual-grid autotile `TileMapLayer` replacement, **isometric supported**, real-time **in-editor** updates |
| Godot | Godot 4 TileMapLayer-based |
| Android | Should export if used as runtime node; primarily an **editing/autotile** aid |
| Fit | **Not a world generator**. Complements generators by making isometric terrain painting cheaper (15-tile dual grid). Helps WORLD-EDITOR visual quality |
| Risks | Extra node type vs stock TileMapLayer; art pipeline must match dual-grid template |

### D. GenMap Lite (Asset Library)

| Field | Finding |
|-------|---------|
| URL | https://godotengine.org/asset-library / GenMap Lite listings (Godot **4.3+**, MIT) |
| License | **MIT** (Lite) |
| Working code | Editor plugin: one-click rooms/corridors → `TileMapLayer`, Ctrl+Z undo, rooms as Area2D |
| Fit | Strong for **orthogonal dungeon rooms**, weak for **open isometric continent regions** and prop-layer MMO locations |
| Risks | Wrong genre shape for Dawn Valley / Asterville overworld; still useful later for dungeon Stage 5 research |

### E. GDQuest procedural demos (via Hysen Labs + GitHub)

| Field | Finding |
|-------|---------|
| URLs | https://github.com/gdquest-demos/godot-4-procedural-generation · Hysen Labs analysis page |
| License | **Other / NOASSERTION** (course material — **not cleared for commercial copy**) |
| Fit | Teaching demos (RandomWalker, WorldMap, BasicDungeon). Good **algorithm reference**, not a dependency |
| Risks | License blocks vendoring; not a plugin API |

### Rejected as primary (with reason)

| Project | Why rejected for this system |
|---------|------------------------------|
| Dingo Room Generator / 3D dungeon generators | **Runtime 3D / GridMap** oriented — conflicts with NewMMO 2D-only client rule |
| `moosyu/procedural-isometric-godot` | Prototype, 0★, itch.io tiles of unclear license for our shipping |
| `SaroltaMihaly/Procedural-Map-Generator` | Older TileMap workflow, Python sidecar for Gram-Elites — heavier/safer to skip |
| Fast WFC (C++ wrapper, Asset Lib #4149) | Interesting perf, but native dependency + younger project; revisit only if pure GDScript WFC is too slow offline |

## 5. Comparison matrix (location overworld needs)

| Criterion | Gaea | WFC (AlexeyBond) | TileMapDual | GenMap Lite |
|-----------|------|------------------|---------------|-------------|
| MIT / commercial | Yes | Yes | Yes | Yes |
| Active enough | High | Medium | High | Active listing |
| Godot 4.3 today | v1.X branch | Yes (Asset Lib 4.3) | Yes (4.x) | Yes |
| Path to 4.7.x | Likely via 2.0 | Likely | Likely | Likely |
| Isometric terrain | Possible via tileset | Via TileMapLayer rules | **First-class iso autotile** | Orthogonal dungeon bias |
| Editable after gen | If renderer writes TileMap/scenes | Writes TileMapLayer | Live editor paint | Undo + hand tweak |
| Manual-safe regen | **Must wrap ourselves** | Partial (prefilled cells) | N/A (not gen) | Undo only |
| Prop `.tscn` scatter | Custom / own nodes | No | No | Limited markers |
| Android-safe if offline-gen | Yes | Yes | Yes | Editor-focused |
| Architecture invasiveness | Medium–High | Low–Medium | Low | Medium (dungeon-centric) |

## 6. Recommendation

### Preferred approach (hybrid — do **not** replace World Studio)

**Build a thin Lunex generator** that owns Manual/Generated policy, and optionally **reuse two small MIT components** after confirmation:

1. **Terrain fill:** `AlexeyBond/godot-constraint-solving` (WFC) **or** **Gaea v1.X** (for Godot 4.3) / Gaea 2.0 after engine bump — generate into a `Generated` `TileMapLayer` inside the location `.tscn`.
2. **Isometric painting UX:** consider **TileMapDual** later as a Ground-layer helper (optional, separate decision).
3. **Prop/vegetation/buildings/NPC markers:** keep **Lunex World Studio** + reusable prop scenes; generator only instances under `Generated/*` with stable IDs.
4. **Never** treat GDQuest demos as a dependency (license). Algorithms may inspire in-house code only.
5. **Do not** adopt 3D dungeon generators for overworld.

### Why not “just install Gaea”?

Gaea is the strongest general procgen framework, but:

- Version split (4.3 vs 4.4+/2.0) vs our pin
- Does not encode Lunex Manual-layer / server export contracts
- Risk of making generation runtime-owned instead of authored `.tscn` content

Safer path: **confirm** → spike Gaea **or** WFC in an isolated branch under `client/addons/` with a wrapper that only writes `Generated` nodes and proves “move tree → regen → tree remains.”

### If none are approved

Implement in-house: seed noise + biome masks + road spline + Poisson prop scatter → serialize to `.tscn` / PackedScene diffs. More work, full control over Manual protection and Android/offline tooling.

## 7. Safe integration sketch (only after confirmation)

```text
tools/ or editor plugin "Lunex Generate"
  → read seed + biome params + forbidden/manual masks
  → optional: call WFC/Gaea for Ground/Paths TileMapLayer (Generated)
  → instance props under Generated/Vegetation with server_object_id
  → never modify Manual/* or Gameplay/* without diff UI
  → export JSON for server spawns/collision separately
```

Acceptance test (already required by WORLD_EDITOR):

1. Move Manual tree → regenerate → Manual tree unchanged  
2. Open `.tscn` in Godot → edit visually  
3. Android build unaffected (generator editor-only)

## 8. Confirmation requested

Please choose one:

- **A.** Spike **WFC** (`godot-constraint-solving`) + Lunex wrapper (lowest invasiveness on 4.3)  
- **B.** Spike **Gaea v1.X** on Godot 4.3 (richer biome graphs)  
- **C.** In-house only (no third-party addon)  
- **D.** Defer generator; continue Stage 2 online / hand-authored locations  

**Nothing has been installed.** No code from the candidates was copied into this repository.
