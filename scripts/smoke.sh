#!/usr/bin/env bash
# End-to-end smoke test of the API. Requires the server running and `jq`.
# Usage: ./scripts/smoke.sh [base_url]   (default http://localhost:8080)
set -u

BASE="${1:-http://localhost:8080}"
PASS=0
FAIL=0

# check <label> <expected_status> <method> <path> [curl args...]
check() {
  local label="$1" want="$2" method="$3" path="$4"
  shift 4
  local got
  got=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "$BASE$path" "$@")
  if [ "$got" = "$want" ]; then
    printf '  ok   %-40s %s\n' "$label" "$got"
    PASS=$((PASS + 1))
  else
    printf '  FAIL %-40s want %s got %s\n' "$label" "$want" "$got"
    FAIL=$((FAIL + 1))
  fi
}

# unique creds so the script is re-runnable without a DB reset
SUFFIX="$RANDOM$RANDOM"
NEO="neo_$SUFFIX"
TRIN="trinity_$SUFFIX"

echo "== auth =="
check "health"                 200 GET  /health
check "register neo"           201 POST /users -d "{\"username\":\"$NEO\",\"email\":\"$NEO@x.io\",\"password\":\"whiterabbit\"}"
check "register neo (dup)"     409 POST /users -d "{\"username\":\"$NEO\",\"email\":\"$NEO@x.io\",\"password\":\"whiterabbit\"}"
check "register short pw"      400 POST /users -d "{\"username\":\"z_$SUFFIX\",\"email\":\"z_$SUFFIX@x.io\",\"password\":\"short\"}"
check "register bad email"     400 POST /users -d "{\"username\":\"y_$SUFFIX\",\"email\":\"nope\",\"password\":\"whiterabbit\"}"
check "login wrong password"   401 POST /tokens/authentication -d "{\"username\":\"$NEO\",\"password\":\"nope\"}"
check "login unknown user"     401 POST /tokens/authentication -d "{\"username\":\"ghost_$SUFFIX\",\"password\":\"x\"}"

curl -s -X POST "$BASE/users" -d "{\"username\":\"$TRIN\",\"email\":\"$TRIN@x.io\",\"password\":\"redpill123\"}" >/dev/null
NEO_TOKEN=$(curl -s -X POST "$BASE/tokens/authentication" -d "{\"username\":\"$NEO\",\"password\":\"whiterabbit\"}" | jq -r .auth_token.token)
TRIN_TOKEN=$(curl -s -X POST "$BASE/tokens/authentication" -d "{\"username\":\"$TRIN\",\"password\":\"redpill123\"}" | jq -r .auth_token.token)
AUTH="Authorization: Bearer $NEO_TOKEN"
AUTH_TRIN="Authorization: Bearer $TRIN_TOKEN"

echo "== workouts =="
check "create workout no token" 401 POST /workouts -d '{}'
WID=$(curl -s -X POST "$BASE/workouts" -H "$AUTH" -d '{
  "title":"push day","duration_minutes":60,
  "entries":[{"exercise_name":"bench","sets":3,"reps":10,"order_index":1}]
}' | jq -r .workout.id)
echo "  (created workout id=$WID)"

check "get own workout"        200 GET    "/workouts/$WID" -H "$AUTH"
check "get as other (IDOR)"    403 GET    "/workouts/$WID" -H "$AUTH_TRIN"
check "get missing workout"    404 GET    /workouts/99999999 -H "$AUTH"
check "get non-numeric id"     400 GET    /workouts/abc -H "$AUTH"
check "update as other"        403 PUT    "/workouts/$WID" -H "$AUTH_TRIN" -d '{"title":"hacked"}'
check "update own"             200 PUT    "/workouts/$WID" -H "$AUTH" -d '{"title":"pull day"}'
check "delete as other"        403 DELETE "/workouts/$WID" -H "$AUTH_TRIN"
check "delete own"             204 DELETE "/workouts/$WID" -H "$AUTH"
check "get deleted workout"    404 GET    "/workouts/$WID" -H "$AUTH"

echo
echo "== $PASS passed, $FAIL failed =="
[ "$FAIL" -eq 0 ]
