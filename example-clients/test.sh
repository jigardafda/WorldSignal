#!/usr/bin/env bash
# End-to-end test of every example client against a running WorldSignal server.
#
# Provisions a throwaway subscription with a no-match filter (so only our own
# injected delivery lands in its feed) plus one known delivery, then runs each
# client with WS_SINCE=0 WS_MAX=1 and asserts it received that exact event.
#
#   ./test.sh              # uses WS_API_BASE / DATABASE_URL / admin defaults
set -uo pipefail

BASE="${WS_API_BASE:-http://localhost:4800}"
DB="${DATABASE_URL:-postgresql://worldsignal:worldsignal@localhost:5432/worldsignal?sslmode=disable}"
HERE="$(cd "$(dirname "$0")" && pwd)"
SUB="client-e2e-$(python3 -c 'import time;print(int(time.time()))')"
MARKER="__E2E_$(python3 -c 'import time;print(int(time.time()*1000))')__"
PASS=0
FAIL=0

say() { printf '\n\033[1m== %s ==\033[0m\n' "$*"; }
run_to() { perl -e 'alarm shift; exec @ARGV' "$@"; } # portable `timeout` (macOS has none)
check() { # label  output  expected
  if grep -qF "$3" <<<"$2"; then printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS + 1));
  else printf '  \033[31m✗\033[0m %s (missing %q)\n' "$1" "$3"; sed 's/^/      /' <<<"$2"; FAIL=$((FAIL + 1)); fi
}
cleanup() { psql "$DB" -q -c "DELETE FROM \"Subscription\" WHERE id='$SUB'" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# --- API key -----------------------------------------------------------------
KEY="${WS_API_KEY:-}"
if [ -z "$KEY" ]; then
  say "Provisioning a signals:read API key (admin GraphQL)"
  TOKEN=$(curl -s "$BASE/graphql" -H 'content-type: application/json' \
    -d '{"query":"mutation($e:String!,$p:String!){login(email:$e,password:$p){token}}","variables":{"e":"admin@worldsignal.local","p":"admin12345"}}' \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["login"]["token"])')
  KEY=$(curl -s "$BASE/graphql" -H 'content-type: application/json' -H "authorization: Bearer $TOKEN" \
    -d '{"query":"mutation($i:CreateApiKeyInput!){createApiKey(input:$i){key}}","variables":{"i":{"name":"client-e2e","scopes":["signals:read"]}}}' \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["createApiKey"]["key"])')
fi
[ -n "$KEY" ] || { echo "could not obtain an API key"; exit 1; }

# --- Seed a throwaway subscription + one known delivery ----------------------
say "Seeding subscription $SUB + one known delivery"
psql "$DB" -q -c "INSERT INTO \"Subscriber\"(\"id\",\"name\",\"createdAt\") VALUES('__default__','D',now()) ON CONFLICT DO NOTHING"
# filter matches nothing real, so only our injected row appears in the feed
psql "$DB" -q -c "INSERT INTO \"Subscription\"(\"id\",\"subscriberId\",\"name\",\"channel\",\"filter\",\"config\",\"createdAt\") VALUES('$SUB','__default__','e2e','SSE','{\"keyword\":\"$MARKER\"}','{}',now())"
psql "$DB" -q -c "INSERT INTO \"Signal\"(\"id\",\"title\",\"summary\",\"status\",\"severity\",\"confidence\",\"sourceCount\",\"firstSeenAt\",\"lastSeenAt\",\"updatedAt\") VALUES('$SUB','$MARKER title','s','CONFIRMED','HIGH',0.9,1,now(),now(),now())"
psql "$DB" -q -c "INSERT INTO \"DeliveryEvent\"(\"id\",\"subscriptionId\",\"signalId\",\"channel\",\"status\",\"payload\",\"createdAt\") VALUES('$SUB','$SUB','$SUB','SSE','SENT','{\"data\":{\"title\":\"$MARKER title\",\"severity\":\"HIGH\",\"country\":\"US\"},\"event_id\":\"$SUB\"}',now())"

export WS_API_BASE="$BASE" WS_API_KEY="$KEY" WS_SUBSCRIPTION="$SUB" WS_SINCE=0 WS_MAX=1

# --- Python clients ----------------------------------------------------------
say "Python clients"
check "python  sse"  "$(run_to 15 python3 "$HERE/python/sse_client.py"  2>/dev/null)" "$MARKER"
check "python  poll" "$(WS_INTERVAL=1 run_to 15 python3 "$HERE/python/poll_client.py" 2>/dev/null)" "$MARKER"
if python3 -c 'import websockets' 2>/dev/null; then
  check "python  ws" "$(run_to 15 python3 "$HERE/python/ws_client.py" 2>/dev/null)" "$MARKER"
else echo "  … python ws skipped (pip install -r python/requirements.txt)"; fi

# --- Python webhook receiver (signed vs unsigned) ----------------------------
say "Python webhook receiver — signature verification"
SECRET="${WS_WEBHOOK_SECRET:-dev-secret}"
WHOUT=$(mktemp)
WS_WEBHOOK_SECRET="$SECRET" WS_PORT=8091 WS_MAX=1 python3 "$HERE/python/webhook_receiver.py" >"$WHOUT" 2>/dev/null &
WHPID=$!
sleep 1
BODY="{\"data\":{\"title\":\"$MARKER webhook\",\"severity\":\"LOW\",\"country\":\"GB\"},\"event_id\":\"wh\"}"
SIG=$(python3 -c "import hmac,hashlib;print('sha256='+hmac.new(b'''$SECRET''',b'''$BODY''',hashlib.sha256).hexdigest())")
BADCODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://localhost:8091" -H 'X-WorldSignal-Signature: sha256=bad' -d "$BODY")
GOODCODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://localhost:8091" -H "X-WorldSignal-Signature: $SIG" -d "$BODY")
sleep 1
kill "$WHPID" 2>/dev/null || true
check "webhook rejects bad signature (401)" "$BADCODE" "401"
check "webhook accepts signed (200)" "$GOODCODE" "200"
check "webhook prints verified event" "$(cat "$WHOUT")" "$MARKER"
rm -f "$WHOUT"

# --- TypeScript clients (if Node is available) -------------------------------
if command -v node >/dev/null 2>&1; then
  say "TypeScript clients (npm install once)"
  ( cd "$HERE/typescript" && npm install --silent >/dev/null 2>&1 )
  check "ts  sse"  "$(cd "$HERE/typescript" && run_to 20 npx tsx sse_client.ts  2>/dev/null)" "$MARKER"
  check "ts  poll" "$(cd "$HERE/typescript" && WS_INTERVAL=1 run_to 20 npx tsx poll_client.ts 2>/dev/null)" "$MARKER"
  check "ts  ws"   "$(cd "$HERE/typescript" && run_to 20 npx tsx ws_client.ts   2>/dev/null)" "$MARKER"
else echo "  … TypeScript skipped (node not found)"; fi

# --- Node.js clients (dependency-free, Node 21+ for the WebSocket global) -----
if command -v node >/dev/null 2>&1; then
  say "Node.js clients (dependency-free)"
  check "node  sse"  "$(run_to 15 node "$HERE/node/sse_client.mjs"  2>/dev/null)" "$MARKER"
  check "node  poll" "$(WS_INTERVAL=1 run_to 15 node "$HERE/node/poll_client.mjs" 2>/dev/null)" "$MARKER"
  check "node  ws"   "$(run_to 15 node "$HERE/node/ws_client.mjs"   2>/dev/null)" "$MARKER"
else echo "  … Node.js skipped (node not found)"; fi

# --- Go clients (stdlib only) ------------------------------------------------
if command -v go >/dev/null 2>&1; then
  say "Go clients (stdlib only)"
  check "go  sse"  "$(cd "$HERE/go" && run_to 40 go run ./sse  2>/dev/null)" "$MARKER"
  check "go  poll" "$(cd "$HERE/go" && WS_INTERVAL=1 run_to 40 go run ./poll 2>/dev/null)" "$MARKER"
else echo "  … Go skipped (go not found)"; fi

# --- Ruby clients (stdlib only) ----------------------------------------------
if command -v ruby >/dev/null 2>&1; then
  say "Ruby clients (stdlib only)"
  check "ruby  sse"  "$(run_to 15 ruby "$HERE/ruby/sse_client.rb"  2>/dev/null)" "$MARKER"
  check "ruby  poll" "$(WS_INTERVAL=1 run_to 15 ruby "$HERE/ruby/poll_client.rb" 2>/dev/null)" "$MARKER"
else echo "  … Ruby skipped (ruby not found)"; fi

# --- PHP clients -------------------------------------------------------------
if command -v php >/dev/null 2>&1; then
  say "PHP clients"
  check "php  sse"  "$(run_to 15 php "$HERE/php/sse_client.php"  2>/dev/null)" "$MARKER"
  check "php  poll" "$(WS_INTERVAL=1 run_to 15 php "$HERE/php/poll_client.php" 2>/dev/null)" "$MARKER"
else echo "  … PHP skipped (php not found)"; fi

# --- Shell clients (curl + jq) -----------------------------------------------
if command -v jq >/dev/null 2>&1; then
  say "Shell clients (curl + jq)"
  check "shell  sse"  "$(run_to 15 bash "$HERE/shell/sse_client.sh"  2>/dev/null)" "$MARKER"
  check "shell  poll" "$(WS_INTERVAL=1 run_to 15 bash "$HERE/shell/poll_client.sh" 2>/dev/null)" "$MARKER"
else echo "  … Shell skipped (jq not found)"; fi

# --- Result ------------------------------------------------------------------
printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
