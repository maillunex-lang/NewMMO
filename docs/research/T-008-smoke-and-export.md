# T-008 — Strict headless smoke + JSON export filters

**Date:** 2026-11-21  
**Lane:** client · **Extra-Write:** `tools/smoke_stage1.sh`  
**Engine pin:** Godot **4.3** (unchanged)

## 1. Need

Make `./tools/smoke_stage1.sh` fail on import/script failures (exit code **and** success markers), fix MQ01 marker ordering, and ensure `client/data/**/*.json` is covered by Android export filters.

## 2. Sources

| Source | Used |
|--------|------|
| In-repo | `docs/research/godot-4-7-2-upgrade.md` §4–§6 |
| Godot docs | [Exporting projects](https://docs.godotengine.org/en/stable/tutorials/export/exporting_projects.html), CLI tutorial |
| Hysen Labs | [godot-ci](https://hysenlabs.com/projects/abarichello-godot-ci), [godot-export](https://hysenlabs.com/projects/firebelley-godot-export), [GUT](https://hysenlabs.com/projects/bitwes-gut) |
| GitHub issues | JSON-as-resource / include_filter behavior (e.g. godot#67855) |

## 3. Candidates (≥3)

| # | Approach | Fit | Risks | Verdict |
|---|----------|-----|-------|---------|
| 1 | **Native bash smoke** — require exit 0; capture stdout; `grep` markers `STAGE1_SMOKE_OK` / `MQ01_SMOKE_OK`; no `\|\| true` | Matches current tooling; pin 4.3 | Must document import exit quirks | **Adopt** |
| 2 | [abarichello/godot-ci](https://hysenlabs.com/projects/abarichello-godot-ci) Docker image | Full CI export containers | Out of lane (CI/director); installs image | Research only — **do not install** |
| 3 | [firebelley/godot-export](https://hysenlabs.com/projects/firebelley-godot-export) GH Action | Preset-driven exports | Needs templates + Action; CI | Research only |
| 4 | [GUT](https://hysenlabs.com/projects/bitwes-gut) unit tests | In-engine asserts | Addon + broader rewrite | Not for this gate |

## 4. Import exit-code experiment (this machine)

| Case | Result |
|------|--------|
| Warm `client/` `--import --quit-after 3` | exit **0** |
| Cold copy without `.godot` then `--import --quit-after 8` | exit **0** |

Policy: treat non-zero import as **failure** and print the exit code. Do **not** silence with `|| true`. No benign non-zero observed on this machine for 4.3; if one appears later, document an explicit allowlist with a log line — never silent ignore.

**Artificial fail demos (after strict script):** missing `MQ01_SMOKE_OK` → exit 1; simulated import exit 7 → exit 7.

## 5. JSON / `include_filter`

Godot 4 treats `.json` as resources in many cases, so `export_filter="all_resources"` may already pack them; docs still document **Filters to export non-resource files** (`include_filter`) for `.json`/`.txt`/`.csv`.

**This task:** set `include_filter="data/*.json,data/**/*.json"` as defense-in-depth (director request).

**Verified (this run):**  
`$GODOT --headless --path client --export-pack Android /tmp/t008_lunex.pck`  
then `strings /tmp/t008_lunex.pck | grep data/` →  
`res://data/dialogues/mq_01_01_to_05.json` and `res://data/quests/mq_01_01_to_05.json` present.  
Manual APK check (when Android templates + keystore available):  
`"$GODOT" --headless --path client --export-debug Android /tmp/Lunex.apk` then  
`unzip -l /tmp/Lunex.apk` / `strings` for the same `res://data/...json` paths.

## 6. Recommendation

Implement (1) in `tools/smoke_stage1.sh` + fix `mq01_smoke.gd` marker order + set `include_filter`. No third-party install; no engine upgrade.
