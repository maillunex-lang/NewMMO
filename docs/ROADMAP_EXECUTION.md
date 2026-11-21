# ROADMAP_EXECUTION — исполнительная карта NewMMO

**Для:** любого ИИ-исполнителя / Cursor Agent  
**Репозиторий:** `maillunex-lang/NewMMO`  
**Ветка:** **только `main`** (коммит + push в `main`; отдельные PR/ветки — только по явной команде владельца)  
**Главный канон-оркестратор:** `docs/content/14_CONTENT_INTEGRATION.md`  
**Канон контента:** `docs/content/01_WORLD.md` … `14_CONTENT_INTEGRATION.md`  
**Процесс:** `AGENTS.md`, `.cursor/rules/open-source-research.mdc`  
**Обновлено:** 2026-11-21 · tip `main` — EX-P00 [x]; EX-P01 [x]; EX-P02 [x] (+C-SET-001, C-ITEMQ-001 resolved, client-sync, docs-links); **EX-DIR** (мульти-агентный протокол) [x]; EX-P03 next (ждёт ответов владельца и «делай»); EX-CONTENT-01 plan awaiting approval

---

## 0. Обязательный процесс каждого этапа

Перед стартом крупной подсистемы:

1. `git checkout main && git pull origin main`
2. Прочитать `14_CONTENT_INTEGRATION.md` + профильные MD этапа
3. **OSS research (обязательно перед крупной подсистемой):**  
   - GitHub (рабочий код + лицензия)  
   - **https://hysenlabs.com/projects** (каталог / поиск шаблонов и аналогов; проверить upstream)  
   - Godot Asset Library (если клиент)  
   - официальная документация  
   → сравнить ≥3 кандидатов, когда есть → `docs/research/<система>.md` → **ждать подтверждения** владельца **перед любой установкой/vendoring**
4. Реализовать минимальный срез; не трогать чужие VPS/сервисы
5. Тесты (unit/smoke/go test по критериям этапа)
6. Отметить этап `[x]` в **этом файле** (+ коммит SHA, дата, результат тестов)
7. Актуализировать связанные docs (`AUDIT*`, `docs/content/README.md`, итерационные отчёты)
8. `git commit` + `git push origin main`
9. Если работают несколько исполнителей — протокол `docs/director/DIRECTOR.md` (lanes, trailers `Lane:`/`Task:`, `git pull --rebase` перед push)
10. Короткий отчёт владельцу; **один** следующий этап — только после команды «делай»

**Запреты:** не переписывать канон 01–14 молча; конфликты → `docs/content/CONFLICTS.md`; не делать все 240 квестов за раз; не выдавать docs за готовую игру; клиент не авторитетен для боя/лута/валюты/квестов.

**Статусы чекбоксов:** `[ ]` не начато · `[~]` частично · `[x]` сделано и проверено · `[!]` блокер

---

## 1. Уже сделано (не переделывать)

| ID | Что | Доказательство | Статус |
|----|-----|----------------|--------|
| DONE-A | Baseline audits | `docs/AUDIT_BASELINE.md` (+ legacy `AUDIT.md`, `AUDIT_CONTENT_SYNC.md`) | [x] EX-P00 |
| DONE-B | Stage 1 visual slice | Dawn Valley `.tscn`, joystick/HUD, World Studio, procedural art | [x] `5846c1a` era / на `main` |
| DONE-C | Agent process rules | `AGENTS.md`, `.cursor/rules/open-source-research.mdc` | [x] |
| DONE-D | Location procgen research + decision D | `docs/research/location-procgen-isometric.md` — генератор отложен | [x] |
| DONE-E | MQ-01-01…05 local runtime | `client/data/quests/*`, `dialogues/*`, NPC-802/102, `mq01_smoke.gd` | [x] `11d48ca` — **локальный stub, не Go/Postgres** |
| DONE-F | Canon files 01–14 on main | `docs/content/*.md` | [x] через `efe1c7b` |

**Не считать DONE-E завершением P07 из §20 интеграции:** нет 5 SQ, нет серверных наград/идемпотентности, нет DNG/EVT.

---

## 2. Карта этапов (строгий порядок)

Этапы `EX-P*` зеркалят §20 `14_CONTENT_INTEGRATION.md` (P00–P13), с учётом фактического репозитория.

### EX-P00 — Read-only baseline audit
**Зависит от:** полный канон 01–14  
**Цель:** единый `docs/AUDIT_BASELINE.md` (пути, LOC строк, DONE/PARTIAL/MISSING/BLOCKED)  
**Не делать:** новый геймплей  
**Приёмка:**
- [x] Таблица всех подсистем client/server/admin/db/ci
- [x] Матрица REG-01: что уже в коде vs канон
- [x] Список блокеров (Godot 4.3 vs 4.7.2, нет Postgres, APK toolchain)
- [x] Обновлён этот ROADMAP  
**Тесты:** N/A (docs)  
**Статус:** [x] `docs/AUDIT_BASELINE.md` + `docs/content/CONFLICTS.md` (C-DIA-001)  
**Проверки:** read-only сверка дерева, `quest_manager`/`main.go`/`ci.yml`/`project.godot`, объёмы канона vs runtime

### EX-P01 — Content parsers + JSON Schema (MD → AST)
**Зависит от:** EX-P00  
**OSS research first:** schema/lint tooling (не ставить без подтверждения)  
**Артефакты:** `tools/content/`, `content/schemas/` (или адаптация под текущее дерево `client/data/`)  
**Приёмка:**
- [x] Профильный парсер по документам (не глобальный regex-как-истина) — `tools/content/profiles.py`
- [x] Отчёт validation; ERROR блокирует «publish» — `validation-report.json`, `content.manifest.json` (`publishable`), exit 1
- [x] CI job на lint content — `.github/workflows/ci.yml` → `content-lint`  
**Тесты:** CT-001…CT-002 из §19  
**OSS research:** [x] `docs/research/content-validation.md` (stdlib-only рекомендация; **ничего не установлено**)  
**Статус:** [x] `ea7f7e9` (2026-11-21) — Python stdlib, 0 зависимостей; 24 JSON Schema в `content/schemas/`; 16 unittest (CT-001/CT-002) OK; `lint.py` на каноне: 0 ERROR / 16 WARN → PUBLISHABLE (при allowlist C-SET-001)
**Не входит (→ EX-P02):** полное разрешение crossrefs, `crossrefs.json`, CT-003…009 (графы квестов, лут, диалоговые переходы)
**Найдено каноном:** C-SET-001 (15 ID `SET-*`) — позже RESOLVED в EX-P02

### EX-CONTENT-01 — Dialogue expansion (not new quests)
**Зависит от:** EX-P01 (lint) + owner approval of plan  
**План:** `docs/CONTENT_EXPANSION_PLAN.md`  
**Цель:** сохранить все 240 MQ/SQ ID; переписать `10_DIALOGUES.md` до 12–25 (MQ) / 8–15 (SQ) содержательных реплик; стадии START/PROGRESS/OBJ/TURNIN/DONE; `DIA.*` only  
**Не делать:** новые квесты; правки механик; массовый ренейм ID; раскрытие тайны Договора до гл. IV  
**Батчи:** pilot MQ-01-01…05 + SQ-01-001…005 → REG-01 MQ → REG-01 SQ → REG-02…  
**Приёмка:** см. §6 плана; без штампов-заглушек; state-gated текст  
**Статус:** [~] **план утверждён владельцем 2026-11-21**; аудит качества и руководство по стилю — T-016 ✔ (`docs/reviews/T-016-dialogue-audit.md`; линт `tools/content/dialogue_lint.py` — baseline 240/240 квестов с замечаниями; C-CHOICE-001 решён вариантом C: на MQ-01…05-20 ветвления нет); **пилот выполнен и перенесён в канон 2026-11-21 (T-017; MQ-01-05, SQ-01-001…005; канон `e69dd80`, клиент `987945f`, CI зелёный `9a7b7d3`; клиент пока без LATER и второго ASK — временное соответствие)**; полный объём 240 — не раньше успешного пилота

### EX-P02 — ID registry, crossrefs, manifest
**Зависит от:** EX-P01  
**Артефакты:** `crossrefs.json`, `content.manifest.json`, `validation-report.json`  
**Приёмка:**
- [x] объявления vs ссылки (4925 сущностей; все ссылки на hard-namespace резолвятся, 0 unresolved) — `crossrefs.json`
- [x] объёмы 6/8/32/5/96/120/120/12/18 — golden counts в `lint.py`
- [x] CONFLICTS при расхождениях — C-SET-001 RESOLVED, C-ITEMQ-001 OPEN
- [x] manifest: `content.manifest.json` (sha256 файлов, content_hash, sha256 `crossrefs.json`)
**Тесты:** CT-002…CT-009 (static)  
**Статус:** [x] `59f5cbd` + доводка (2026-11-21) — 35 unittest OK; `lint.py`: 0 ERROR / 1 WARN (`DLG-SCN-01-01`), allowlist пуст
**Сделано сверх реестра (после владельца «вариант A»):** C-ITEMQ-001 A — 36 `ITEM-Q-*` зарегистрированы в `11` §12.1 (`8a61c6b`), проверка «объявлено == выдаётся квестами», allowlist пуст; `check_client_sync.py` (client/data ↔ канон: 0 ERROR) и `check_docs_links.py` (0 битых путей) в CI.
**Сделано сверх реестра:** C-SET-001 A применён (`715558c`): 18 поселений `SET-RR-NN`→`STL-RR-NN` в `03`, строки реестра в `03`/`14`; наборы брони не тронуты; allowlist C-SET-001 удалён
**Частично (честно):** CT-009 — проверены связи loot↔enemy и вероятности 0..100, но не суммы весов/«бесконечная валюта» (нужны данные баланса, EX-P08); CT-006/007 — только ссылки, не граф комнат/расписания (EX-P09/P10)

### EX-DIR — Multi-agent protocol (AI-директор)
**Зависит от:** EX-P02  
**Цель:** безопасная параллельная работа Cursor / ChatGPT / Codex без взаимных поломок  
**Артефакты:** `docs/director/DIRECTOR.md`, `lanes.json`, `TASKS.md`, `prompts/_TEMPLATE.md`, `docs/contracts/`, `tools/director/lane_check.py`, CI job `lane-check`  
**Приёмка:**
- [x] Lanes (client / server / research / director) без пересечений; protected-пути только директору
- [x] Обязательные trailers `Lane:` / `Task:`; gate проверяет файлы коммита против lane (11 unittest)
- [x] Hard rules для исполнителей, цикл задач, формат отчёта, откат
- [x] Ответы владельца получены (все могут push; Godot/Go есть; lanes и режим git делегированы директору → branch-per-task `agent/<lane>/T-NNN`, PR/merge только директор)
- [x] Research-first (GitHub + Hysen Labs + Asset Library + docs) — правило №0 для всех исполнителей
**Статус:** [x] протокол/инструменты; **выданы T-001, T-004, T-005** (`docs/director/prompts/`), ждём отчётов

### EX-P03 — PostgreSQL migrations + content loader
**Зависит от:** EX-P02; backup/dry-run policy  
**OSS research:** migrate libs / sqlc / goose — отчёт (задача T-001, ChatGPT/research) → ждать ОК владельца
**Контракты до старта:** D-001 content export (director) в `docs/contracts/`
**OSS research:** [x] T-001 `docs/research/db-migrations.md` (merge `3fb9aa6`) — рекомендация: **goose + pgx/v5 (pgxpool)**, запасной golang-migrate; без ORM
**Проверка директором (go.mod на тегах upstream, 2026-11-21):** goose v3.30.0 → Go 1.26.0 (v3.26.0 → 1.23.0); golang-migrate v4.20.1 → 1.25.11; pgx v5.13.0 → 1.25.0 (v5.7.6 → 1.23.0); tern v2.4.3 → 1.25.0; sqlc v1.31.1 → 1.26.0. `server/go.mod` = Go 1.22, CI `setup-go 1.22.x`, локально Go 1.24.7 ⇒ **без апгрейда Go актуальные версии не подключить**
**Решения владельца (2026-11-21) — блокер снят:** (1) **Go сервера → 1.25+** (CI `setup-go 1.25.x`; директива `go 1.25.0` в `server/go.mod` — сделано в T-006); (2) **goose одобрен** — как закреплённый внешний CLI/образ в отдельном migration-job (не зависимость сервера) + **pgx/v5/pgxpool одобрен**; без ORM; sqlc — отдельное решение позже
**Условия Фазы 2 (обязательны до `go get`/скачивания):** точные версии и контрольные суммы фиксируются в `docs/contracts/` (pin-матрица), проверяются LICENSE/SPDX на теге, `go.mod`/дерево зависимостей, GHSA/открытые issues; кандидаты: pgx v5.13.0 (Go 1.25.0), goose v3.30.0 CLI (готовый бинарь/образ, не сборка из исходников); запасной golang-migrate v4.20.1  
**Приёмка:** миграции из §16 (адаптированные), rollback, content_builds loader  
**Тесты:** migrate up/down на локальном/Compose Postgres  
**Приёмка факты:** [x] миграции 00001–00005 (goose, up→reset→up, два параллельных запуска), [x] загрузчик (Stage/Publish/Rollback, `expected_current`, SQL-`ingest_digest`, триггер целостности), [x] `contentctl`, [x] изолированный Compose `deploy/newmmo/`, [x] CI `server-db` с живым PostgreSQL 17 без пропусков, [x] две независимые рецензии (T-010 контракты, T-013 код) учтены
**Статус:** [x] **EX-P03 закрыт 2026-11-21** — `3fb9aa6` (T-009) + T-014 (hardening); отложено в EX-P04: GC/TTL, FK игровых таблиц на `content_builds`, замер пикового RSS

### EX-P04 — Go quest state machine (authoritative)
**Зависит от:** EX-P03 ✔  
**Заменяет** клиентский stub как источник истины (клиент остаётся UI)  
**OSS research:** [x] T-003 `docs/research/go-websocket.md`; OK владельца на coder/websocket v1.8.15 + JSON получен; govulncheck v1.10.0 (официальный инструмент Go) закреплён  
**Контракты (директор) [x] 2026-11-21:** `docs/contracts/auth-session.md`, `db-gameplay.md`, `quest-engine.md`, `websocket-protocol.md` rev 1.1 (`world.interact.request`), реестр взаимодействий `content/interactables/reg-01.json` (D-002), `min_level` в экспорте квестов  
**Задачи:** T-018 (сервер: аккаунты, вход, персонажи, WS-билеты) · T-022 (клиент: экраны входа) → T-019 (WebSocket-сервер) → T-020 (автомат квестов, журнал наград, идемпотентность) → T-021 (рецензия безопасности) → T-023 (клиент подключается к настоящему серверу)  
**Приёмка:**
- [ ] аккаунты/вход/билеты по `auth-session.md` §8, без секретов в логах
- [ ] WS-сессия по D-003 (билет, heartbeat, лимиты, остановка)
- [ ] LOCKED→AVAILABLE→ACTIVE→READY_TO_TURN_IN→COMPLETED на сервере; accept/interact/turn-in идемпотентны
- [ ] награда ровно один раз (`reward_claims` UNIQUE + ledger), числа = канон `08` для MQ-01-01…05; сумма ledger = агрегаты
- [ ] конкурентный turn-in → одна награда; build pinning
- [ ] клиент проходит MQ-01-01…05 через сервер (T-023)  
**Известные интеримы (видны владельцу):** R-PROX — без серверной позиции проксимити не проверяется до EX-P08/P09; аккаунты слайса — не продовая идентичность (`auth-session.md` §7)  
**Тесты:** CT-010, CT-011, CT-013 + интеграционные (CI `server-db`)  
**Статус:** [~] запущен 2026-11-21: контракты [x]; T-022 [x] (`dc183e2`); T-018 [x] (`3e25b43`); T-019 промпт готов, T-019…T-021, T-023 в очереди

### EX-P05 — Go dialogue + choices
**Зависит от:** EX-P04  
**Приёмка:** серверная фильтрация опций; proximity/predicates; ветки из 10_DIALOGUES  
**Тесты:** CT-008, CT-014  
**Статус:** [ ]

### EX-P06 — Godot quest/dialogue UI polish (editable .tscn)
**Зависит от:** EX-P05 (или параллельно тонкий UI после P04 с mock)  
**Приёмка:** HUD/журнал/диалоги по Theme; landscape; без программной геометрии экранов  
**Тесты:** ручной + headless smoke  
**Статус:** [~] HUD/диалог + safe-area (T-004), журнал квестов + `tr()` (T-007) готовы на локальном stub; осталось: привязка к серверу (после P04/P05), переключатель локали, Godot 4.7.2 (по гейту)

### EX-P07 — Vertical slice REG-01 (играбельный контракт интеграции)
**Зависит от:** EX-P04…P06  
**Объём (не 240):**  
- [x] 5 MQ локально (DONE-E) — **перевести на сервер**  
- [ ] + довести MQ-01-01…05 на Go + persist  
- [ ] + 5 SQ из `09_QUESTS_SIDE.md` (только REG-01, выбранные по канону)  
- [ ] награды через ledger  
**Тесты:** CT-012, CT-018 (урезанный), `mq01_smoke` против API  
**Статус:** [~]

### EX-P08 — Inventory, loot, 5 classes
**Зависит от:** EX-P07; канон 05 + 11  
**OSS research:** inventory patterns — ждать ОК  
**Приёмка:** серверный инвентарь; `MAT-01-*` / `ITEM-Q-*` как QUEST_KEY; класс CLS-01…05 selectable  
**Тесты:** CT-009, CT-017  
**Статус:** [ ]

### EX-P09 — First dungeon `DNG-01-001`
**Зависит от:** EX-P08 (базовый бой/лут), `12_DUNGEONS.md`  
**OSS research:** Godot dungeon instance patterns — ждать ОК  
**Приёмка:** 7 комнат, solo, checkpoint, idempotent reward; editable `.tscn`  
**Тесты:** CT-006, CT-015  
**Статус:** [ ]

### EX-P10 — First public event `EVT-01-001`
**Зависит от:** EX-P07+; `13_WORLD_EVENTS.md`  
**Приёмка:** schedule/phases, contribution anti-AFK, one reward per instance  
**Тесты:** CT-007, CT-016  
**Статус:** [ ]

### EX-P11 — Full REG-01 + start REG-02
**Зависит от:** EX-P07…P10  
**Объём:** MQ-01-06…20, остальные SQ-01, затем вход в REG-02 (не все 40 сразу в одном коммите — дробить)  
**Приёмка:** матрица §7 интеграции; регрессия  
**Статус:** [ ]

### EX-P12 — REG-03…06 + 12 DNG + 18 EVT
**Зависит от:** EX-P11 играбелен  
**Дробить** по регионам; не монолит  
**Статус:** [ ]

### EX-P13 — Endings, admin, hardening, Android RC
**Зависит от:** EX-P12 / достаточный контент  
**Приёмка:**
- [ ] **Отложенный долг (R-DEVICE):** замеры на реальном устройстве 6 ГБ (CT-020) — без них релиз/RC и апгрейд движка не принимаются END-01…03 персональны; admin RBAC; Android 6GB metrics; rollback  
**Тесты:** CT-019…CT-024  
**Статус:** [ ]

---

## 3. Региональный backlog (после серверного фундамента)

| Регион | MQ | SQ | DNG | EVT | Статус |
|--------|----|----|-----|-----|--------|
| REG-01 | 20 | 20 | 2 | 3 | [~] MQ 1–5 local only |
| REG-02 | 20 | 20 | 2 | 3 | [ ] |
| REG-03 | 20 | 20 | 2 | 3 | [ ] |
| REG-04 | 20 | 20 | 2 | 3 | [ ] |
| REG-05 | 20 | 20 | 2 | 3 | [ ] |
| REG-06 | 20 | 20 | 2 | 3 | [ ] |

---

## 4. Журнал выполненных этапов (заполнять при закрытии)

| Дата | Этап | Commit | Тесты | Заметка |
|------|------|--------|-------|---------|
| 2026-11-21 | DONE-B Stage1 visual | `5846c1a`+ | Godot smoke | polygons→sprites |
| 2026-11-21 | DONE-C/D process+research | `84ccea4`/`7f817f9` | — | procgen = D |
| 2026-11-21 | DONE-E MQ-01-01…05 local | `11d48ca` | `MQ01_SMOKE_OK` | не сервер |
| 2026-11-21 | Canon 01–14 present | `efe1c7b` | — | integration+events |
| 2026-11-21 | EX-P00 baseline audit | `7567a54` | docs-only | local rewards + authority gaps documented |
| 2026-11-21 | EX-CONTENT-01 plan (draft) | `ad45425` | — | ждёт утверждения владельца |
| 2026-11-21 | EX-P01 research only | `663ecc5` | — | content-validation.md; no deps installed |
| 2026-11-21 | EX-P01 parsers + schema + CI | `ea7f7e9` | unittest 16 OK; lint 0 ERROR | stdlib only; C-SET-001 найден, не правился канон |
| 2026-11-21 | EX-P02 C-SET-001 STL rename | `715558c` | unittest 18 OK | одноразовая правка `03` по approve владельца |
| 2026-11-21 | EX-P02 crossrefs + CT-003…009 | `59f5cbd` | unittest 32 OK; lint 0 ERROR | C-ITEMQ-001 найден (36 ITEM-Q без объявления) |
| 2026-11-21 | C-ITEMQ-001 A (по решению владельца) | `8a61c6b` | unittest 32 OK | 36 `ITEM-Q-*` в `11` §12.1, имена — рабочие |
| 2026-11-21 | Полная сверка связей: client-sync + docs-links + CI | см. `git log` | unittest 35 OK; client-sync 0 ERR; docs-links 0 | клиент == канон; 1 doc-путь починен (`AUDIT.md`) |
| 2026-11-21 | T-015 переход на Godot 4.7.2 (Cursor) | см. `git log` | executor: strict smoke на 4.7.2 зелёный, кадры до/после совпали, APK собран; CI `client-godot` | пин = 4.7.2; Android minSdk 24/targetSdk 36; GUI-проверка World Studio не сделана; замеры устройства — гейт перед релизом |
| 2026-11-21 | T-014 EX-P03 hardening (Codex) | см. `git log` | CI на ветке зелёный (server-db, живой PG 17) | EX-P03 закрыт |
| 2026-11-21 | T-012 Android debug APK baseline, Godot 4.3 (Cursor) | см. `git log` | executor: build_debug_apk OK, apksigner OK, smoke OK; device unavailable | B-APK: сборка воспроизводима (скрипты + CI workflow `android-debug`); замеры на телефоне 6 ГБ — за владельцем (`tools/android/measure_device.sh`) |
| 2026-11-21 | T-011 client WebSocket layer per D-003 (Cursor) | см. `git log` | executor: smoke OK incl. NET_SMOKE_OK; artificial fail → exit 1 | клиентская часть EX-P04/P06 готова на FakeTransport; живого сервера ещё нет |
| 2026-11-21 | T-008 strict smoke + JSON in export filter (Cursor) | см. `git log` | executor: smoke OK, artificial failures fail, PCK contains data JSON | закрыты пробелы смоука и экспорта из T-002; Android APK/keystore dry-run — не сделан (B-APK) |
| 2026-11-21 | T-007 quest journal + tr() strings (Cursor) | см. `git log` | smokes (executor) identical; client-sync 0 err; static node-path check | EX-P06: журнал за read-only адаптером (готов к замене серверным источником); `ui.csv` |
| 2026-11-21 | T-006 Go 1.25 directive + WS-ready wrapper (Codex) | см. `git log` | director re-run on Go 1.25.0: vet + test -race OK | R-GO закрыт на уровне `server/go.mod`; CI уже 1.25.x |
| 2026-11-21 | T-002 Godot 4.7.2 gate research (ChatGPT) | см. `git log` | docs-only; director spot-check of smoke/export claims | 4.7.2 вышел 2026-08-18; рекомендация: держать 4.3 до B-APK + базы на 6 ГБ; найдены пробелы смоука/экспорта → T-008 |
| 2026-11-21 | T-004 client UI safe-area + scene choices (Cursor) | см. `git log` | smokes (executor): stage1/MQ01/boot OK; client-sync 0 err | в P06 сделано: safe-area, кнопки выбора в `.tscn`; журнал квестов и i18n → T-007 |
| 2026-11-21 | T-005 Go server hardening (Codex) | `b026eab` | go vet + test -race OK; live smoke OK | stdlib only; replaced by T-006 follow-up (go 1.25 directive, WS-ready wrapper) |
| 2026-11-21 | EX-DIR multi-agent protocol | см. `git log` | unittest 11 OK (lane_check) | задачи не выданы, ждём ответов владельца |

---

## 5. Известные риски (из §23 + репо)

| ID | Риск | Действие |
|----|------|----------|
| R-GO | Go 1.22 vs актуальные pgx/goose; затем уязвимости stdlib | **RESOLVED**: Go ≥1.25 одобрен; `govulncheck` (2026-11-21) нашёл 9 достижимых уязвимостей stdlib в Go 1.25.14 → сервер на **Go 1.26.11** (`go 1.26.0` + `toolchain go1.26.11`, CI 1.26.x), на нём чисто |
| R-GODOT | ~~Target 4.7.2; pin 4.3~~ **RESOLVED 2026-11-21: проект на 4.7.2** (T-015) | **Решение владельца 2026-11-21: переходим на 4.7.2 сейчас** (T-015, гейт `docs/research/godot-4-7-2-upgrade.md`); пин на `main` меняется после приёмки T-015; замеры на устройстве — гейт перед релизом (R-DEVICE) |
| R-AUTH | Квесты на клиенте | EX-P04 обязателен до «готово онлайн» |
| R-DIA | `DLG-*` в 13/14 | **RESOLVED A** — канон только `DIA.*`; `DLG-*` obsolete |
| R-POI | Клиентские stub POI ≠ `POI-01-*` | OPEN (C-POI-001) → задача D-002 (контракт маппинга) |
| R-MULTI | 3 исполнителя ломают друг друга | EX-DIR: lanes + trailers + CI `lane-check` + rebase-before-push; откат директором |
| R-NAMES | Названия `ITEM-Q-*` рабочие (от названий квестов) | Владелец утверждает финальные имена; не блокер для P03 |
| R-EQ | `EQ-CC-RR-SS-RR` | Typed fields, не слепой regex |
| R-APK | Нет Android templates в CI VM | Честно блокировать APK stage; **T-012** (2026-11-21): локальная debug-сборка на 4.3 + CI `android-debug`, инструментарий одобрен владельцем |
| R-PACE | Игру можно пройти слишком быстро | **Решение владельца 2026-11-21:** запуск по регионам (новый каждые ~5 дней), 60 уровень ≈ за месяц, rested XP разрешён — `docs/PACING.md`; региональные ворота и параметры XP — серверный конфиг, подключаются в EX-P04 (T-020: `REGION_CLOSED`) и EX-P08/P09 |
| R-DEVICE | Нет устройства 6 ГБ для замеров CT-020 | **ПРИНЯТО владельцем 2026-11-21: временно считаем базу Android «ок» без цифр; замеры (`tools/android/measure_device.sh`, 3×3: старт, FPS/p95, peak PSS, падения) — ОБЯЗАТЕЛЬНЫЙ гейт перед релизом/Android RC (EX-P13) и перед апгрейдом на Godot 4.7.2.** До тех пор цифры производительности не заявляются |
| R-VPS | 8GB / 18 events | Caps + staged rollout |

---

## 6. Следующий этап (предложение)

**EX-P00, EX-P01, EX-P02, EX-DIR закрыты.** Что дальше:

1. ~~Владелец отвечает на вопросы~~ — получены.
2. **Первый параллельный батч (выдан, ждём отчётов)** (разные lanes, без зависимостей): T-001 research БД-миграций (ChatGPT) · T-004 compliance-ревью клиентского UI (Cursor) · T-005 Go-скелет без зависимостей (Codex). Параллельно директор: D-001 (content export contract), D-002 (POI mapping).
3. **EX-P03** (миграции + loader) — OK владельца получен (goose + pgx, Go ≥1.25). Первый шаг директора: D-001 (контракт экспорта + pin-матрица + имена миграций), затем задача серверной lane. Стартует по команде **«делай»**.
4. **EX-CONTENT-01** — только после утверждения `docs/CONTENT_EXPANSION_PLAN.md`.

Не начинать P03+ без команды **«делай»**.
