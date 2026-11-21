# T-040 — free/CC0 art for the game (tiles, props, heroes, monsters)

**Question (owner, 2026-11-21):** use ready-made art from the internet now; the owner replaces it piece by piece later.
**Status:** research only. **Nothing downloaded or vendored** — awaiting owner OK (AGENTS.md: new third-party assets need owner approval).
**Verification note:** found by web search on 2026-11-21 from itch.io / OpenGameArt / kenney.nl listing pages. Licences below are what the listing pages say; **the licence file inside each download must be re-checked before it is committed** (listing pages can differ from the archive, and OpenGameArt licences vary per page).

## What the game needs (current format)
`DirectionalAnimator`: one sheet per actor, **8 direction rows × columns idle(4) | run(6) | attack(4)**, frame size per actor (now 64×64), isometric 2.5D, small file sizes for Android ~6 GB. Tiles 64×64-ish, props ~64–140 px. Frame counts/size are already exported properties, so a different pack only needs a **repack script** (`tools/spritegen/import_pack.py`, to be written) that arranges its frames into this grid.

## Candidates
| # | Pack | Content | Licence (per listing) | Fit / risks |
|---|------|---------|-----------------------|-------------|
| 1 | **Kenney — Isometric Dungeon Tiles** ([itch](https://kenney-assets.itch.io/isometric-dungeon-tiles)) | isometric dungeon tiles + a character, **8 directions × 3 animations** | CC0 1.0, no attribution required | Best match for the dungeon (T-037) and the character grid. Small character set (1 hero). |
| 2 | **Kenney — Isometric Prototype Tiles / Library Tiles** ([prototype](https://kenney-assets.itch.io/isometric-prototypes-tiles), [library](https://kenney-assets.itch.io/isometric-library-tiles)) | 50+ / 30+ tiles + character sprites | CC0 | Good for interiors/props; same author ⇒ consistent look with #1. |
| 3 | Kenney — Isometric Landscape / City tiles ([landscape](https://www.kenney.nl/assets/isometric-landscape), [city](https://kenney.nl/assets/isometric-tiles-city)) | terrain, trees, buildings | CC0 (kenney.nl says "see the licence file in the download") | Outdoor valley (REG-01) — verify the archive's licence file. |
| 4 | **DezrasDragons — Isometric Classic Hero + Tiles (32×32)** ([OpenGameArt](https://opengameart.org/content/isometric-hero-and-creatures) lists related work; check the author's page) | 8 animated characters (idle, walk, attack, hit) + tileset | CC0 (per search listing) | Needs 32→64 upscale or per-actor frame size; verify page. |
| 5 | Tactical RPG isometric set (OpenGameArt node 162543) | Mage, Cleric, Fighter, 4 animations | tagged CC0 (unconfirmed) | Class-like heroes; verify licence on the page. |
| 6 | Clint Bellanger — *Isometric Hero and Creatures* ([OpenGameArt](https://opengameart.org/content/isometric-hero-and-creatures)) | classic isometric hero + monsters | **not shown in results — unverified** | Monsters are the weakest area of CC0 isometric art; check licence and attribution terms first. |
| 7 | itch.io: chierit 8×8 Isometric TRPG pack; Engvee free isometric tileset; *Cursed Kingdoms* monsters | tiles, characters, monsters | CC0 / free **unconfirmed** per pack | Possible monster source; each needs a page-level licence check. |

Excluded: anything CC-BY-NC / "no commercial use", AI-generated packs with unclear rights, paid bundles (Kenney's $4.99 isometric bundle is optional, not needed).

## Recommendation
1. **One primary source for visual consistency: Kenney (#1–#3, all CC0).** Mixing many styles makes a world look patchy; a single author gives a coherent placeholder look. Use #4–#7 only to fill gaps (monsters, extra hero classes) after per-page licence checks.
2. **Process (after owner OK):** download a first batch into `client/assets/third_party/<pack>/` with the pack's licence file copied next to it; add a row per pack to `client/assets/LICENSES.md` (URL, author, licence, date, what it is used for); write `tools/spritegen/import_pack.py` to repack into our 8-dir grid; swap **by keeping our existing file names** (`guardian_sheet.png`, `slime_sheet.png`, `tree_oak.png`, …) so the owner's later replacements are drop-in; keep the procedural originals in git history.
3. **Scope of the first batch:** valley terrain + props (trees, cottage, fence, rocks), dungeon tiles for R01–R07, one hero sheet, 3 monster stand-ins. Heroes per class and canon monsters come as the owner's own art arrives.
4. **Risks:** style mismatch between packs; frame-count mismatch (repack script mitigates); tile size/scale vs our 64 px grid; Android memory (atlas, texture compression) — measured later with the device gate (R-DEVICE); attribution duty for any non-CC0 pack (tracked in `LICENSES.md` and an in-game credits screen).

## Decision needed from the owner
OK to download **Kenney CC0 packs #1–#3** into the repo and wire them in as placeholder art (first batch above)? Other packs only after a licence check on their pages.
