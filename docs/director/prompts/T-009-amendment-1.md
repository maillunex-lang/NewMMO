# T-009 — поправка №1 (контракты обновлены по рецензии T-010)

Это дополнение к **T-009** (твоя текущая задача, ветка `agent/server/T-009`). Контракты в `docs/contracts/` обновлены до **rev 1.1** после независимой рецензии. Выполни поправку **в той же ветке**, до сдачи отчёта.

## Что сделать сначала
`git fetch origin && git rebase origin/main` (твои коммиты — на свежий `main`; **только** свою ветку), затем перечитай целиком:
- `docs/contracts/content-export.md` (rev 1.1)
- `docs/contracts/db-content.md` (rev 1.1)
- `docs/reviews/T-010-contracts-review.md` (замечания R-01…R-13, для контекста)
Чтобы получить актуальный бандл: `python3 tools/content/export.py --out <dir>` (формат изменился: см. ниже).

## Что изменилось и что это значит для кода
1. **Формат бандла.** `manifest.json` теперь — каноническая **однострочная** JSON-строка + LF; `build_id = sha256(сырых байтов manifest.json)`; рядом файл `BUILD_ID` (64 hex + LF). В самом manifest **нет** полей `manifest_sha256`/`build_id`. Загрузчик **не пересериализует** manifest: хеширует сырые байты, сравнивает с `BUILD_ID`, потом парсит.
2. **Строгая загрузка** (`content-export.md` §4): UTF-8 без BOM, запрет дубликатов ключей JSON, NaN/Infinity, обязательная сортировка `(kind,id)`, LF-only, лимиты (бандл ≤ 64 MiB, строка ≤ 1 MiB, вложенность ≤ 16, ≤ 100 000 сущностей), потоковое чтение. Тесты на каждый случай.
3. **Схема БД** (`db-content.md` §2): `content_active` **сидируется миграцией** (`id=1`, `build_id NULL`); новые поля `ingest_digest` (билды), `line_sha256` (сущности); хеши — `text` + `CHECK`; `jsonb_typeof(...)='object'`; триггеры: переходы статусов и неизменяемые колонки `content_builds`, запрет `TRUNCATE`, защита указателя (только на `published`), append-only `content_audit`; **роли** `newmmo_migrator` / `newmmo_content_publisher` / `newmmo_app` (SELECT-only). В Compose/CI создание ролей — отдельным init-скриптом, если миграционная роль не имеет `CREATEROLE` (опиши в README).
4. **Операции** (`db-content.md` §3): порядок блокировок «указатель → билды по возрастанию `build_id`»; `Publish` и `Rollback` принимают `expected_current` (NULL = ничего не опубликовано) и возвращают `CONFLICT` при несовпадении; ретраи на `40P01`/`40001` (до 3, с паузой); `Stage` берёт `pg_advisory_xact_lock(hashtext(build_id))`, пересчитывает `ingest_digest` для уже существующего staged-билда; `Publish` пересчитывает `ingest_digest` и счётчики **из строк БД** и сверяет; стабильные коды ошибок (`BUNDLE_INVALID`, `HASH_MISMATCH`, `CONFLICT`, …); best-effort audit `reject`.
5. **Приёмка расширена** (`db-content.md` §6): две одновременные ПЕРВЫЕ публикации → ровно одна успешна, вторая `CONFLICT`; publish vs rollback и stage vs publish без взаимоблокировок; прямой DML от `newmmo_app` запрещён; обнаружение подмены staged-строк; два одновременных запуска goose не портят схему.
6. **COPY vs INSERT**: сделай небольшой бенчмарк (4925 строк) и запиши результат и выбор в `docs/research/T-009-loader-patterns.md`.

## Не делать
Всё прежнее из T-009 остаётся: не трогать контракты (`docs/contracts/**`), `tools/**`, `.github/**`, канон; новые зависимости — только pgx из `toolchain-pins`. Если контракт кажется противоречивым или неполным — не додумывай, вопрос в отчёт.

## Отчёт
Тот же формат, что в T-009, плюс отдельный список «как закрыто каждое замечание R-01…R-13» (одна строка на пункт, со ссылкой на файл/тест).

Коммиты по-прежнему заканчиваются:
```
Lane: server
Task: T-009
```
