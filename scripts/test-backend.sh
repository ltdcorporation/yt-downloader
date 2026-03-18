#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/apps/backend"
ENV_FILE="$BACKEND_DIR/.env"

STRICT_INTEGRATION="true"
WITH_COVER="true"
COVER_FILE="${COVER_FILE:-$ROOT_DIR/tmp/backend-cover.out}"

REDIS_ADDR_OVERRIDE=""
REDIS_PASSWORD_OVERRIDE=""
PG_ADMIN_DSN_OVERRIDE=""

EXTRA_GO_TEST_ARGS=()

usage() {
  cat <<'EOF'
Usage: ./scripts/test-backend.sh [options] [-- <extra go test args>]

Run full backend tests with Redis + Postgres integration checks.

Options:
  --redis-addr <host:port>      Override Redis test address
  --redis-password <password>   Override Redis password
  --pg-admin-dsn <dsn>          Override Postgres admin DSN used by integration tests
  --allow-integration-skip      Do not fail when integration tests are skipped
  --no-cover                    Disable go coverage output/profile generation
  -h, --help                    Show this help

Env overrides:
  YTD_TEST_REDIS_ADDR
  YTD_TEST_REDIS_PASSWORD
  YTD_TEST_POSTGRES_ADMIN_DSN
  COVER_FILE

Notes:
  - By default, script reads apps/backend/.env for REDIS_ADDR/REDIS_PASSWORD/POSTGRES_DSN.
  - Postgres admin DSN is auto-derived from POSTGRES_DSN by switching DB path to /postgres.
EOF
}

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%d %H:%M:%S UTC')" "$*"
}

die() {
  log "ERROR: $*"
  exit 1
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "Required command not found: $cmd"
}

read_env_value() {
  local key="$1"
  local file="$2"
  local fallback="${3:-}"

  if [[ ! -f "$file" ]]; then
    printf '%s' "$fallback"
    return 0
  fi

  local line
  line="$(grep -E "^${key}=" "$file" | tail -n1 || true)"
  if [[ -z "$line" ]]; then
    printf '%s' "$fallback"
    return 0
  fi

  local value
  value="${line#*=}"
  value="${value%$'\r'}"

  if [[ ${#value} -ge 2 ]]; then
    if [[ "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
      value="${value:1:${#value}-2}"
    elif [[ "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
      value="${value:1:${#value}-2}"
    fi
  fi

  printf '%s' "$value"
}

derive_pg_admin_dsn() {
  local source_dsn="$1"

  python3 - "$source_dsn" <<'PY'
import sys
from urllib.parse import urlparse, urlunparse

dsn = (sys.argv[1] or "").strip()
if not dsn:
    print("")
    raise SystemExit(0)

parsed = urlparse(dsn)
if not parsed.scheme or not parsed.netloc:
    print(dsn)
    raise SystemExit(0)

path = parsed.path or ""
if path in ("", "/"):
    new_path = "/postgres"
else:
    new_path = "/postgres"

print(urlunparse(parsed._replace(path=new_path)))
PY
}

split_host_port() {
  local addr="$1"

  python3 - "$addr" <<'PY'
import sys

addr = (sys.argv[1] or "").strip()
if not addr:
    print("")
    print("")
    raise SystemExit(0)

host = ""
port = ""

if addr.startswith("["):
    end = addr.find("]")
    if end != -1:
        host = addr[1:end]
        rest = addr[end+1:]
        if rest.startswith(":") and len(rest) > 1:
            port = rest[1:]
elif ":" in addr:
    host, port = addr.rsplit(":", 1)
else:
    host = addr

if not port:
    port = "6379"

print(host)
print(port)
PY
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --redis-addr)
        [[ $# -ge 2 ]] || die "Missing value for --redis-addr"
        REDIS_ADDR_OVERRIDE="$2"
        shift 2
        ;;
      --redis-password)
        [[ $# -ge 2 ]] || die "Missing value for --redis-password"
        REDIS_PASSWORD_OVERRIDE="$2"
        shift 2
        ;;
      --pg-admin-dsn)
        [[ $# -ge 2 ]] || die "Missing value for --pg-admin-dsn"
        PG_ADMIN_DSN_OVERRIDE="$2"
        shift 2
        ;;
      --allow-integration-skip)
        STRICT_INTEGRATION="false"
        shift
        ;;
      --no-cover)
        WITH_COVER="false"
        shift
        ;;
      --)
        shift
        EXTRA_GO_TEST_ARGS+=("$@")
        break
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
  done
}

check_redis() {
  local addr="$1"
  local password="$2"

  mapfile -t hp < <(split_host_port "$addr")
  local host="${hp[0]:-}"
  local port="${hp[1]:-}"

  [[ -n "$host" ]] || die "Invalid redis address: $addr"
  [[ -n "$port" ]] || die "Invalid redis port from address: $addr"

  local cmd=(redis-cli -h "$host" -p "$port")
  if [[ -n "$password" ]]; then
    cmd+=(-a "$password")
  fi
  cmd+=(ping)

  local out
  out="$(${cmd[@]} 2>/dev/null || true)"
  if [[ "$out" != "PONG" ]]; then
    die "Redis ping failed for ${host}:${port} (output: ${out:-<empty>})"
  fi
}

check_postgres() {
  local dsn="$1"
  [[ -n "$dsn" ]] || die "Postgres admin DSN is empty"

  if ! PGCONNECT_TIMEOUT=5 psql "$dsn" -Atqc 'select 1' >/dev/null 2>&1; then
    die "Postgres connectivity check failed for DSN: $dsn"
  fi
}

main() {
  parse_args "$@"

  require_cmd go
  require_cmd python3
  require_cmd redis-cli
  require_cmd psql

  local redis_addr redis_password pg_admin_dsn
  redis_addr="${REDIS_ADDR_OVERRIDE:-${YTD_TEST_REDIS_ADDR:-$(read_env_value REDIS_ADDR "$ENV_FILE" "127.0.0.1:6379")}}"
  redis_password="${REDIS_PASSWORD_OVERRIDE:-${YTD_TEST_REDIS_PASSWORD:-$(read_env_value REDIS_PASSWORD "$ENV_FILE" "")}}"

  local source_pg_dsn
  source_pg_dsn="$(read_env_value POSTGRES_DSN "$ENV_FILE" "")"
  pg_admin_dsn="${PG_ADMIN_DSN_OVERRIDE:-${YTD_TEST_POSTGRES_ADMIN_DSN:-$(derive_pg_admin_dsn "$source_pg_dsn")}}"

  [[ -n "$redis_addr" ]] || die "Cannot resolve Redis address"
  [[ -n "$pg_admin_dsn" ]] || die "Cannot resolve Postgres admin DSN (set --pg-admin-dsn or YTD_TEST_POSTGRES_ADMIN_DSN)"

  export YTD_TEST_REDIS_ADDR="$redis_addr"
  export YTD_TEST_REDIS_PASSWORD="$redis_password"
  export YTD_TEST_POSTGRES_ADMIN_DSN="$pg_admin_dsn"

  log "Using Redis: $YTD_TEST_REDIS_ADDR"
  log "Using Postgres admin DSN: $YTD_TEST_POSTGRES_ADMIN_DSN"

  log "Preflight: checking Redis connectivity"
  check_redis "$YTD_TEST_REDIS_ADDR" "$YTD_TEST_REDIS_PASSWORD"

  log "Preflight: checking Postgres connectivity"
  check_postgres "$YTD_TEST_POSTGRES_ADMIN_DSN"

  local test_log
  test_log="$(mktemp)"

  local -a test_cmd
  test_cmd=(go test ./... -count=1 -v)

  if [[ "$WITH_COVER" == "true" ]]; then
    mkdir -p "$(dirname "$COVER_FILE")"
    test_cmd+=("-coverprofile=$COVER_FILE" -cover)
  fi

  if [[ ${#EXTRA_GO_TEST_ARGS[@]} -gt 0 ]]; then
    test_cmd+=("${EXTRA_GO_TEST_ARGS[@]}")
  fi

  log "Running backend test suite"
  if ! (
    cd "$BACKEND_DIR"
    "${test_cmd[@]}"
  ) 2>&1 | tee "$test_log"; then
    rm -f "$test_log"
    die "Backend tests failed"
  fi

  if [[ "$STRICT_INTEGRATION" == "true" ]]; then
    local run_count
    run_count=$(grep -E '^=== RUN   Test.*Integration' "$test_log" | wc -l | tr -d ' ')
    if [[ "$run_count" -eq 0 ]]; then
      rm -f "$test_log"
      die "No integration tests executed (expected Redis/Postgres integration tests)"
    fi

    if grep -Eq '^--- SKIP: Test.*Integration' "$test_log"; then
      rm -f "$test_log"
      die "One or more integration tests were skipped"
    fi
  fi

  rm -f "$test_log"

  if [[ "$WITH_COVER" == "true" ]]; then
    log "Coverage summary"
    (
      cd "$BACKEND_DIR"
      go tool cover -func="$COVER_FILE" | tail -n 1
    )
    log "Coverage profile written to: $COVER_FILE"
  fi

  log "Backend tests completed successfully"
}

main "$@"
