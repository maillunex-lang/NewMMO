# T-014 — Проверка целостности публикации контента

Проверено 2026-11-21. Строчный `BEFORE UPDATE` с условием `WHEN` подходит для проверки конкретного перехода: он читает строки сущностей до изменения статуса и может остановить SQL-операцию с `55000`. Функции оставляем `VOLATILE`, чтобы проверка могла видеть актуальные изменения, когда UPDATE дождался блокировки родительской строки. Действующий guard переходов и проверка указателя дополняют эту проверку. [CREATE TRIGGER](https://www.postgresql.org/docs/17/sql-createtrigger.html) · [Видимость в триггерах](https://www.postgresql.org/docs/17/trigger-datachanges.html) · [Блокировки](https://www.postgresql.org/docs/17/explicit-locking.html)

Порядок строк задаём внутри `string_agg`. Golden-вектор сверяет текст jsonb и разделители контракта: управляющие символы внутри JSON-строки остаются экранированными в `jsonb::text`. Для тестовой фикстуры Python независимо собирает ожидаемые байты и считает SHA-256. Сервер и триггер используют одну SQL-функцию. [Агрегаты](https://www.postgresql.org/docs/17/functions-aggregate.html) · [Типы JSON](https://www.postgresql.org/docs/17/datatype-json.html)

SQLSTATE проверяется через `pgconn.PgError` из закреплённого pgx v5.13.0. В Hysen Labs подтверждена карточка pgx. Поиск по `postgresql trigger` показал Graphile Worker, который решает задачу очередей; подходящего реестра контента в этой выдаче не обнаружено. Для hardening достаточно PostgreSQL, существующего pgx и стандартной библиотеки. [Документация pgx](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.13.0/pgconn#PgError) · [GitHub pgx](https://github.com/jackc/pgx) · [Каталог Hysen Labs](https://hysenlabs.com/projects/jackc-pgx)

## Независимый golden-вектор

У вектора две строки. В payload первой есть кириллица, U+001E, U+001F, кавычка, обратный слеш и перевод строки. Во второй есть табуляция и непустой регион. Независимый расчёт использует Python без подключения к БД. Для этих плоских объектов ключи упорядочены по длине UTF-8 и байтам, как в выводе PostgreSQL. Пробелы между полями и экранирование соответствуют `jsonb::text`.

```python
import hashlib
import json

rows = [
    ("TXT-02", "item", "REG-02", "02_TEST.md", 12, {"text": "Ключ\t«Луна»", "ok": True}),
    ("TXT-01", "dialogue", None, "01_TEST.md", 7, {"text": "Север\x1eюг\x1f\"\\\n", "n": 7}),
]

def jsonb_fixture_text(payload):
    keys = sorted(payload, key=lambda key: (len(key.encode("utf-8")), key.encode("utf-8")))
    return json.dumps({key: payload[key] for key in keys}, ensure_ascii=False)

wire = "\x1e".join(
    "\x1f".join((entity_id, kind, region or "", file, str(line), jsonb_fixture_text(payload)))
    for entity_id, kind, region, file, line, payload in sorted(rows, key=lambda row: (row[1], row[0]))
)
print(hashlib.sha256(wire.encode("utf-8")).hexdigest())
```

Результат Python и константа теста совпадают — `c5b22a62109e54e095a349d17539878090a821ade7fba6b883801423de2cbba5`. Интеграционный тест сравнивает константу с сохранённым и пересчитанным значениями. [Исходник вывода jsonb](https://github.com/postgres/postgres/blob/REL_17_STABLE/src/backend/utils/adt/jsonb.c) · [Порядок ключей](https://github.com/postgres/postgres/blob/REL_17_STABLE/src/backend/utils/adt/jsonb_util.c)
