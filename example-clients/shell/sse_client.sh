#!/usr/bin/env bash
# Consume a WorldSignal subscription as Server-Sent Events — just curl + jq.
# curl -N streams the response; we forward each `data:` line to jq.
# Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX.
set -uo pipefail

BASE="${WS_API_BASE:-http://localhost:4000}"
KEY="${WS_API_KEY:?WS_API_KEY is required}"
SUB="${WS_SUBSCRIPTION:-demo-stream}"
SINCE="${WS_SINCE:-0}"
MAX="${WS_MAX:-0}"

URL="$BASE/v1/stream/sse?subscription=$SUB&since=$SINCE"
echo "[sse] connecting to $URL" >&2

seen=0
# -N disables buffering so lines arrive as they are sent.
curl -sN -H "Authorization: Bearer $KEY" "$URL" | while IFS= read -r line; do
  case "$line" in
    data:*) ;;              # an event payload
    *) continue ;;          # skip id:/event:/comment/blank lines
  esac
  json="${line#data:}"
  read -r sev country title < <(jq -r '.data | "\(.severity // "?") \(.country // "--") \(.title // "")"' <<<"$json")
  printf '[sse] %-8s %s  %s\n' "$sev" "$country" "$title"
  seen=$((seen + 1))
  if [ "$MAX" -gt 0 ] && [ "$seen" -ge "$MAX" ]; then
    echo "[sse] received $seen event(s), exiting" >&2
    exit 0
  fi
done
