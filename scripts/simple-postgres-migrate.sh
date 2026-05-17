#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}"
ARG2="${2:-}"
ARG3="${3:-}"

DEPLOY_DIR="${DEPLOY_DIR:-/root/new-api}"
COMPOSE_FILE="${COMPOSE_FILE:-$DEPLOY_DIR/docker-compose.yml}"
APP_CONTAINER="${APP_CONTAINER:-new-api}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
REDIS_CONTAINER="${REDIS_CONTAINER:-redis}"
DB_NAME="${DB_NAME:-new-api}"
DB_USER="${DB_USER:-root}"
TARGET_DEPLOY_DIR="${TARGET_DEPLOY_DIR:-/root/new-api}"
TARGET_USER="${TARGET_USER:-root}"
SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-}"
REMOTE_SCRIPT_PATH="${REMOTE_SCRIPT_PATH:-/root/simple-postgres-migrate.sh}"
REMOTE_BUNDLE_PATH="${REMOTE_BUNDLE_PATH:-/root/new-api-migration.tar.gz}"
BUNDLE_PATH=""

SSH_ARGS=(-p "$SSH_PORT")
SCP_ARGS=(-P "$SSH_PORT")
if [[ -n "$SSH_KEY" ]]; then
  SSH_ARGS+=(-i "$SSH_KEY")
  SCP_ARGS+=(-i "$SSH_KEY")
fi

log() {
  printf '[simple-postgres-migrate] %s\n' "$*"
}

die() {
  printf '[simple-postgres-migrate] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

compose() {
  (
    cd "$DEPLOY_DIR"
    docker compose -f "$COMPOSE_FILE" "$@"
  )
}

remote_host() {
  local host="$1"
  if [[ "$host" == *"@"* ]]; then
    printf '%s\n' "$host"
  else
    printf '%s@%s\n' "$TARGET_USER" "$host"
  fi
}

resolve_bundle_path() {
  local bundle_path="$1"
  if [[ -z "$bundle_path" ]]; then
    bundle_path="/root/new-api-migration.tar.gz"
  fi
  printf '%s\n' "$bundle_path"
}

wait_for_postgres() {
  local i
  for i in $(seq 1 60); do
    if docker exec "$POSTGRES_CONTAINER" pg_isready -U "$DB_USER" >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done
  die "postgres did not become ready in time"
}

export_bundle() {
  local bundle_path="$1"
  local temp_dir bundle_dir app_image postgres_image redis_image
  local parent_dir deploy_name

  [[ -f "$COMPOSE_FILE" ]] || die "compose file not found: $COMPOSE_FILE"
  docker inspect "$APP_CONTAINER" >/dev/null 2>&1 || die "container not found: $APP_CONTAINER"
  docker inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1 || die "container not found: $POSTGRES_CONTAINER"

  temp_dir="$(mktemp -d /tmp/new-api-export.XXXXXX)"
  bundle_dir="$temp_dir/bundle"
  mkdir -p "$bundle_dir"

  app_image="$(docker inspect --format '{{.Config.Image}}' "$APP_CONTAINER")"
  postgres_image="$(docker inspect --format '{{.Config.Image}}' "$POSTGRES_CONTAINER")"
  redis_image="$(docker inspect --format '{{.Config.Image}}' "$REDIS_CONTAINER" 2>/dev/null || true)"

  log "stopping app container"
  compose stop "$APP_CONTAINER"

  log "dumping postgres database"
  docker exec "$POSTGRES_CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" | gzip >"$bundle_dir/postgres.sql.gz"

  parent_dir="$(dirname "$DEPLOY_DIR")"
  deploy_name="$(basename "$DEPLOY_DIR")"

  log "packing deployment directory"
  tar -C "$parent_dir" -czf "$bundle_dir/deploy-dir.tar.gz" "$deploy_name"

  log "exporting docker images"
  if [[ -n "$redis_image" ]]; then
    docker save "$app_image" "$postgres_image" "$redis_image" | gzip >"$bundle_dir/docker-images.tar.gz"
  else
    docker save "$app_image" "$postgres_image" | gzip >"$bundle_dir/docker-images.tar.gz"
  fi

  cat >"$bundle_dir/README.txt" <<EOF
Source deploy dir: $DEPLOY_DIR
Database: postgres
Bundle restore command:
  DEPLOY_DIR=$DEPLOY_DIR ./scripts/simple-postgres-migrate.sh import $bundle_path
EOF

  tar -C "$temp_dir" -czf "$bundle_path" bundle

  log "stopping full stack"
  compose down

  rm -rf "$temp_dir"
  log "bundle created: $bundle_path"
}

import_bundle() {
  local bundle_path="$1"
  local temp_dir bundle_dir parent_dir

  [[ -f "$bundle_path" ]] || die "bundle not found: $bundle_path"

  temp_dir="$(mktemp -d /tmp/new-api-import.XXXXXX)"
  tar -C "$temp_dir" -xzf "$bundle_path"
  bundle_dir="$temp_dir/bundle"

  log "loading docker images"
  gunzip -c "$bundle_dir/docker-images.tar.gz" | docker load

  parent_dir="$(dirname "$DEPLOY_DIR")"
  mkdir -p "$parent_dir"

  log "restoring deployment directory"
  tar -C "$parent_dir" -xzf "$bundle_dir/deploy-dir.tar.gz"

  [[ -f "$COMPOSE_FILE" ]] || die "restored compose file not found: $COMPOSE_FILE"

  log "starting postgres and redis"
  compose up -d "$POSTGRES_CONTAINER" "$REDIS_CONTAINER"
  wait_for_postgres

  log "resetting target database"
  docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'

  log "restoring postgres database"
  gunzip -c "$bundle_dir/postgres.sql.gz" | docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME"

  log "starting app"
  compose up -d "$APP_CONTAINER"

  rm -rf "$temp_dir"
  log "import completed"
  log "verify with: curl http://127.0.0.1:3000/api/status"
}

remote_preflight() {
  local host="$1"
  ssh "${SSH_ARGS[@]}" "$host" 'command -v docker >/dev/null && command -v tar >/dev/null && command -v gzip >/dev/null'
}

migrate_remote() {
  local target_host="$1"
  local bundle_path="$2"
  local host

  [[ -n "$target_host" ]] || die "target host is required for migrate mode"
  require_cmd ssh
  require_cmd scp

  host="$(remote_host "$target_host")"
  bundle_path="$(resolve_bundle_path "$bundle_path")"

  log "checking remote prerequisites on $host"
  remote_preflight "$host" || die "remote host is missing docker/tar/gzip or SSH access failed"

  log "exporting local bundle"
  export_bundle "$bundle_path"

  log "copying bundle to $host"
  scp "${SCP_ARGS[@]}" "$bundle_path" "$host:$REMOTE_BUNDLE_PATH"

  log "copying migration script to $host"
  scp "${SCP_ARGS[@]}" "$0" "$host:$REMOTE_SCRIPT_PATH"

  log "running remote import on $host"
  ssh "${SSH_ARGS[@]}" "$host" \
    "chmod +x '$REMOTE_SCRIPT_PATH' && DEPLOY_DIR='$TARGET_DEPLOY_DIR' '$REMOTE_SCRIPT_PATH' import '$REMOTE_BUNDLE_PATH'"

  log "remote migration completed"
  log "target verify with: ssh ${host} 'curl http://127.0.0.1:3000/api/status'"
}

usage() {
  cat <<EOF
Usage:
  ./scripts/simple-postgres-migrate.sh export [/root/new-api-migration.tar.gz]
  DEPLOY_DIR=/root/new-api ./scripts/simple-postgres-migrate.sh import /root/new-api-migration.tar.gz
  ./scripts/simple-postgres-migrate.sh migrate root@1.2.3.4 [/root/new-api-migration.tar.gz]

Optional env vars for migrate mode:
  TARGET_USER=root
  TARGET_DEPLOY_DIR=/root/new-api
  SSH_PORT=22
  SSH_KEY=/path/to/key.pem
EOF
}

main() {
  require_cmd docker
  require_cmd tar
  require_cmd gzip

  case "$MODE" in
    export)
      BUNDLE_PATH="$(resolve_bundle_path "$ARG2")"
      export_bundle "$BUNDLE_PATH"
      ;;
    import)
      BUNDLE_PATH="$(resolve_bundle_path "$ARG2")"
      import_bundle "$BUNDLE_PATH"
      ;;
    migrate)
      migrate_remote "$ARG2" "$ARG3"
      ;;
    ""|-h|--help|help)
      usage
      ;;
    *)
      die "unknown mode: $MODE"
      ;;
  esac
}

main
