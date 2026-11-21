#!/usr/bin/env bash

(
    set -euo pipefail

    psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1 <<'SQL'
\getenv migrator_password NEWMMO_MIGRATOR_PASSWORD
\getenv publisher_password NEWMMO_PUBLISHER_PASSWORD
\getenv app_password NEWMMO_APP_PASSWORD

CREATE ROLE newmmo_migrator LOGIN PASSWORD :'migrator_password';
CREATE ROLE newmmo_content_publisher LOGIN PASSWORD :'publisher_password';
CREATE ROLE newmmo_app LOGIN PASSWORD :'app_password';

GRANT USAGE, CREATE ON SCHEMA public TO newmmo_migrator;
GRANT USAGE ON SCHEMA public TO newmmo_content_publisher, newmmo_app;
SQL
)
