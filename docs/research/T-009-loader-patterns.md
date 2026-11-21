# T-009 — Загрузка и переключение сборки контента

Проверено 2026-11-21. D-001 задаёт формат бандла, схему и транзакции, а T-001 и toolchain-pins фиксируют goose v3.30.0, pgx v5.13.0 и PostgreSQL 17. В каталоге Hysen Labs проверены карточки pgx, pgxmock и Testcontainers. Метаданные сверены с GitHub. [Каталог](https://hysenlabs.com/projects) · [pgx](https://hysenlabs.com/projects/jackc-pgx) · [pgxmock](https://hysenlabs.com/projects/pashagolub-pgxmock) · [Testcontainers](https://hysenlabs.com/projects/testcontainers-testcontainers-go)

| Подход | Оценка для T-009 |
|---|---|
| Проверка stdlib, одна транзакция `pgxpool` и `CopyFrom` | Выбран. pgx MIT, последний push 2026-11-16. Проверить снимок до запросов к БД, передать payload как `json.RawMessage` в кодек jsonb, пересчитать строки и виды в SQL и записать аудит в той же транзакции. Для ошибки нужен отдельный аудит после отката. [pgx v5.13.0](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.13.0) · [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.13.0/pgxpool) · [PostgreSQL JSON](https://www.postgresql.org/docs/17/datatype-json.html) |
| Пакетные INSERT или отдельная временная таблица и перенос | Теми же одобренными pgx и PostgreSQL, без новых модулей. Подход уместен при SQL-преобразованиях или объединении данных. Проверенный идентифицируемый снимок записывается напрямую, поэтому дополнительный проход здесь лишний. [PostgreSQL COPY](https://www.postgresql.org/docs/17/sql-copy.html) · [pgx на GitHub](https://github.com/jackc/pgx) |
| Версии строк и указатель в БД либо внешняя публикация с переключением в CDN | Выбран указатель в БД. В rev 1.1 миграция заранее создаёт строку указателя. `FOR UPDATE` сериализует и первую, и следующие операции. Публикация и откат сравнивают указатель с ожидаемым значением. Откат использует тот же указатель. Внешний указатель потребовал бы согласования между системами и не давал бы общей транзакции с аудитом. [Блокировки](https://www.postgresql.org/docs/17/explicit-locking.html) · [Read Committed](https://www.postgresql.org/docs/17/transaction-iso.html) |
| Изолированный PostgreSQL из Compose/CI либо Go-библиотека для тестового окружения | Выбран настоящий PostgreSQL 17 по URL из окружения. Testcontainers Go (MIT, push 2026-11-21) удобен для управления контейнером из теста, но расширяет граф модулей и требует Docker от тестового процесса. Мок pgx не проверит блокировки, триггеры, goose и COPY. [Testcontainers на GitHub](https://github.com/testcontainers/testcontainers-go) · [pgxmock на GitHub](https://github.com/pashagolub/pgxmock) · [goose](https://github.com/pressly/goose/tree/v3.30.0) |

Директиву Go и версии инструментов берём из toolchain-pins. Единственная прямая зависимость сервера — pgx. Goose запускается отдельно от игрового процесса. CLI `verify` остаётся офлайн, а `stage` записывает `reject` для отклонённого бандла, когда БД доступна. Rev 1.1 определяет пустой `expected_current` как отсутствие активной сборки.

## Целостность сохранённых строк

Решение директора по R-04 от 2026-11-21 закрепляет два отдельных хеша. Loader сверяет SHA-256 исходных байтов JSONL до записи. PostgreSQL затем вычисляет `ingest_digest` из сохранённых полей в стабильном порядке, включая `payload::text`. Хеш строки от клиента в БД не хранится. Тест меняет один payload при прежнем числе строк, получает `HASH_MISMATCH` на Publish и проверяет восстановление через повторный Stage. [Контракт](../contracts/db-content.md)

Для новой сборки Stage откладывает проверку внешнего ключа до commit. Сущности загружаются первыми, затем SQL вычисляет отпечаток, после чего регистрируется родитель уже с неизменяемым `ingest_digest`. COPY, запись родителя, проверка счётчиков и аудит принадлежат одной транзакции.

## COPY против пакетного INSERT

Локальный PostgreSQL 17.12 в Compose, Go 1.25.14, Linux amd64. В каждом прогоне 4 925 строк из реального экспорта. Сравнивались binary COPY и пакетный INSERT по 250 строк через pgx Batch. Оба метода записывали данные в одинаковую временную таблицу с индексами и CHECK, затем транзакция откатывалась. Замер изолирует путь вставки и не включает верификацию бандла, SQL-отпечаток и аудит.

| Метод | 3 прогона по 5 итераций, мс на 4 925 строк | Медиана | Строк/с по медиане |
|---|---:|---:|---:|
| `CopyFrom` | 53,10 / 46,77 / 45,95 | 46,77 мс | 105 306 |
| INSERT по 250 | 88,93 / 87,06 / 79,81 | 87,06 мс | 56 571 |

COPY примерно в 1,86 раза быстрее по медиане, поэтому Stage использует `CopyFrom`.

```sh
cd server
go test ./internal/content -run '^$' -bench '^BenchmarkRegistryIngest$' -benchtime=5x -count=3
```
