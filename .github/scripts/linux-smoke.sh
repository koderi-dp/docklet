#!/usr/bin/env bash
# Runs Docklet under a virtual display and a private session bus (used by the Linux CI job).
#
#   linux-smoke.sh stays-up BINARY   passes if BINARY is still running after 15 seconds
#   linux-smoke.sh refuses BINARY    passes if BINARY exits with status 78 and the friendly message
#
# The application log is printed at the end, pass or fail. DOCKLET_LOG selects the log file.
set -uo pipefail

export WEBKIT_DISABLE_DMABUF_RENDERER=1
export WEBKIT_DISABLE_COMPOSITING_MODE=1
export GDK_BACKEND=x11
log="${DOCKLET_LOG:-/tmp/docklet.log}"

run_inner() {
  local mode="$1" bin="$2"
  if [ "$mode" = stays-up ]; then
    "$bin" >"$log" 2>&1 &
    local pid=$!
    sleep 15
    if kill -0 "$pid" 2>/dev/null; then
      echo "docklet is still running after 15s"
      kill "$pid"
      return 0
    fi
    wait "$pid"
    echo "docklet exited early with status $?"
    return 1
  fi

  timeout 30 "$bin" >"$log" 2>&1
  local status=$?
  echo "docklet exited with status $status"
  [ "$status" -eq 78 ] || { echo "expected exit status 78"; return 1; }
  grep -q "Docklet cannot start" "$log" || { echo "the friendly message is missing"; return 1; }
}

if [ "${1:-}" = inner ]; then
  run_inner "$2" "$3"
  exit $?
fi

mode="$1"
bin="$2"
mkdir -p "$HOME"
status=0
xvfb-run -a --server-args="-screen 0 1440x900x24" dbus-run-session -- bash "$0" inner "$mode" "$bin" || status=$?
echo "---- application log ($bin) ----"
cat "$log" || true
exit "$status"
