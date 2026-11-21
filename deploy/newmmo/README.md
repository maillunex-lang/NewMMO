# Локальный PostgreSQL для NewMMO

Compose создаёт проект `newmmo-dev` со своей сетью и томом. PostgreSQL 17 доступен на `127.0.0.1:15432` по умолчанию. При первом запуске пустого тома создаются роли `newmmo_migrator` для DDL и `newmmo_app` для работы с данными.

Из корня репозитория создайте локальный `.env` из примера и задайте свои пароли.

```sh
cp deploy/newmmo/.env.example deploy/newmmo/.env
docker compose --env-file deploy/newmmo/.env -f deploy/newmmo/compose.yml up -d --build --wait
GOTOOLCHAIN=auto go install github.com/pressly/goose/v3/cmd/goose@v3.30.0
```

Перед миграцией задайте `NEWMMO_MIGRATE_DATABASE_URL` для роли `newmmo_migrator`. Для CLI задайте `NEWMMO_DATABASE_URL` для роли `newmmo_app`. Используйте локальный порт из `.env`, базу `newmmo` и `sslmode=disable`. В PostgreSQL URL специальные символы пароля должны быть percent-encoded. Храните URL в окружении или хранилище секретов.

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

Для последующей публикации используйте `publish '<new-id>' --expected-previous '<active-id>'`. Команда `rollback '<previous-id>'` переключает указатель на ранее опубликованную сборку. В `status` отображаются активный и предыдущий ID, состояние и число сущностей каждой сборки.

Тесты ожидают отдельную тестовую БД с применёнными миграциями и экспорт из текущего канона. Передайте её URL через `NEWMMO_TEST_DATABASE_URL` и каталог экспорта через `NEWMMO_CONTENT_EXPORT_DIR`. Для проверки миграций на пустой локальной БД выполните `goose up`, `goose reset`, затем `goose up` с конфигурацией выше. `reset` удаляет таблицы и данные контента. `docker compose down` останавливает локальный стек и сохраняет том.
