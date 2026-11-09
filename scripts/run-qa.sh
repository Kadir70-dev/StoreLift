#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo '[1/5] Go API module tests'
(cd "$ROOT/api" && go test ./...)
echo '[2/5] Frontend TypeScript'
(cd "$ROOT/web" && npm run typecheck)
echo '[3/5] Frontend ESLint'
(cd "$ROOT/web" && npm run lint)
echo '[4/5] Production Next.js build'
(cd "$ROOT/web" && npm run build)
echo '[5/5] Playwright desktop and mobile smoke/browser screenshots'
(cd "$ROOT/web" && npm test)
if command -v psql >/dev/null 2>&1 && [[ -n "${DATABASE_URL:-}" ]]; then
  echo '[extra] PostgreSQL migrations + transactional smoke'
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$ROOT/db/schema.sql" >/dev/null
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$ROOT/db/smoke.sql"
else
  echo 'PostgreSQL smoke skipped: psql and DATABASE_URL are required' >&2
fi
