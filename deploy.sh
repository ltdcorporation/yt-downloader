#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCK_FILE="$ROOT_DIR/.deploy.lock"
BACKEND_ENV_FILE="$ROOT_DIR/apps/backend/.env"

REMOTE="origin"
BRANCH="main"
SCOPE="auto"          # auto | user | system
WITH_WORKER="false"
DO_PULL="true"
DO_SMOKE="true"
DRY_RUN="false"

usage() {
  cat <<'EOF'
Usage: ./deploy.sh [options]

Options:
  --remote <name>       Git remote (default: origin)
  --branch <name>       Git branch (default: main)
  --scope <auto|user|system>
                        Service scope for systemctl (default: auto)
  --with-worker         Include ytd-worker.service in restart target
  --no-pull             Skip git pull step
  --no-smoke            Skip smoke test step
  --dry-run             Print planned commands without executing
  -h, --help            Show this help

Notes:
  - Default service targets: ytd-api.service ytd-web.service
  - Smoke test uses scripts/smoke-mvp.sh against API URL inferred from apps/backend/.env
EOF
}

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%d %H:%M:%S UTC')" "$*"
}

die() {
  log "ERROR: $*"
  exit 1
}

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    log "DRY-RUN: $*"
  else
    log "+ $*"
    "$@"
  fi
}

run_subshell() {
  local cmd="$1"
  if [[ "$DRY_RUN" == "true" ]]; then
    log "DRY-RUN: $cmd"
  else
    log "+ $cmd"
    bash -lc "$cmd"
  fi
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

infer_api_base_url() {
  local http_addr http_port no_scheme
  http_addr="$(read_env_value HTTP_ADDR "$BACKEND_ENV_FILE" "")"
  http_port="$(read_env_value HTTP_PORT "$BACKEND_ENV_FILE" "8080")"

  local port="$http_port"
  if [[ -n "$http_addr" ]]; then
    no_scheme="${http_addr#http://}"
    no_scheme="${no_scheme#https://}"
    if [[ "$no_scheme" =~ :([0-9]+)$ ]]; then
      port="${BASH_REMATCH[1]}"
    fi
  fi

  printf 'http://127.0.0.1:%s' "$port"
}

service_exists() {
  local scope="$1"
  local service="$2"

  if [[ "$scope" == "user" ]]; then
    systemctl --user cat "$service" >/dev/null 2>&1
  else
    systemctl cat "$service" >/dev/null 2>&1
  fi
}

detect_scope() {
  if [[ "$SCOPE" == "user" || "$SCOPE" == "system" ]]; then
    printf '%s' "$SCOPE"
    return 0
  fi

  if service_exists "user" "ytd-api.service" || service_exists "user" "ytd-web.service"; then
    printf 'user'
    return 0
  fi

  if service_exists "system" "ytd-api.service" || service_exists "system" "ytd-web.service"; then
    printf 'system'
    return 0
  fi

  die "Cannot auto-detect service scope. Set --scope user or --scope system."
}

restart_services() {
  local scope="$1"
  shift
  local services=("$@")

  if [[ ${#services[@]} -eq 0 ]]; then
    die "No services selected for restart"
  fi

  log "Restart scope: $scope"
  log "Services: ${services[*]}"

  for svc in "${services[@]}"; do
    service_exists "$scope" "$svc" || die "Service not found in ${scope} scope: $svc"
  done

  if [[ "$scope" == "user" ]]; then
    run systemctl --user daemon-reload
    run systemctl --user restart "${services[@]}"
    for svc in "${services[@]}"; do
      run systemctl --user is-active --quiet "$svc"
    done
  else
    require_cmd sudo
    run sudo systemctl daemon-reload
    run sudo systemctl restart "${services[@]}"
    for svc in "${services[@]}"; do
      run sudo systemctl is-active --quiet "$svc"
    done
  fi
}

wait_for_api_health() {
  local api_base_url="$1"
  local retries="${2:-30}"
  local sleep_seconds="${3:-2}"

  if [[ "$DRY_RUN" == "true" ]]; then
    log "DRY-RUN: wait for ${api_base_url}/healthz"
    return 0
  fi

  for ((i=1; i<=retries; i++)); do
    if curl -fsS "${api_base_url}/healthz" >/dev/null 2>&1; then
      log "API healthy at ${api_base_url}/healthz"
      return 0
    fi
    sleep "$sleep_seconds"
  done

  return 1
}

run_smoke() {
  local api_base_url="$1"

  [[ -x "$ROOT_DIR/scripts/smoke-mvp.sh" ]] || die "Smoke script not found or not executable: scripts/smoke-mvp.sh"

  local admin_user admin_pass
  admin_user="$(read_env_value ADMIN_BASIC_AUTH_USER "$BACKEND_ENV_FILE" "")"
  admin_pass="$(read_env_value ADMIN_BASIC_AUTH_PASS "$BACKEND_ENV_FILE" "")"

  if [[ "$DRY_RUN" == "true" ]]; then
    log "DRY-RUN: API_BASE_URL=$api_base_url ADMIN_BASIC_AUTH_USER=$admin_user scripts/smoke-mvp.sh"
    return 0
  fi

  log "Running smoke test against ${api_base_url}"
  API_BASE_URL="$api_base_url" \
  ADMIN_BASIC_AUTH_USER="$admin_user" \
  ADMIN_BASIC_AUTH_PASS="$admin_pass" \
    "$ROOT_DIR/scripts/smoke-mvp.sh"
}

ensure_clean_worktree() {
  if [[ "$DRY_RUN" == "true" ]]; then
    return 0
  fi

  if ! git diff --quiet || ! git diff --cached --quiet; then
    die "Working tree is not clean. Commit/stash changes before deploy."
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --remote)
        [[ $# -ge 2 ]] || die "Missing value for --remote"
        REMOTE="$2"
        shift 2
        ;;
      --branch)
        [[ $# -ge 2 ]] || die "Missing value for --branch"
        BRANCH="$2"
        shift 2
        ;;
      --scope)
        [[ $# -ge 2 ]] || die "Missing value for --scope"
        case "$2" in
          auto|user|system) SCOPE="$2" ;;
          *) die "Invalid --scope value: $2 (expected auto|user|system)" ;;
        esac
        shift 2
        ;;
      --with-worker)
        WITH_WORKER="true"
        shift
        ;;
      --no-pull)
        DO_PULL="false"
        shift
        ;;
      --no-smoke)
        DO_SMOKE="false"
        shift
        ;;
      --dry-run)
        DRY_RUN="true"
        shift
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

main() {
  parse_args "$@"

  cd "$ROOT_DIR"

  require_cmd flock
  require_cmd git
  require_cmd make
  require_cmd npm
  require_cmd curl
  require_cmd systemctl

  [[ -f "$BACKEND_ENV_FILE" ]] || die "Backend env file not found: $BACKEND_ENV_FILE"

  exec 9>"$LOCK_FILE"
  if ! flock -n 9; then
    die "Another deploy is currently running (lock: $LOCK_FILE)"
  fi

  log "Starting deploy"
  log "Config: remote=$REMOTE branch=$BRANCH scope=$SCOPE with_worker=$WITH_WORKER pull=$DO_PULL smoke=$DO_SMOKE dry_run=$DRY_RUN"

  if [[ "$DO_PULL" == "true" ]]; then
    ensure_clean_worktree
    run git fetch "$REMOTE" "$BRANCH"
    run git pull --ff-only "$REMOTE" "$BRANCH"
  else
    log "Skipping git pull step (--no-pull)"
  fi

  if [[ -f "$ROOT_DIR/apps/web/package-lock.json" ]]; then
    run_subshell "cd '$ROOT_DIR/apps/web' && npm ci --no-audit --no-fund"
  else
    run_subshell "cd '$ROOT_DIR/apps/web' && npm install --no-audit --no-fund"
  fi

  run make backend-build web-build

  local resolved_scope
  resolved_scope="$(detect_scope)"

  local services=("ytd-api.service" "ytd-web.service")
  if [[ "$WITH_WORKER" == "true" ]]; then
    services+=("ytd-worker.service")
  fi

  restart_services "$resolved_scope" "${services[@]}"

  local api_base_url
  api_base_url="$(infer_api_base_url)"

  log "Waiting for API health on ${api_base_url}/healthz"
  wait_for_api_health "$api_base_url" 30 2 || die "API health check failed after restart"

  if [[ "$DO_SMOKE" == "true" ]]; then
    run_smoke "$api_base_url"
  else
    log "Skipping smoke test step (--no-smoke)"
  fi

  log "Deploy completed successfully"
}

main "$@"
