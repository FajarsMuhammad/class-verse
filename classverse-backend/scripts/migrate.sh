#!/usr/bin/env bash
set -euo pipefail
# Stop author writes before invoking; old binaries do not maintain immutable snapshots.
: "${DATABASE_URL:?DATABASE_URL required}"
version=$(go tool goose -dir db/migrations postgres "$DATABASE_URL" version 2>&1 | sed -n 's/.*goose: version \([0-9]*\).*/\1/p')
if [[ -z "$version" ]]; then
  echo 'Cannot determine goose schema version; migration stopped' >&2
  exit 1
fi
if (( version < 20 )); then
  go tool goose -dir db/migrations postgres "$DATABASE_URL" up-to 20
fi
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f db/upgrades/content_studio_preflight.sql
go tool goose -dir db/migrations postgres "$DATABASE_URL" up
