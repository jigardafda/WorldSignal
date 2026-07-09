#!/usr/bin/env bash
# Start the WorldSignal Go backend + React frontend together.
#   ./dev.sh                 # backend on :4800, frontend on :5400
#   PORT=4002 ./dev.sh       # backend on :4002 (e.g. if 4800 is busy)
# Ctrl+C stops both.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load local secrets (gitignored) if present: OPENAI_API_KEY, OPENAI_MODEL, etc.
if [ -f "$ROOT/backend/.env.local" ]; then
  set -a; . "$ROOT/backend/.env.local"; set +a
  echo "loaded backend/.env.local"
fi

export DATABASE_URL="${DATABASE_URL:-postgresql://worldsignal:worldsignal@localhost:5432/worldsignal?sslmode=disable}"
export OPENAI_API_KEY="${OPENAI_API_KEY:-}"          # empty → heuristic enrichment
export ROLE="${ROLE:-all}"                            # api + workers + scheduler
export HOST="${HOST:-127.0.0.1}"
export PORT="${PORT:-4800}"
export WEBHOOK_SIGNING_SECRET="${WEBHOOK_SIGNING_SECRET:-dev-secret}"

# Kill the whole process group (backend + frontend) on exit / Ctrl+C.
cleanup() { echo; echo "stopping WorldSignal…"; kill 0 2>/dev/null || true; }
trap cleanup EXIT INT TERM

echo "building Go backend…"
( cd "$ROOT/backend" && go build -o /tmp/ws-go ./cmd/server )

echo "backend → http://$HOST:$PORT   frontend → http://localhost:5400"
"/tmp/ws-go" &
WS_BACKEND="http://$HOST:$PORT" npm --prefix "$ROOT/frontend" run dev &

wait
