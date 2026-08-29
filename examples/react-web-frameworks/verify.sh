#!/bin/sh
set -eu

example_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace=$(mktemp -d)
gin_pid=
fiber_pid=
vite_pid=

cleanup() {
  for pid in "$vite_pid" "$fiber_pid" "$gin_pid"; do
    if [ -n "$pid" ]; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  rm -rf "$workspace"
}
trap cleanup EXIT HUP INT TERM

fail_with_logs() {
  echo "$1" >&2
  for log in "$workspace"/*.log; do
    if [ -f "$log" ]; then
      echo "--- $(basename "$log") ---" >&2
      sed -n '1,160p' "$log" >&2
    fi
  done
  exit 1
}

wait_for() {
  url=$1
  attempts=0
  until curl -fsS "$url" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 30 ]; then
      fail_with_logs "timed out waiting for $url"
    fi
    sleep 1
  done
}

cd "$example_dir/backend"
ontama build -o "$workspace/gin-demo" gin/main.otm
ontama build -o "$workspace/fiber-demo" fiber/main.otm
"$workspace/gin-demo" >"$workspace/gin.log" 2>&1 &
gin_pid=$!
"$workspace/fiber-demo" >"$workspace/fiber.log" 2>&1 &
fiber_pid=$!

cd "$example_dir/frontend"
npm run dev -- --host 127.0.0.1 --strictPort >"$workspace/vite.log" 2>&1 &
vite_pid=$!

wait_for http://127.0.0.1:5173/
wait_for http://127.0.0.1:5173/gin-api/health
wait_for http://127.0.0.1:5173/fiber-api/health

page=$(curl -fsS http://127.0.0.1:5173/)
case "$page" in
  *"OnsenTamago Web Demo"*) ;;
  *) fail_with_logs "Vite did not serve the OnsenTamago application" ;;
esac

gin_health=$(curl -fsS http://127.0.0.1:5173/gin-api/health)
fiber_health=$(curl -fsS http://127.0.0.1:5173/fiber-api/health)
gin_created=$(curl -fsS -X POST -H 'Content-Type: application/json' -d '{"title":"full-stack Gin"}' http://127.0.0.1:5173/gin-api/todos)
fiber_created=$(curl -fsS -X POST -H 'Content-Type: application/json' -d '{"title":"full-stack Fiber"}' http://127.0.0.1:5173/fiber-api/todos)

[ "$gin_health" = '{"framework":"Gin","language":"OnsenTamago","status":"ok"}' ] || fail_with_logs "unexpected Gin health response: $gin_health"
[ "$fiber_health" = '{"framework":"Fiber","language":"OnsenTamago","status":"ok"}' ] || fail_with_logs "unexpected Fiber health response: $fiber_health"
[ "$gin_created" = '{"item":{"completed":false,"id":3,"title":"full-stack Gin"}}' ] || fail_with_logs "unexpected Gin create response: $gin_created"
[ "$fiber_created" = '{"item":{"completed":false,"id":3,"title":"full-stack Fiber"}}' ] || fail_with_logs "unexpected Fiber create response: $fiber_created"

echo "full-stack Vite/Gin/Fiber smoke contract passed"
