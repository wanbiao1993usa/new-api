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
SSH_CONTROL_PERSIST="${SSH_CONTROL_PERSIST:-10m}"
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
  find /tmp -maxdepth 1 -type f -name 'new-api-image*.tar.gz' -delete 2>/dev/null || true
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

  cleanup() {
    cleanup_ssh_connection_sharing "$host"
    if [[ "$cleanup_local_archive" == "true" ]]; then
      rm -f "$local_image_archive"
    fi
  }
  trap cleanup EXIT

  if [[ -z "$LOCAL_IMAGE_ARCHIVE" ]]; then
    log "cleaning stale local image archives in /tmp"
    cleanup_stale_temp_image_archives
  fi

  log "building image archive"
  build_image_archive "$local_image_archive"

  setup_ssh_connection_sharing "$host"

  log "checking remote prerequisites on $host"
  remote_preflight "$host" || die "remote host is missing docker/gzip or SSH access failed"

  log "uploading image archive to $host"
  scp "${SCP_ARGS[@]}" "$local_image_archive" "$host:$REMOTE_IMAGE_ARCHIVE"

  log "loading image on remote host"
  ssh "${SSH_ARGS[@]}" "$host" "gunzip -c '$REMOTE_IMAGE_ARCHIVE' | docker load && rm -f '$REMOTE_IMAGE_ARCHIVE'"

  log "recreating service on remote host"
  ssh "${SSH_ARGS[@]}" "$host" "cd '$remote_dir' && docker compose up -d --no-deps --force-recreate '$REMOTE_COMPOSE_SERVICE'"

  log "verifying remote service"
  ssh "${SSH_ARGS[@]}" "$host" "curl -fsS '$HEALTHCHECK_URL' >/dev/null"

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
  SSH_CONTROL_PERSIST=10m
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
