#!/usr/bin/env bash
# Consume a WorldSignal subscription by polling — just curl + jq.
# Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX,
#      WS_INTERVAL (seconds, default 3).
set -euo pipefail

BASE="${WS_API_BASE:-http://localhost:4800}"
KEY="${WS_API_KEY:?WS_API_KEY is required}"
SUB="${WS_SUBSCRIPTION:-demo-stream}"
CURSOR="${WS_SINCE:-0}"
MAX="${WS_MAX:-0}"
INTERVAL="${WS_INTERVAL:-3}"

seen=0
echo "[poll] $BASE/v1/stream/poll subscription=$SUB" >&2
while :; do
  resp="$(curl -s -H "Authorization: Bearer $KEY" "$BASE/v1/stream/poll?subscription=$SUB&since=$CURSOR")"
  while IFS=$'\t' read -r sev country title; do
    [ -z "$sev$country$title" ] && continue
    printf '[poll] %-8s %s  %s\n' "${sev:-?}" "${country:---}" "$title"
    seen=$((seen + 1))
    if [ "$MAX" -gt 0 ] && [ "$seen" -ge "$MAX" ]; then exit 0; fi
  done < <(jq -r '.events[]?.payload.data | [.severity, .country, .title] | @tsv' <<<"$resp")
  CURSOR="$(jq -r '.cursor // 0' <<<"$resp")"
  sleep "$INTERVAL"
done
