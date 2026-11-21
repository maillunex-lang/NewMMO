# AUDIT_BASELINE — EX-P00 (read-only)

**Repo:** `maillunex-lang/NewMMO`  
**Branch / commit:** `main` @ `7567a54` (audit body; tip may include stamp commits)  
**Canon orchestrator:** `docs/content/14_CONTENT_INTEGRATION.md`  
**Execution map:** `docs/ROADMAP_EXECUTION.md`  
**Date:** 2026-11-21  
**Scope:** factual implementation vs canon 01–14. **No gameplay code changed. No VPS touched. Canon MD not rewritten.**

**Statuses:** `DONE` | `PARTIAL` | `MISSING` | `BLOCKED`

---

## 0. Executive verdict

The repo is a **playable local Stage-1 / MQ-01-01…05 stub**, not an authoritative MMORPG. Canon docs 01–14 are present (~13k lines). Runtime truth for quests, gold, loot, and combat lives on the **Godot client**. Go server is `/healthz` only. CI does not run Godot or quest smokes. **Do not treat DONE-E as EX-P07.**

| Focus area (owner ask) | Status | One-line finding |
|------------------------|--------|------------------|
| Local reward granting | **PARTIAL / risk** | Client `_complete` and monster `_die` grant XP/gold/items locally |
| Server authority | **MISSING** | No WS/quest/inventory APIs; no Postgres schema |
| Dialogue IDs | **PARTIAL** | Runtime uses `DIA.*` (matches `10_DIALOGUES.md`); `14` namespace says `DLG-*` |
| Quest objective re-count | **PARTIAL** | Session cap at `count`; no persist; no `reward_claim_id` / CT-011 |
| Godot 4.3 vs 4.7.2 | **BLOCKED** (upgrade) | Project pin `4.3`; integration cites `4.7.2` — do not upgrade without owner pin |
| CI coverage | **PARTIAL** | `go test` + file existence only; no Godot/mq01/content lint |

---

## 1. Inventory metrics (facts)

| Metric | Value | Evidence |
|--------|------:|----------|
| Canon MD (01–14 + README) | ~13 009 lines | `wc -l docs/content/*.md` |
| Client GDScript | ~1 192 LOC | `client/scripts/**/*.gd` |
| Server Go | 50 LOC | `server/main.go` + `main_test.go` |
| Client quests in runtime | 5 / 120 MQ | `client/data/quests/mq_01_01_to_05.json` |
| Side quests in runtime | 0 / 120 SQ | no `client/data` SQ files |
| Dialogue line keys loaded | 34 | `client/data/dialogues/mq_01_01_to_05.json` |
| Locations `.tscn` | 1 | `client/scenes/locations/dawn_valley.tscn` |
| Admin app | 0 | `admin/` absent |
| Content schemas / tools | 0 | `content/schemas`, `tools/content` absent |
| DB migrations | empty dir | `database/migrations/` has no SQL files |
| Deploy | stub README | `deploy/README.md` only |
| Automated CT-001…024 | 0 implemented in CI | `.github/workflows/ci.yml` |

---

## 2. Subsystem matrix (DONE / PARTIAL / MISSING / BLOCKED)

### 2.1 Documentation & process

| Subsystem | Status | Evidence |
|-----------|--------|----------|
| Canon 01–14 on `main` | **DONE** | `docs/content/01_WORLD.md` … `14_CONTENT_INTEGRATION.md` |
| `ROADMAP_EXECUTION.md` | **DONE** | EX-P00…P13 checklist |
| `AGENTS.md` + OSS research rule | **DONE** | `AGENTS.md`, `.cursor/rules/open-source-research.mdc` |
| Prior audits | **PARTIAL** | `docs/AUDIT.md` (stale Stage-0 narrative), `AUDIT_CONTENT_SYNC.md`, `ITERATION_MQ01_01_05.md` |
| `CONFLICTS.md` | **PARTIAL** | Created/updated this audit for DIA vs DLG; other risks still open |
| Procgen research + decision D | **DONE** | `docs/research/location-procgen-isometric.md` — generator deferred |

### 2.2 Client (Godot)

| Subsystem | Status | Evidence |
|-----------|--------|----------|
| Project boots, main scene | **DONE** | `client/project.godot` → `res://scenes/world/main.tscn` |
| Engine pin | **PARTIAL** | `config/features=… "4.3"`; canon/integration target **4.7.2** → see blockers |
| Dawn Valley visual slice | **DONE** | `dawn_valley.tscn`, sprites under `client/assets/`, World Studio plugin enabled |
| Player move / joystick / HUD | **DONE** | `player_controller.gd`, `virtual_joystick.gd`, `hud.tscn` + `hud_controller.gd` |
| Local combat (3 monsters) | **PARTIAL** | `slime/wolf/bandit.tscn` + `monster_actor.gd`; IDs not `ENM-01-*` |
| Local monster loot/gold | **PARTIAL / risk** | `monster_actor.gd` `_die` → `add_gold` / `add_loot` on killer (client) |
| NPC-802 / NPC-102 | **PARTIAL** | `feira_niss.tscn`, `ilra_venn.tscn` in scene; shared placeholder sheet `elder_mira_sheet.png`; leftover `elder_mira.tscn` unused in valley |
| MQ-01-01…05 local chain | **PARTIAL** | Data + `quest_manager.gd` + POIs; **not** server-authoritative |
| Quest UI / dialogue UI | **PARTIAL** | `dialogue.tscn` + `dialogue_ui.gd`; missing canon `OBJ1/OBJ2/OBJ3` lines in JSON |
| LOC-01-001…003 | **PARTIAL** | Metadata markers in one scene; not separate LOC `.tscn` |
| Classes CLS-01…05 | **MISSING** | Guardian stand-in only |
| Inventory ledger | **MISSING** | In-memory `inventory` dict on player |
| Networking to game server | **MISSING** | No WebSocket client usage found |
| Android APK | **BLOCKED** | `export_presets.cfg` Android preset; no templates/SDK in typical CI/VM |

### 2.3 Quest / dialogue / rewards (focus)

| Subsystem | Status | Evidence |
|-----------|--------|----------|
| Local accept / objectives / turn-in | **PARTIAL** | `quest_manager.gd` states `locked\|available\|active\|objectives_complete\|completed` |
| **Local reward grant** | **PARTIAL / critical risk** | `_complete()` adds `_xp`, `_rep_fac08`, `player.add_gold`, `player.add_loot` — **client is source of truth** |
| Reward idempotency ledger | **MISSING** | No `reward_claim_id`; no unique constraint; CT-011/013 not applicable yet |
| Objective re-count (session) | **PARTIAL** | `_try_event`: increments until `have >= need` then skips — **no over-count past need in-session** |
| Objective re-count (persist / reconnect) | **MISSING** | Progress only in RAM; restart resets quests |
| Double turn-in (same session) | **PARTIAL** | `_complete` requires `objectives_complete`; then status → `completed` / next quest — soft gate only |
| Server `objective_event` | **MISSING** | Events are local `npc_interact` / `world_interact` / `escort_checkpoint` |
| Dialogue runtime IDs | **PARTIAL** | Keys `DIA.MQ-01-0X.*` / `DIA.NPC.*` in JSON; match **`10_DIALOGUES.md` format** |
| Dialogue namespace in §14 | **CONFLICT** | `14_CONTENT_INTEGRATION.md` §4 lists `DLG-*`; file 10 uses `DIA.*` — see `docs/content/CONFLICTS.md` |
| Dialogue completeness MQ-01-01…05 | **PARTIAL** | Client has START/PROGRESS/TURNIN/DONE; **0** `OBJ1/OBJ2/OBJ3` lines (canon has ~30 for these five) |
| Server dialogue predicates | **MISSING** | Choices not filtered by server |

### 2.4 Server / data / admin

| Subsystem | Status | Evidence |
|-----------|--------|----------|
| HTTP `/healthz` | **DONE** | `server/main.go` loopback `:18086` |
| Go module | **DONE** | `go 1.22` in `go.mod` (aligned with CI 1.22.x) |
| Health unit test | **DONE** | `server/main_test.go` `TestHealthz` |
| WebSocket game protocol | **MISSING** | No handlers for §15 events |
| Quest state machine (Go) | **MISSING** | — |
| PostgreSQL + migrations | **MISSING** | `database/migrations/` empty; no Compose Postgres |
| Inventory / loot ledger | **MISSING** | — |
| Auth / sessions / rate limits | **MISSING** | — |
| Admin React/TS | **MISSING** | `admin/` absent |
| Content loader / manifest | **MISSING** | No `content.manifest.json` / schemas |

### 2.5 CI / tests / tools

| Subsystem | Status | Evidence |
|-----------|--------|----------|
| GitHub Actions workflow | **PARTIAL** | `.github/workflows/ci.yml` |
| Server `go test` + build | **DONE** (narrow) | Only healthz coverage |
| Required file existence checks | **PARTIAL** | Stage-1 paths + `docs/AUDIT.md` / `IMPLEMENTATION_PLAN.md` — **does not** check quest JSON, `AUDIT_BASELINE`, content 01–14 |
| Godot headless in CI | **MISSING** | No Godot install step |
| `mq01_smoke.gd` in CI | **MISSING** | Script exists locally; not invoked by workflow |
| `tools/smoke_stage1.sh` | **PARTIAL** | Boot + `go test` only; no mq01 |
| Content lint / CT-001…009 | **MISSING** | No parsers |
| CT-010…024 | **MISSING** | No server quest/dungeon/event tests |
| Android export in CI | **BLOCKED** | Toolchain absent |

### 2.6 Content volumes (canon vs code)

| Category | Canon | In code/runtime | Status |
|----------|------:|----------------:|--------|
| Regions | 6 | 1 visual (REG-01 markers) | **PARTIAL** |
| Factions | 8 | FAC-08 counter only | **PARTIAL** |
| LOC | 48 | 3 IDs as metadata | **PARTIAL** |
| NPC | 32 | 2 quest NPCs (+ unused elder_mira scene) | **PARTIAL** |
| Classes | 5 | 0 selectable | **MISSING** |
| Enemies | 96 | 3 placeholder monsters | **PARTIAL** |
| MQ | 120 | 5 local | **PARTIAL** |
| SQ | 120 | 0 | **MISSING** |
| DNG | 12 | 0 | **MISSING** |
| EVT | 18 | 0 | **MISSING** |

---

## 3. REG-01 matrix (canon contract vs facts)

| Contract (`14` §7) | Fact | Status |
|--------------------|------|--------|
| `LOC-01-001..008`, `POI-01-001..012` | One scene; custom POI ids (`POI-BOARD-01`, witnesses, etc.) not full `POI-01-*` registry | **PARTIAL** |
| `MQ-01-01..20` | Only 01–05 local JSON | **PARTIAL** |
| 3 server-confirmed objectives / quest | Client `_try_event` only | **MISSING** (server) |
| 20 `SQ-01-*` | None | **MISSING** |
| Regional NPC from 04 | NPC-802, NPC-102 present | **PARTIAL** |
| `ENM-01-*` | slime/wolf/bandit strings | **MISSING** |
| `DNG-01-001/002` | Absent | **MISSING** |
| `EVT-01-001..003` | Absent | **MISSING** |
| Equipment / `MAT-01-*` | Absent | **MISSING** |
| Test gate: path + 1 SQ + 1 EVT + 1 DNG + idempotent reward | Only local 5 MQ smoke | **MISSING** |

**Connectivity scenario status:** enter LOC → talk NPC → accept MQ-01-01 → progress → reward → next MQ works **locally**. No ledger, no DNG, no EVT, no SQ.

---

## 4. Deep dives (owner focus)

### 4.1 Local reward granting

```141:167:client/scripts/quest/quest_manager.gd
func _complete(player: Node) -> void:
	if _status != "objectives_complete":
		return
	# ...
	_xp += xp
	_rep_fac08 += rep
	if player and player.has_method("add_gold"):
		player.add_gold(gold)
	if reward.has("evidence"):
		_evidence.append(str(reward["evidence"]))
		if player and player.has_method("add_loot"):
			player.add_loot(str(reward["evidence"]), 1)
```

```78:84:client/scripts/actors/monster_actor.gd
func _die(killer: Node) -> void:
	# ...
	if killer and killer.has_method("add_gold"):
		killer.add_gold(randi_range(gold_min, gold_max))
	if killer and killer.has_method("add_loot") and randf() <= loot_chance:
		killer.add_loot(loot_item, 1)
```

**Verdict:** Any modified client can inflate gold/XP/loot. Explicit stub comments in `quest_manager.gd` L2–3 acknowledge this. Must move to Go + ledger before claiming online readiness (EX-P04 / EX-P08).

### 4.2 Server authority

```11:20:server/main.go
mux.HandleFunc("/healthz", ...)
log.Fatal(http.ListenAndServe("127.0.0.1:"+port, mux))
```

No `quest.*`, `dialogue.*`, `inventory.delta`, or WS. **MISSING** relative to `14` §15.

### 4.3 Dialogue IDs

| Source | Namespace |
|--------|-----------|
| `10_DIALOGUES.md` §2 | `DIA.<QUEST_ID>.<STAGE>.<INDEX>` |
| Client JSON / UI | `DIA.MQ-01-01.START.01`, … |
| `14_CONTENT_INTEGRATION.md` §4 table | `DLG-*` / nodes |
| `13_WORLD_EVENTS.md` cross-ref | mentions `DLG-*` |

**Runtime correctly follows file 10.** Integration shorthand `DLG-*` is a **doc conflict**, not a second client format. Tracked in `docs/content/CONFLICTS.md` (C-DIA-001).

Missing in client for MQ-01-01…05: all `DIA.MQ-01-0X.OBJ*` lines present in canon (~30).

### 4.4 Objective re-count / turn-in

```183:202:client/scripts/quest/quest_manager.gd
func _try_event(...):
	# ...
	if have >= need:
		continue
	_progress[oid] = have + 1
```

- **In-session:** progress capped at `need` (no overshoot past need).
- **Shared-target spam (PARTIAL):** progress keys by objective `target` string, not world instance. Repeating interact on **one** node with matching `interact_id` (e.g. single `POI-CLUE` / `POI-RESIDENT-GROUP`) can fill `count: 2|3` without visiting sibling markers in `dawn_valley.tscn`.
- **Not durable:** no save / server revision / `request_id` dedupe.
- **Turn-in:** status gate only; `QF-*-REWARD` set after grant, not used as ledger keys → fails CT-011/CT-013.
- `mq01_smoke.gd` drives events via API without proximity — fine for headless; proves client trust model.
- `QF-*-DONE` unlocks the chain only; not a reward idempotency key.

### 4.5 Godot 4.3 / 4.7.2

| Claim | Location |
|-------|----------|
| Project features **4.3** | `client/project.godot` L8 |
| Integration / architecture cite **4.7.2** | `14_CONTENT_INTEGRATION.md` header + §13.1 |

**BLOCKED:** do not bump Godot until owner pins installable binary and updates `config/features`. Risk to APIs, export templates, CI image.

### 4.6 CI coverage gap

`.github/workflows/ci.yml`:

1. `server`: `go test ./...` + `go build` (healthz only).
2. `client-scripts`: `test -f` for a few Stage-1 paths.

**Not covered:** Godot import/boot, `mq01_smoke.gd`, `stage1_smoke.gd`, presence of `docs/content/01…14`, quest/dialogue JSON validity, content schemas, Android, Postgres migrate, CT matrix.

Local `tools/smoke_stage1.sh` also skips mq01.

---

## 5. Blockers & priorities

### Blockers (`BLOCKED`)

| ID | Blocker | Action needed from owner |
|----|---------|--------------------------|
| B-GODOT | 4.3 pin vs 4.7.2 docs | Explicit pin: stay 4.3 or upgrade path |
| B-APK | No Android export templates in agent/CI VM | Provide toolchain or accept APK as out-of-scope until then |
| B-AUTH | Client-authoritative economy/quests | Required before any “online playable” claim (EX-P03–P04) |
| B-VPS | No Compose stack; isolation rule | Do not touch live VPS; design isolated `newmmo` stack later |

### Priority order after EX-P00 (recommendation only — wait for «делай»)

| Prio | Stage | Why |
|-----:|-------|-----|
| 1 | **EX-P01** | Parsers + schema so 13k lines of canon become machine-checkable |
| 2 | EX-P02 | ID registry / crossrefs / DIA alias / volume gates |
| 3 | EX-P03 | Postgres migrations + content loader |
| 4 | EX-P04 | Authoritative quest SM + reward ledger (kills local reward risk) |
| … | EX-P05… | Dialogue server → UI polish → REG-01 vertical (5 MQ server + 5 SQ) |

**Do not** start all 240 quests. **Do not** re-implement Stage 1 visual or local MQ-01-01…05 stub until replacing with server path.

---

## 6. Risks register (audit-time)

| ID | Risk | Severity | Mitigation path |
|----|------|----------|-----------------|
| R-REWARD | Client grants gold/XP/loot | Critical | EX-P04/P08 ledger |
| R-OBJ | Objective progress not durable / not authoritative | High | EX-P04 |
| R-OBJ-SPAM | Same `interact_id` node can satisfy `count>1` | Medium | Distinct targets or server instance IDs in EX-P04 |
| R-DIA | `DLG-*` vs `DIA.*` naming in docs | Medium | CONFLICTS + schema alias |
| R-OBJDIA | Missing OBJ dialogue lines in client | Low–Med | Content export EX-P01/P06 |
| R-GODOT | Version drift 4.3/4.7.2 | High (if ignored) | Owner pin |
| R-CI | False confidence from green CI | High | Add Godot + mq01 + content lint jobs |
| R-ENM | Monsters not canon IDs | Medium | Remap with 06 after combat authority |
| R-POI | Ad-hoc POI ids vs `POI-01-*` | Medium | Registry in EX-P02 |
| R-MIRA | Leftover `elder_mira.tscn` | Low | Delete or quarantine later |
| R-EMPTY-DB | Empty `database/migrations` looks like progress | Low | Document as placeholder |

---

## 7. What is explicitly **not** claimed done

- Authoritative MMO / multiplayer  
- PostgreSQL persistence  
- 5 SQ, DNG, EVT for REG-01  
- Full dialogue graph (OBJ stages, all NPC greetings)  
- Content validation CI  
- Android performance metrics  
- Admin RBAC  
- Any VPS deployment  

---

## 8. Acceptance for EX-P00

| Criterion | Met? |
|-----------|------|
| Table of subsystems with DONE/PARTIAL/MISSING/BLOCKED | Yes §2 |
| REG-01 matrix code vs canon | Yes §3 |
| File-path evidence for rewards, authority, dialogues, objectives, Godot, CI | Yes §4 |
| Blockers + priorities | Yes §5 |
| No new gameplay / no canon rewrite / no VPS | Yes |
| `ROADMAP_EXECUTION.md` EX-P00 marked complete | Yes (same commit) |

**Next stage proposal:** EX-P01 (content parsers + JSON Schema) — **await owner «делай»**.
