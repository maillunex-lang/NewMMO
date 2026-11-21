# ROADMAP_EXECUTION — исполнительная карта NewMMO

**Для:** любого ИИ-исполнителя / Cursor Agent  
**Репозиторий:** `maillunex-lang/NewMMO`  
**Ветка:** **только `main`** (коммит + push в `main`; отдельные PR/ветки — только по явной команде владельца)  
**Главный канон-оркестратор:** `docs/content/14_CONTENT_INTEGRATION.md`  
**Канон контента:** `docs/content/01_WORLD.md` … `14_CONTENT_INTEGRATION.md`  
**Процесс:** `AGENTS.md`, `.cursor/rules/open-source-research.mdc`  
**Обновлено:** 2026-11-21 · `main` @ post-`efe1c7b`

---

## 0. Обязательный процесс каждого этапа

Перед стартом крупной подсистемы:

1. `git checkout main && git pull origin main`
2. Прочитать `14_CONTENT_INTEGRATION.md` + профильные MD этапа
3. **OSS research** (GitHub + https://hysenlabs.com/projects + Asset Library + docs) → `docs/research/<система>.md` → **ждать подтверждения** владельца перед установкой
4. Реализовать минимальный срез; не трогать чужие VPS/сервисы
5. Тесты (unit/smoke/go test по критериям этапа)
6. Отметить этап `[x]` в **этом файле** (+ коммит SHA, дата, результат тестов)
7. Актуализировать связанные docs (`AUDIT*`, `docs/content/README.md`, итерационные отчёты)
8. `git commit` + `git push origin main`
9. Короткий отчёт владельцу; **один** следующий этап — только после команды «делай»

**Запреты:** не переписывать канон 01–14 молча; конфликты → `docs/content/CONFLICTS.md`; не делать все 240 квестов за раз; не выдавать docs за готовую игру; клиент не авторитетен для боя/лута/валюты/квестов.

**Статусы чекбоксов:** `[ ]` не начато · `[~]` частично · `[x]` сделано и проверено · `[!]` блокер

---

## 1. Уже сделано (не переделывать)

| ID | Что | Доказательство | Статус |
|----|-----|----------------|--------|
| DONE-A | Read-ish audits | `docs/AUDIT.md`, `docs/AUDIT_CONTENT_SYNC.md` | [~] нужна формализация `AUDIT_BASELINE.md` (P00) |
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
- [ ] Таблица всех подсистем client/server/admin/db/ci
- [ ] Матрица REG-01: что уже в коде vs канон
- [ ] Список блокеров (Godot 4.3 vs 4.7.2, нет Postgres, APK toolchain)
- [ ] Обновлён этот ROADMAP  
**Тесты:** N/A (docs)  
**Статус:** [~] частично закрыто `AUDIT.md` / `AUDIT_CONTENT_SYNC.md` — **следующий рекомендуемый шаг**

### EX-P01 — Content parsers + JSON Schema (MD → AST)
**Зависит от:** EX-P00  
**OSS research first:** schema/lint tooling (не ставить без подтверждения)  
**Артефакты:** `tools/content/`, `content/schemas/` (или адаптация под текущее дерево `client/data/`)  
**Приёмка:**
- [ ] Профильный парсер по документам (не глобальный regex-как-истина)
- [ ] Отчёт validation; ERROR блокирует «publish»
- [ ] CI job на lint content  
**Тесты:** CT-001…CT-002 из §19  
**Статус:** [ ]

### EX-P02 — ID registry, crossrefs, manifest
**Зависит от:** EX-P01  
**Артефакты:** `crossrefs.json`, `content.manifest.json`, `validation-report.json`  
**Приёмка:** объявления vs ссылки; объёмы 6/8/32/5/96/120/120/12/18; CONFLICTS при расхождениях  
**Тесты:** CT-002…CT-009 (static)  
**Статус:** [ ]

### EX-P03 — PostgreSQL migrations + content loader
**Зависит от:** EX-P02; backup/dry-run policy  
**OSS research:** migrate libs / sqlc / goose — отчёт → ждать ОК  
**Приёмка:** миграции из §16 (адаптированные), rollback, content_builds loader  
**Тесты:** migrate up/down на локальном/Compose Postgres  
**Статус:** [ ]

### EX-P04 — Go quest state machine (authoritative)
**Зависит от:** EX-P03  
**Заменяет** клиентский stub как источник истины (клиент остаётся UI)  
**Приёмка:** LOCKED→…→COMPLETED; idempotent accept/turn-in; `reward_claims` unique  
**Тесты:** CT-010, CT-011, CT-013  
**Статус:** [ ]

### EX-P05 — Go dialogue + choices
**Зависит от:** EX-P04  
**Приёмка:** серверная фильтрация опций; proximity/predicates; ветки из 10_DIALOGUES  
**Тесты:** CT-008, CT-014  
**Статус:** [ ]

### EX-P06 — Godot quest/dialogue UI polish (editable .tscn)
**Зависит от:** EX-P05 (или параллельно тонкий UI после P04 с mock)  
**Приёмка:** HUD/журнал/диалоги по Theme; landscape; без программной геометрии экранов  
**Тесты:** ручной + headless smoke  
**Статус:** [~] базовый HUD/диалог есть; нужна привязка к серверу

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
**Приёмка:** END-01…03 персональны; admin RBAC; Android 6GB metrics; rollback  
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
| | EX-P00 | | | *ожидает* |

---

## 5. Известные риски (из §23 + репо)

| ID | Риск | Действие |
|----|------|----------|
| R-GODOT | Канон 4.7.2 vs проект 4.3 | Не апгрейдить без пина владельца |
| R-AUTH | Квесты на клиенте | EX-P04 обязателен до «готово онлайн» |
| R-DIA | `DIA.*` в 10 vs `DLG-*` в §14 | Зафиксировать в CONFLICTS/схеме как alias |
| R-EQ | `EQ-CC-RR-SS-RR` | Typed fields, не слепой regex |
| R-APK | Нет Android templates в CI VM | Честно блокировать APK stage |
| R-VPS | 8GB / 18 events | Caps + staged rollout |

---

## 6. Следующий этап (предложение)

**Рекомендация:** выполнить **EX-P00** — создать `docs/AUDIT_BASELINE.md` (read-only сводка репозитория под чеклист §19/§20), отметить `[x]` здесь, затем остановиться.

После команды владельца **«делай»** — только этот этап (без P01+).

Альтернатива по явной команде: сразу **EX-P01** (валидатор контента), если baseline считать достаточным.
