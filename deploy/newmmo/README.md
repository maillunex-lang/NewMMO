# Локальный PostgreSQL для NewMMO

Compose создаёт проект `newmmo-dev` со своей сетью и томом. PostgreSQL 17 доступен на `127.0.0.1:15432` по умолчанию. При первом запуске пустого тома создаются роли `newmmo_migrator` для DDL, `newmmo_content_publisher` для публикации и `newmmo_app` для чтения.

Из корня репозитория создайте локальный `.env` из примера и задайте свои пароли.

```sh
cp deploy/newmmo/.env.example deploy/newmmo/.env
docker compose --env-file deploy/newmmo/.env -f deploy/newmmo/compose.yml up -d --build --wait
GOTOOLCHAIN=auto go install github.com/pressly/goose/v3/cmd/goose@v3.30.0
```

Перед миграцией задайте `NEWMMO_MIGRATE_DATABASE_URL` для роли `newmmo_migrator`. Для CLI задайте `NEWMMO_DATABASE_URL` для роли `newmmo_content_publisher`. Используйте локальный порт из `.env`, базу `newmmo` и `sslmode=disable`. В PostgreSQL URL специальные символы пароля должны быть percent-encoded. Храните URL в окружении или хранилище секретов.

```sh
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$NEWMMO_MIGRATE_DATABASE_URL" goose -dir server/migrations up

export_dir="$(mktemp -d)"
python3 tools/content/export.py --out "$export_dir"
cd server
go run ./cmd/contentctl verify "$export_dir"
go run ./cmd/contentctl stage "$export_dir"
go run ./cmd/contentctl status
go run ./cmd/contentctl publish '<build-id>'
```

Для последующей публикации используйте `publish '<new-id>' --expected-current '<active-id>'`. Команда `rollback '<previous-id>' --expected-current '<active-id>'` переключает указатель на ранее опубликованную сборку. В `status` отображаются активный и предыдущий ID, состояние и число сущностей каждой сборки.

Тесты ожидают отдельную тестовую БД с применёнными миграциями и экспорт из текущего канона. Передайте её URL через `NEWMMO_TEST_DATABASE_URL` и каталог экспорта через `NEWMMO_CONTENT_EXPORT_DIR`. Для проверки миграций на пустой локальной БД выполните `goose up`, `goose reset`, затем `goose up` с конфигурацией выше. `reset` удаляет таблицы и данные контента. `docker compose down` останавливает локальный стек и сохраняет том.

В CI скрипт инициализации можно выполнить через `bash deploy/newmmo/init/10-content-roles.sh` от локального администратора PostgreSQL. Передайте `POSTGRES_USER`, `POSTGRES_DB`, `PGHOST`, `PGPORT`, `PGPASSWORD` и три переменные паролей ролей. Скрипт выполняется один раз на чистой БД, затем goose под `newmmo_migrator` создаёт таблицы и выдаёт гранты.

Интеграционный прогон с проверкой отдельных ролей использует `NEWMMO_TEST_DATABASE_URL` для publisher, `NEWMMO_TEST_MIGRATE_DATABASE_URL` для владельца таблиц и `NEWMMO_TEST_APP_DATABASE_URL` для reader. Если два дополнительных URL опущены, основной тестовый URL должен принадлежать администратору, который может выполнить `SET ROLE newmmo_app` и менять staged-строку для теста целостности. Для теста первых публикаций у тестовой БД должен быть пустой указатель.

Запускайте goose одним заданием. Два прямых запуска CLI v3.30.0 на чистой БД могут столкнуться при создании таблицы версий. Локальная проверка дала один успешный запуск, один отказ с конфликтом в системном каталоге и чистую версию 4 с одной записью на миграцию. Повторный `up` подтверждает, что незавершённых миграций нет.

Входной бандл ограничен 64 MiB. Декодированный набор сущностей целиком хранится в памяти; замер пикового RSS отложен.

При отклонении операции CLI пытается записать аудит в течение 2 секунд. Если БД недоступна, CLI сохраняет исходную ошибку и пишет структурное сообщение о сбое аудита без DSN.

В CI `NEWMMO_REQUIRE_ROLE_URLS=1` требует все три URL и, перед тестами записи, сверяет роль каждого подключения через `current_user`.

Игровой HTTP-сервер ожидает `NEWMMO_DATABASE_URL` роли `newmmo_app`. Утилита `contentctl` использует отдельный URL издателя контента. Сначала примените миграции через goose, затем запустите `go run .` из каталога серверного модуля. `/readyz` проверяет доступность БД.

`NEWMMO_PUBLIC_WS_URL` задаёт URL для клиента. Локальное значение по умолчанию использует `127.0.0.1`, порт HTTP-сервера и путь `/v1/ws`. Значения `NEWMMO_LOGIN_LIMIT_PAIR` и `NEWMMO_LOGIN_LIMIT_IP` задают пределы неудачных входов за 15 минут (по умолчанию 5 и 20). `NEWMMO_REGISTRATION_LIMIT_IP` задаёт предел валидных запросов регистрации за час (по умолчанию 5). Лимиты используют IP TCP-соединения.

WebSocket принимает `newmmo.v1` по `/v1/ws` и одноразовый билет в первом сообщении `session.auth`. Перед подключением опубликуйте совместимую сборку контента через `contentctl`. Для доступа вне loopback клиенту нужен `wss://`. При завершении TLS на обратном прокси используйте loopback между прокси и HTTP-сервером. Лимиты используют адрес TCP-соединения, поэтому клиенты за одним прокси делят его IP-лимит.

| Переменная | По умолчанию |
|---|---:|
| `NEWMMO_WS_HANDSHAKES_PER_MINUTE` | 10 |
| `NEWMMO_WS_PREAUTH_PER_IP` | 3 |
| `NEWMMO_WS_COMMANDS_PER_SECOND` | 20 |
| `NEWMMO_WS_COMMAND_BURST` | 40 |
| `NEWMMO_WS_MUTATIONS_PER_SECOND` | 5 |
| `NEWMMO_WS_QUEUE_SIZE` | 256 |
| `NEWMMO_WS_RATE_VIOLATIONS` | 3 подряд |
| `NEWMMO_WS_PREAUTH_TIMEOUT` | `5s` |
| `NEWMMO_WS_HEARTBEAT_INTERVAL` | `25s` |
| `NEWMMO_WS_PONG_TIMEOUT` | `15s` |
| `NEWMMO_WS_WRITE_TIMEOUT` | `10s` |

На SIGTERM сервер прекращает принимать соединения и закрывает WebSocket с кодом 1001. На дренаж отведено до 10 секунд.
