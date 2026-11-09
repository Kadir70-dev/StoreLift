#!/usr/bin/env bash
set -euo pipefail
BASE="${API_URL:-http://127.0.0.1:8080}"
code(){curl -sS --max-time 8 -o /dev/null -w '%{http_code}' "$@";}
expect(){local want="$1";shift;local got;got="$(code "$@")";if [[ "$got" != "$want" ]];then echo "Expected HTTP $want, got $got for $*" >&2;exit 1;fi;}
expect 200 "$BASE/api/health"
expect 400 -X POST "$BASE/api/audits" -H 'Content-Type: application/json' -d '{"website":"http://127.0.0.1/"}'
expect 400 -X POST "$BASE/api/audits" -H 'Content-Type: application/json' -d '{"website":"https://169.254.169.254/"}'
expect 400 -X POST "$BASE/api/leads" -H 'Content-Type: application/json' -d '{"email":"x@example.com","website":"https://example.com","consent":false}'
expect 401 "$BASE/api/admin/leads"
expect 401 "$BASE/api/admin/deals"
expect 401 -X POST "$BASE/api/admin/comparisons" -H 'Content-Type: application/json' -d '{}'
expect 503 -X POST "$BASE/api/checkout" -H 'Content-Type: application/json' -d '{"leadId":"00000000-0000-0000-0000-000000000001"}'
echo 'API negative/security route smoke passed.'
