#!/usr/bin/env bash
set -euo pipefail

TARGET_HOST="${1:-}"
IMAGE="${IMAGE:-new-api-local:latest}"
TARGET_PLATFORM="${PLATFORM:-linux/amd64}"
TARGET_USER="${TARGET_USER:-root}"
SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-}"
TARGET_DEPLOY_DIR="${TARGET_DEPLOY_DIR:-/root/new-api}"
REMOTE_IMAGE_ARCHIVE="${REMOTE_IMAGE_ARCHIVE:-/root/new-api-image.tar.gz}"
REMOTE_COMPOSE_SERVICE="${REMOTE_COMPOSE_SERVICE:-new-api}"
HEALTHCHECK_URL="${HEALTHCHECK_URL:-http://127.0.0.1:3000/api/status}"
LOCAL_IMAGE_ARCHIVE="${LOCAL_IMAGE_ARCHIVE:-}"
SSH_CONTROL_PERSIST="${SSH_CONTROL_PERSIST:-30m}"
HEALTHCHECK_MAX_ATTEMPTS="${HEALTHCHECK_MAX_ATTEMPTS:-30}"
HEALTHCHECK_INTERVAL_SECONDS="${HEALTHCHECK_INTERVAL_SECONDS:-2}"
REMOTE_LOG_TAIL_LINES="${REMOTE_LOG_TAIL_LINES:-100}"
STALE_LOCAL_IMAGE_MAX_AGE_DAYS="${STALE_LOCAL_IMAGE_MAX_AGE_DAYS:-1}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

SSH_ARGS=(-p "$SSH_PORT")
SCP_ARGS=(-P "$SSH_PORT")
if [[ -n "$SSH_KEY" ]]; then
  SSH_ARGS+=(-i "$SSH_KEY")
  SCP_ARGS+=(-i "$SSH_KEY")
fi

SSH_CONNECTION_DIR=""
SSH_CONTROL_PATH=""
SSH_CONNECTION_ACTIVE=false
CLEANUP_REMOTE_HOST=""
CLEANUP_LOCAL_IMAGE_ARCHIVE=""
CLEANUP_LOCAL_IMAGE_ENABLED=false

log() {
  printf '[deploy-remote] %s\n' "$*"
}

die() {
  printf '[deploy-remote] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

remote_host() {
  local host="$1"
  if [[ "$host" == *"@"* ]]; then
    printf '%s\n' "$host"
  else
    printf '%s@%s\n' "$TARGET_USER" "$host"
  fi
}

remote_preflight() {
  local host="$1"
  ssh "${SSH_ARGS[@]}" "$host" 'command -v docker >/dev/null && command -v gzip >/dev/null && command -v curl >/dev/null && docker compose version >/dev/null'
}

setup_ssh_connection_sharing() {
  local host="$1"

  SSH_CONNECTION_DIR="$(mktemp -d /tmp/new-api-ssh.XXXXXX)"
  SSH_CONTROL_PATH="$SSH_CONNECTION_DIR/control.sock"

  SSH_ARGS+=(
    -o "ControlMaster=auto"
    -o "ControlPersist=$SSH_CONTROL_PERSIST"
    -o "ControlPath=$SSH_CONTROL_PATH"
  )
  SCP_ARGS+=(
    -o "ControlMaster=auto"
    -o "ControlPersist=$SSH_CONTROL_PERSIST"
    -o "ControlPath=$SSH_CONTROL_PATH"
  )

  log "opening shared SSH connection to $host"
  ssh "${SSH_ARGS[@]}" -Nf "$host"
  SSH_CONNECTION_ACTIVE=true
}

cleanup_ssh_connection_sharing() {
  local host="$1"

  if [[ -z "$host" ]]; then
    return
  fi
  if [[ "$SSH_CONNECTION_ACTIVE" == "true" ]]; then
    ssh "${SSH_ARGS[@]}" -O exit "$host" >/dev/null 2>&1 || true
    SSH_CONNECTION_ACTIVE=false
  fi
  if [[ -n "$SSH_CONNECTION_DIR" ]]; then
    rm -rf "$SSH_CONNECTION_DIR"
    SSH_CONNECTION_DIR=""
    SSH_CONTROL_PATH=""
  fi
}

build_image_archive() {
  local archive_path="$1"
  (
    cd "$ROOT_DIR"
    IMAGE="$IMAGE" PLATFORM="$TARGET_PLATFORM" IMAGE_ARCHIVE="$archive_path" EXPORT_IMAGE=true \
      "$SCRIPT_DIR/build-docker.sh"
  )
}

make_temp_image_archive_path() {
  local temp_path
  temp_path="$(mktemp /tmp/new-api-image.XXXXXX)"
  rm -f "$temp_path"
  printf '%s.tar.gz\n' "$temp_path"
}

cleanup_stale_temp_image_archives() {
  find /tmp -maxdepth 1 -type f -name 'new-api-image*.tar.gz' -mtime +"$STALE_LOCAL_IMAGE_MAX_AGE_DAYS" -delete 2>/dev/null || true
}

print_remote_service_logs() {
  local host="$1"
  ssh "${SSH_ARGS[@]}" "$host" "cd '$TARGET_DEPLOY_DIR' && docker compose logs --tail='$REMOTE_LOG_TAIL_LINES' '$REMOTE_COMPOSE_SERVICE'" || true
}

wait_for_remote_healthcheck() {
  local host="$1"
  local attempt

  for attempt in $(seq 1 "$HEALTHCHECK_MAX_ATTEMPTS"); do
    if ssh "${SSH_ARGS[@]}" "$host" "curl -fsS '$HEALTHCHECK_URL' >/dev/null" >/dev/null 2>&1; then
      return 0
    fi
    if [[ "$attempt" -lt "$HEALTHCHECK_MAX_ATTEMPTS" ]]; then
      sleep "$HEALTHCHECK_INTERVAL_SECONDS"
    fi
  done
  return 1
}

cleanup() {
  cleanup_ssh_connection_sharing "$CLEANUP_REMOTE_HOST"
  if [[ "$CLEANUP_LOCAL_IMAGE_ENABLED" == "true" && -n "$CLEANUP_LOCAL_IMAGE_ARCHIVE" ]]; then
    rm -f "$CLEANUP_LOCAL_IMAGE_ARCHIVE"
  fi
}

deploy_remote() {
  local target_host="$1"
  local host local_image_archive remote_dir cleanup_local_archive

  [[ -n "$target_host" ]] || die "target host is required"

  host="$(remote_host "$target_host")"
  local_image_archive="$LOCAL_IMAGE_ARCHIVE"
  cleanup_local_archive=false
  if [[ -z "$local_image_archive" ]]; then
    local_image_archive="$(make_temp_image_archive_path)"
    cleanup_local_archive=true
  fi
  remote_dir="$TARGET_DEPLOY_DIR"

  require_cmd docker
  require_cmd ssh
  require_cmd scp

  CLEANUP_REMOTE_HOST="$host"
  CLEANUP_LOCAL_IMAGE_ARCHIVE="$local_image_archive"
  CLEANUP_LOCAL_IMAGE_ENABLED="$cleanup_local_archive"
  trap cleanup EXIT

  if [[ -z "$LOCAL_IMAGE_ARCHIVE" ]]; then
    log "cleaning stale local image archives in /tmp older than $STALE_LOCAL_IMAGE_MAX_AGE_DAYS day(s)"
    cleanup_stale_temp_image_archives
  fi

  setup_ssh_connection_sharing "$host"

  log "checking remote prerequisites on $host"
  remote_preflight "$host" || die "remote host is missing docker/gzip/curl or SSH access failed"

  log "building image archive"
  build_image_archive "$local_image_archive"

  log "uploading image archive to $host"
  scp "${SCP_ARGS[@]}" "$local_image_archive" "$host:$REMOTE_IMAGE_ARCHIVE"

  log "loading image on remote host"
  ssh "${SSH_ARGS[@]}" "$host" "gunzip -c '$REMOTE_IMAGE_ARCHIVE' | docker load && rm -f '$REMOTE_IMAGE_ARCHIVE'"

  log "recreating service on remote host"
  ssh "${SSH_ARGS[@]}" "$host" "cd '$remote_dir' && docker compose up -d --no-deps --force-recreate '$REMOTE_COMPOSE_SERVICE'"

  log "verifying remote service"
  if ! wait_for_remote_healthcheck "$host"; then
    log "healthcheck failed; remote service logs:"
    print_remote_service_logs "$host"
    die "remote healthcheck failed: $HEALTHCHECK_URL"
  fi

  log "remote deploy completed"
}

usage() {
  cat <<EOF
Usage:
  ./scripts/deploy-remote.sh root@1.2.3.4

Optional env vars:
  IMAGE=new-api-local:latest
  PLATFORM=linux/amd64
  TARGET_USER=root
  SSH_PORT=22
  SSH_KEY=/path/to/key.pem
  SSH_CONTROL_PERSIST=30m
  HEALTHCHECK_MAX_ATTEMPTS=30
  HEALTHCHECK_INTERVAL_SECONDS=2
  REMOTE_LOG_TAIL_LINES=100
  STALE_LOCAL_IMAGE_MAX_AGE_DAYS=1
  TARGET_DEPLOY_DIR=/root/new-api
  REMOTE_IMAGE_ARCHIVE=/root/new-api-image.tar.gz
  REMOTE_COMPOSE_SERVICE=new-api
  HEALTHCHECK_URL=http://127.0.0.1:3000/api/status
  LOCAL_IMAGE_ARCHIVE=/tmp/new-api-image.tar.gz
EOF
}

main() {
  case "${TARGET_HOST:-}" in
    ""|-h|--help|help)
      usage
      ;;
    *)
      deploy_remote "$TARGET_HOST"
      ;;
  esac
}

main "$@"
