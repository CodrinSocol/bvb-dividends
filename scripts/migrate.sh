#!/usr/bin/env bash
# Runs goose against the configured database.
#
# Usage: scripts/migrate.sh <goose-command> [args...]
#   scripts/migrate.sh up
#   scripts/migrate.sh down
#   scripts/migrate.sh status
#   scripts/migrate.sh create add_something sql
#
# The same migration files are embedded in the api binary, so what this applies
# and what a deployed build expects cannot drift.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is not set. Copy .env.example to .env, or export it." >&2
  exit 1
fi

exec .bin/goose -dir libs/postgres/migrations postgres "$DATABASE_URL" "$@"
