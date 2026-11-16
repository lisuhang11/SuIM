#!/usr/bin/env bash
set -euo pipefail

# SuIM application service orchestration. Infrastructure (MySQL, Redis, etcd,
# MinIO) is started separately with docker-compose.yaml.
cd "$(dirname "$0")/.."

GO="${GO:-go}"
BIN_DIR="bin"
PID_DIR="run"
LOG_DIR="logs"
mkdir -p "$BIN_DIR" "$PID_DIR" "$LOG_DIR"

# Start order follows service dependencies. push comes after group because it
# dials group to resolve group members, and consumes toPush independently of
# the gateway's consumer group.
ALL_SERVICES=(
  user third conversation rtc
  relation group
  msg
  msgtransfer
  msggateway
  push
  apigateway
  crontask
)

usage() {
  cat <<'EOF'
Usage: scripts/start.sh <start|stop|restart|status> [service...]

Services:
  user third conversation rtc relation group msg msgtransfer
  msggateway push apigateway crontask

With no service argument the action applies to all services. Configuration
for each service is services/<name>/etc/<name>.yaml; services without a
committed yaml (relation, apigateway, and the .example-only ones) are skipped
with a message until you create the file.
EOF
}

is_service() {
  local want="$1" s
  for s in "${ALL_SERVICES[@]}"; do
    [[ "$s" == "$want" ]] && return 0
  done
  return 1
}

resolve_targets() {
  TARGETS=()
  if [[ $# -eq 0 ]]; then
    TARGETS=("${ALL_SERVICES[@]}")
    return
  fi
  local svc
  for svc in "$@"; do
    if ! is_service "$svc"; then
      echo "unknown service: $svc" >&2
      exit 2
    fi
    TARGETS+=("$svc")
  done
}

pid_file() { echo "$PID_DIR/$1.pid"; }

read_pid() { cat "$(pid_file "$1")" 2>/dev/null || true; }

alive() {
  local pid
  pid=$(read_pid "$1")
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

start_one() {
  local svc="$1" cfg="services/$svc/etc/$svc.yaml" pid
  if alive "$svc"; then
    echo "$svc already running (pid $(read_pid "$svc"))"
    return
  fi
  if [[ ! -f "$cfg" ]]; then
    echo "skip $svc: missing $cfg" >&2
    return
  fi
  if ! "$GO" build -o "$BIN_DIR/$svc" "./services/$svc"; then
    echo "build failed: $svc" >&2
    return 1
  fi
  nohup "$BIN_DIR/$svc" -c "$cfg" >>"$LOG_DIR/$svc.log" 2>&1 &
  pid=$!
  echo "$pid" > "$(pid_file "$svc")"
  echo "started $svc (pid $pid, log $LOG_DIR/$svc.log)"
  sleep 1
}

stop_one() {
  local svc="$1" pid
  pid=$(read_pid "$svc")
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid"
    echo "stopped $svc (pid $pid)"
  fi
  rm -f "$(pid_file "$svc")"
}

status_one() {
  local svc="$1"
  if alive "$svc"; then
    echo "$svc running (pid $(read_pid "$svc"))"
  else
    echo "$svc stopped"
  fi
}

main() {
  local action="${1:-}"
  if [[ $# -gt 0 ]]; then shift; fi
  local list=()
  case "$action" in
    start|stop|restart|status)
      resolve_targets "$@"
      list=("${TARGETS[@]}")
      ;;
    *)
      usage
      exit 2
      ;;
  esac

  case "$action" in
    start)
      for svc in "${list[@]}"; do start_one "$svc"; done
      ;;
    stop)
      local i
      for ((i = ${#list[@]} - 1; i >= 0; i--)); do stop_one "${list[i]}"; done
      ;;
    restart)
      local i
      for ((i = ${#list[@]} - 1; i >= 0; i--)); do stop_one "${list[i]}"; done
      for svc in "${list[@]}"; do start_one "$svc"; done
      ;;
    status)
      for svc in "${list[@]}"; do status_one "$svc"; done
      ;;
  esac
}

main "$@"
