#!/usr/bin/env bash
# Local load test for go-gym with hey (https://github.com/rakyll/hey).
#
#   go install github.com/rakyll/hey@latest
#   docker compose up -d db
#   go run .                       # in another shell
#   ./scripts/loadtest.sh
#
# Env overrides: BASE, HEY, DUR, CONC.
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
HEY="${HEY:-hey}"
DUR="${DUR:-15s}"
CONC="${CONC:-100}"

# one demo login → reuse its session cookie (the demo route is rate-limited, the
# app pages under test are not).
jar="$(mktemp)"
curl -s -c "$jar" -L "$BASE/demo" -o /dev/null
tok="$(awk '/session/ {print $NF}' "$jar")"
[ -n "$tok" ] || { echo "no session cookie — is the server up at $BASE?"; exit 1; }

for path in /app /app/progress /health "/exercises?q=b"; do
  echo "### GET $path  (c=$CONC for $DUR)"
  "$HEY" -z "$DUR" -c "$CONC" -H "Cookie: session=$tok" "$BASE$path" \
    | grep -E "Requests/sec|Average:|Slowest:|99%|Status code|\[200|\[500" || true
  echo
done
