# Asset licenses — Lunex / NewMMO

Stage 1 art was **generated for this repository** by `tools/spritegen/generate_stage1_assets.py` (original procedural work). From 2026-11-21 (owner OK, T-040) actors and dungeon props are **third-party placeholder art** listed below; the owner replaces it piece by piece. Replacement keeps file names, so swaps are drop-in.

| Path | Author | License | Notes |
|------|--------|---------|-------|
| `client/assets/env/{grass_tile,path_tile,shadow,lantern,crystal}.png` | Lunex / NewMMO procedural generator | Project-owned original | Replace with licensed Blender/Meshy pipeline renders for release |
| `client/assets/ui/*` | Lunex procedural generator | Project-owned original | Portraits / icons |
| `client/assets/vfx/*` | Lunex procedural generator | Project-owned original | Simple hit spark |

No third-party sprites, fonts, music, or Meshy/Blender exports are bundled yet. When real assets are added, append a row with source URL, author, license (e.g. CC0, CC-BY, paid license), and usage constraints before shipping an APK/AAB.
| `client/assets/characters/guardian_sheet.png`, `npc/{elder_mira,feira_niss,ilra_venn}_sheet.png`, `monsters/{bandit,slime,wolf,goblin,zombie,skeleton,ogre}_sheet.png` | «Isometric Hero and Creatures» — OpenGameArt user **Ti Mi** (see `licenses/isometric-hero-and-creatures-SOURCE.txt`) | **CC-BY 3.0 — attribution required** | https://opengameart.org/content/isometric-hero-and-creatures · repacked by `tools/spritegen/import_flare_pack.py` (crop/scale/recomposite = modified). Credit must appear in the game credits. |
| `client/assets/dungeon/*.png` (barrel, crates, chest, column, table, chair) | **Kenney** (www.kenney.nl) — «Isometric Miniature Dungeon» (OpenGameArt copy) | **CC0 1.0** | https://opengameart.org/content/isometric-miniature-dungeon · licence copy `licenses/kenney-isometric-miniature-dungeon-License.txt` · trimmed and downscaled. Crediting Kenney is optional. |

| `client/assets/env/dawn_valley_ground.png`, `dungeon/floor_stone.png` | «Isometric Ground Tiles» — **Drummyfish** (OpenGameArt) | **CC0** | https://opengameart.org/content/isometric-ground-tiles · tessellated by `tools/spritegen/make_valley_ground.py` (road follows `valley_road_mask.png`) |
| `client/assets/env/{tree_oak,tree_pine}.png` | «Free Isometric Plants Pack» — **Dread Knight** (OpenGameArt) | **CC0** | https://opengameart.org/content/free-isometric-plants-pack · trimmed/scaled by `tools/spritegen/make_valley_props.py` |
| `client/assets/env/cottage.png` | «Western European Medieval Houses (isometric 2.5D)» (OpenGameArt) | **CC0** (as shown on the page) | https://opengameart.org/content/western-european-medieval-houses-isometric-25d · trimmed/scaled; author name not shown on the page — re-check before release |
| `client/assets/env/{rock,fence}.png` | Lunex / NewMMO (`make_valley_props.py`, generated) | Project-owned original | Simple generated props |

Other candidate packs verified as CC0 on their pages but **not used yet** (also: Isometric Medieval Buildings 1/2 by Snabisch / edwill chavez, 300 Isometric Overworld Tiles by Screaming Brain Studios, Isometric Trees by mold) (see `docs/research/T-040-art-assets.md`): Kenney Isometric Landscape / City / Buildings / Prototype / Library, Tiny Tactics Battle Kit I (food_please), Isometric Classic Hero + Tiles 32x32 (GrafxKid).
