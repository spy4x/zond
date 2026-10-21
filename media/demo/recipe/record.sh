#!/usr/bin/env bash
# Remakes zond's demo video in media/demo/ (see README.md next to this file). Builds zond from this
# checkout, stages it with stand-in services in Docker (project "zond-demo", 127.0.0.1:8080),
# serves a clean demo shell with ttyd, records the raw take with record.ts into media/demo/.take/,
# and encodes it with encode-take.ts. Removes the stack, the image tag, ttyd and its scratch folder
# on every exit path.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
TAKE="$ROOT/media/demo/.take"
IMAGE="zond-demo/zond:local"
S="$(mktemp -d)"
TTYD=""
compose() { docker compose -f "$S/stack/compose.yml" --profile down "$@"; }
cleanup() {
  [ -n "$TTYD" ] && kill "$TTYD" 2>/dev/null || true
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  docker image rm "$IMAGE" >/dev/null 2>&1 || true
  find "$S" -delete
}
trap cleanup EXIT INT TERM

if ss -ltn | grep -q '127.0.0.1:8080 \|\*:8080 \|0.0.0.0:8080 \|\[::\]:8080 '; then
  echo "port 8080 is taken; the video shows localhost:8080, so free it first" >&2
  exit 1
fi
PORT=""
for p in $(seq 7700 7799); do
  if ! ss -ltn | grep -q ":$p "; then PORT=$p; break; fi
done
[ -n "$PORT" ] || { echo "no free port for ttyd in 7700-7799" >&2; exit 1; }

echo "1/4 building zond from this checkout"
docker build -q -t "$IMAGE" "$ROOT" >/dev/null

echo "2/4 staging the stack"
mkdir -p "$S/stack" "$S/home"
cp "$HERE/compose.yml" "$S/stack/"
cp "$HERE/zond.start.yml" "$S/stack/zond.yml"
# Only the stand-ins start here; the video starts zond on screen.
compose up -d hl-metube hl-ollama >/dev/null 2>&1

# A clean shell: no environment, profile or rc of the recorder's, and a staged, empty HOME.
THEME='{"background":"#15120f","foreground":"#efebe2","cursor":"#efebe2","cursorAccent":"#15120f","selectionBackground":"#413c38","black":"#15120f","red":"#f87e77","green":"#83d494","yellow":"#e9c46a","blue":"#8ec5ec","magenta":"#d4a5e8","cyan":"#8ec5ec","white":"#efebe2","brightBlack":"#857f79","brightRed":"#f87e77","brightGreen":"#83d494","brightYellow":"#e9c46a","brightBlue":"#8ec5ec","brightMagenta":"#d4a5e8","brightCyan":"#8ec5ec","brightWhite":"#ffffff"}'
(cd "$S/stack" && exec timeout 2400 ttyd -i 127.0.0.1 -p "$PORT" -W -o \
  -t fontSize=22 -t fontFamily=Hack -t lineHeight=1.25 -t cursorBlink=false \
  -t rendererType=dom -t disableLeaveAlert=true -t disableResizeOverlay=true \
  -t "theme=$THEME" \
  env -i HOME="$S/home" PATH=/usr/local/bin:/usr/bin:/bin TERM=xterm-256color LANG=C.UTF-8 \
  bash --noprofile --rcfile "$HERE/demo.bashrc" -i) >"$S/ttyd.log" 2>&1 &
TTYD=$!
for _ in $(seq 50); do ss -ltn | grep -q ":$PORT " && break; sleep 0.2; done

echo "3/4 recording the take (held screenshots; slow on a loaded machine, not in the video)"
find "$TAKE" -delete 2>/dev/null || true
(cd "$HERE" && timeout 1800 deno run -A record.ts "http://127.0.0.1:$PORT/" "$TAKE")

echo "4/4 encoding"
(cd "$HERE" && timeout 900 deno run -A encode-take.ts "$TAKE")
