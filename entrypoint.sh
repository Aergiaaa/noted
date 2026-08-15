#!/bin/sh
set -e

psql "$DB_URL" -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz DEFAULT now())"

for f in /migrations/*.up.sql; do
  v=$(basename "$f")
  if [ "$(psql "$DB_URL" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$v'")" != "1" ]; then
    psql "$DB_URL" -v ON_ERROR_STOP=1 -q -f "$f"
    psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "INSERT INTO schema_migrations (version) VALUES ('$v')"
    echo "applied $v"
  fi
done

exec "$@"