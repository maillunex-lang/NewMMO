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
