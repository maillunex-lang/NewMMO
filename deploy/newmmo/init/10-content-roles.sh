#!/usr/bin/env bash
set -euo pipefail

psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1 <<'SQL'
\getenv migrator_password NEWMMO_MIGRATOR_PASSWORD
\getenv app_password NEWMMO_APP_PASSWORD

CREATE ROLE newmmo_migrator LOGIN PASSWORD :'migrator_password';
CREATE ROLE newmmo_app LOGIN PASSWORD :'app_password';

GRANT USAGE, CREATE ON SCHEMA public TO newmmo_migrator;
GRANT USAGE ON SCHEMA public TO newmmo_app;

ALTER DEFAULT PRIVILEGES FOR ROLE newmmo_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO newmmo_app;
ALTER DEFAULT PRIVILEGES FOR ROLE newmmo_migrator IN SCHEMA public
    GRANT USAGE ON SEQUENCES TO newmmo_app;
SQL
