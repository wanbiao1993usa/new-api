#!/usr/bin/env bash
set -euo pipefail

shopt -s nullglob

SCRIPT_NAME="$(basename "$0")"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

DEPLOY_DIR_INPUT="${DEPLOY_DIR:-$PWD}"
if [[ -d "$DEPLOY_DIR_INPUT" ]]; then
  DEPLOY_DIR="$(cd -- "$DEPLOY_DIR_INPUT" && pwd)"
elif [[ "$DEPLOY_DIR_INPUT" = /* ]]; then
  DEPLOY_DIR="$DEPLOY_DIR_INPUT"
else
  DEPLOY_DIR="$PWD/$DEPLOY_DIR_INPUT"
fi
COMPOSE_FILE_INPUT="${COMPOSE_FILE:-docker-compose.yml}"
if [[ "$COMPOSE_FILE_INPUT" = /* ]]; then
  COMPOSE_FILE_PATH="$COMPOSE_FILE_INPUT"
else
  COMPOSE_FILE_PATH="$DEPLOY_DIR/$COMPOSE_FILE_INPUT"
fi

APP_SERVICE="${APP_SERVICE:-new-api}"
APP_CONTAINER_NAME="${APP_CONTAINER_NAME:-new-api}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
MYSQL_SERVICE="${MYSQL_SERVICE:-mysql}"
REDIS_SERVICE="${REDIS_SERVICE:-redis}"
STOP_APP_ON_EXPORT="${STOP_APP_ON_EXPORT:-true}"
STOP_TARGET_STACK_ON_IMPORT="${STOP_TARGET_STACK_ON_IMPORT:-true}"
EXPORT_ALL_IMAGES="${EXPORT_ALL_IMAGES:-true}"
EXPORT_APP_IMAGES_ONLY="${EXPORT_APP_IMAGES_ONLY:-false}"
EXTRA_BACKUP_PATHS="${EXTRA_BACKUP_PATHS:-}"
EXTRA_CONFIG_FILES="${EXTRA_CONFIG_FILES:-}"
INCLUDE_LOGS="${INCLUDE_LOGS:-true}"
FULL_STOP_AFTER_EXPORT="${FULL_STOP_AFTER_EXPORT:-false}"
FORCE_RESTORE_ABS_BINDS="${FORCE_RESTORE_ABS_BINDS:-true}"

COMPOSE_CMD=()

log() {
  printf '[%s] %s\n' "$SCRIPT_NAME" "$*"
}

warn() {
  printf '[%s] WARNING: %s\n' "$SCRIPT_NAME" "$*" >&2
}

die() {
  printf '[%s] ERROR: %s\n' "$SCRIPT_NAME" "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  scripts/server-migrate.sh export [bundle.tar.gz]
  scripts/server-migrate.sh import <bundle.tar.gz>

Description:
  Export creates a migration bundle for a Docker deployment on the source server.
  Import restores that bundle onto the target server.

Default assumptions:
  - Run the script from the deployment directory, or set DEPLOY_DIR.
  - The deployment uses docker compose and the app service name is new-api.
  - The compose file is docker-compose.yml unless COMPOSE_FILE is set.

Important environment variables:
  DEPLOY_DIR                  Deployment directory on the current server.
  COMPOSE_FILE                Compose file path (default: docker-compose.yml).
  APP_SERVICE                 App service name (default: new-api).
  APP_CONTAINER_NAME          Fallback app container name (default: new-api).
  POSTGRES_SERVICE            Postgres service name (default: postgres).
  MYSQL_SERVICE               MySQL service name (default: mysql).
  REDIS_SERVICE               Redis service name (default: redis).
  STOP_APP_ON_EXPORT          Stop app service before export (default: true).
  FULL_STOP_AFTER_EXPORT      Stop the full stack after export (default: false).
  STOP_TARGET_STACK_ON_IMPORT Stop target stack before restore (default: true).
  EXPORT_ALL_IMAGES           Export all images referenced by compose (default: true).
  EXPORT_APP_IMAGES_ONLY      Export only the app image (default: false).
  EXTRA_BACKUP_PATHS          Comma-separated extra host paths to archive.
  EXTRA_CONFIG_FILES          Comma-separated extra config files to archive.
  INCLUDE_LOGS                Include log mounts in the backup (default: true).
  FORCE_RESTORE_ABS_BINDS     Restore absolute bind mounts as-is (default: true).

Examples:
  ./scripts/server-migrate.sh export
  EXTRA_BACKUP_PATHS=/etc/nginx,/etc/letsencrypt ./scripts/server-migrate.sh export /tmp/new-api-migration.tar.gz
  DEPLOY_DIR=/srv/new-api ./scripts/server-migrate.sh import /tmp/new-api-migration.tar.gz
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD=(docker compose)
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD=(docker-compose)
    return
  fi
  die "docker compose is not available"
}

compose() {
  (
    cd "$DEPLOY_DIR"
    "${COMPOSE_CMD[@]}" -f "$COMPOSE_FILE_PATH" "$@"
  )
}

compose_service_exists() {
  local service="$1"
  compose config --services 2>/dev/null | grep -Fxq "$service"
}

container_id_for_service() {
  local service="$1"
  local cid
  cid="$(compose ps -q "$service" 2>/dev/null | head -n 1 || true)"
  if [[ -n "$cid" ]]; then
    printf '%s\n' "$cid"
    return
  fi
  cid="$(docker ps -aq --filter "label=com.docker.compose.service=$service" | head -n 1 || true)"
  printf '%s\n' "$cid"
}

container_id_for_name() {
  local name="$1"
  docker ps -aq --filter "name=^/${name}$" | head -n 1 || true
}

container_exists() {
  [[ -n "$1" ]] && docker inspect "$1" >/dev/null 2>&1
}

sanitize_name() {
  printf '%s' "$1" | tr '/: ' '---' | tr -cd 'A-Za-z0-9._-'
}

env_value_from_container() {
  local cid="$1"
  local key="$2"
  docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$cid" \
    | awk -F= -v key="$key" '$1 == key { print substr($0, index($0, "=") + 1) }' \
    | tail -n 1
}

detect_db_type() {
  local dsn="$1"
  if [[ -z "$dsn" || "$dsn" == local* ]]; then
    printf 'sqlite\n'
    return
  fi
  if [[ "$dsn" == postgres://* || "$dsn" == postgresql://* ]]; then
    printf 'postgres\n'
    return
  fi
  printf 'mysql\n'
}

csv_to_lines() {
  printf '%s' "$1" | tr ',' '\n' | sed '/^[[:space:]]*$/d'
}

shell_quote() {
  printf '%q' "$1"
}

copy_file_preserve_rel() {
  local source="$1"
  local base_dir="$2"
  local target_root="$3"
  local rel
  rel="${source#"$base_dir"/}"
  mkdir -p "$target_root/$(dirname "$rel")"
  cp -a "$source" "$target_root/$rel"
}

capture_default_config_files() {
  local dest_root="$1"
  local found=0
  local file

  for file in \
    "$DEPLOY_DIR"/docker-compose*.yml \
    "$DEPLOY_DIR"/docker-compose*.yaml \
    "$DEPLOY_DIR"/compose*.yml \
    "$DEPLOY_DIR"/compose*.yaml \
    "$DEPLOY_DIR"/.env \
    "$DEPLOY_DIR"/.env.*; do
    [[ -e "$file" ]] || continue
    copy_file_preserve_rel "$file" "$DEPLOY_DIR" "$dest_root"
    found=1
  done

  while IFS= read -r extra; do
    if [[ "$extra" = /* ]]; then
      [[ -e "$extra" ]] || { warn "extra config file not found: $extra"; continue; }
      local abs_dest="$dest_root/extra-absolute$(dirname "$extra")"
      mkdir -p "$abs_dest"
      cp -a "$extra" "$abs_dest/$(basename "$extra")"
    else
      local rel_file="$DEPLOY_DIR/$extra"
      [[ -e "$rel_file" ]] || { warn "extra config file not found: $rel_file"; continue; }
      copy_file_preserve_rel "$rel_file" "$DEPLOY_DIR" "$dest_root"
    fi
  done < <(csv_to_lines "$EXTRA_CONFIG_FILES")

  if [[ "$found" -eq 0 ]]; then
    warn "no compose or env files were found under $DEPLOY_DIR"
  fi
}

archive_bind_mount() {
  local source="$1"
  local destination="$2"
  local archive_path="$3"

  [[ -e "$source" ]] || { warn "bind mount source not found, skipping: $source"; return 1; }
  mkdir -p "$(dirname "$archive_path")"
  tar -C "$(dirname "$source")" -czf "$archive_path" "$(basename "$source")"
  log "archived bind mount $source -> $destination"
}

archive_named_volume() {
  local volume_name="$1"
  local archive_path="$2"

  mkdir -p "$(dirname "$archive_path")"
  docker run --rm \
    -v "$volume_name:/source:ro" \
    -v "$(dirname "$archive_path"):/backup" \
    alpine:3.20 \
    sh -c "cd /source && tar -czf /backup/$(basename "$archive_path") ."
  log "archived named volume $volume_name"
}

restore_bind_mount() {
  local source="$1"
  local relative_source="$2"
  local archive_path="$3"

  local restore_path="$source"
  if [[ -n "$relative_source" ]]; then
    restore_path="$DEPLOY_DIR/$relative_source"
  elif [[ "$FORCE_RESTORE_ABS_BINDS" != "true" ]]; then
    warn "skipping absolute bind mount restore because FORCE_RESTORE_ABS_BINDS=false: $source"
    return
  fi

  mkdir -p "$(dirname "$restore_path")"
  tar -C "$(dirname "$restore_path")" -xzf "$archive_path"
  log "restored bind mount into $restore_path"
}

restore_named_volume() {
  local volume_name="$1"
  local archive_path="$2"

  docker volume inspect "$volume_name" >/dev/null 2>&1 || docker volume create "$volume_name" >/dev/null
  docker run --rm \
    -v "$volume_name:/target" \
    -v "$(dirname "$archive_path"):/backup" \
    alpine:3.20 \
    sh -c "cd /target && tar -xzf /backup/$(basename "$archive_path")"
  log "restored named volume $volume_name"
}

write_manifest() {
  local manifest_path="$1"
  shift
  : >"$manifest_path"
  while (($#)); do
    printf '%s\n' "$1" >>"$manifest_path"
    shift
  done
}

load_manifest() {
  local manifest_path="$1"
  [[ -f "$manifest_path" ]] || die "manifest not found: $manifest_path"
  # shellcheck disable=SC1090
  source "$manifest_path"
}

collect_images_from_compose() {
  compose config 2>/dev/null | awk '/^[[:space:]]+image:/ {print $2}' | sort -u
}

export_images() {
  local bundle_dir="$1"
  local app_cid="$2"
  local images_file="$bundle_dir/images/docker-images.tar.gz"
  local image_list_file="$bundle_dir/images/image-list.txt"
  local app_image=""
  local images=()
  local image

  mkdir -p "$bundle_dir/images"

  if container_exists "$app_cid"; then
    app_image="$(docker inspect --format '{{.Config.Image}}' "$app_cid")"
  fi

  if [[ -n "$app_image" ]]; then
    images+=("$app_image")
  fi

  if [[ "$EXPORT_APP_IMAGES_ONLY" != "true" ]]; then
    while IFS= read -r image; do
      [[ -n "$image" ]] || continue
      images+=("$image")
    done < <(collect_images_from_compose)
  fi

  if [[ "$EXPORT_ALL_IMAGES" != "true" && ${#images[@]} -gt 0 ]]; then
    images=("${images[0]}")
  fi

  if [[ ${#images[@]} -eq 0 ]]; then
    warn "no images detected; skipping image export"
    return
  fi

  mapfile -t images < <(printf '%s\n' "${images[@]}" | awk '!seen[$0]++')
  printf '%s\n' "${images[@]}" >"$image_list_file"
  docker save "${images[@]}" | gzip >"$images_file"
  log "exported docker images to $images_file"
}

parse_mysql_dsn() {
  local dsn="$1"
  MYSQL_USER=""
  MYSQL_PASSWORD=""
  MYSQL_HOST=""
  MYSQL_PORT=""
  MYSQL_DB=""

  if [[ "$dsn" =~ ^([^:]+):([^@]+)@tcp\(([^:()]+)(:([0-9]+))?\)/([^?]+) ]]; then
    MYSQL_USER="${BASH_REMATCH[1]}"
    MYSQL_PASSWORD="${BASH_REMATCH[2]}"
    MYSQL_HOST="${BASH_REMATCH[3]}"
    MYSQL_PORT="${BASH_REMATCH[5]:-3306}"
    MYSQL_DB="${BASH_REMATCH[6]}"
    return 0
  fi

  if [[ "$dsn" =~ ^([^:]+):([^@]+)@([^:/]+)(:([0-9]+))?/([^?]+) ]]; then
    MYSQL_USER="${BASH_REMATCH[1]}"
    MYSQL_PASSWORD="${BASH_REMATCH[2]}"
    MYSQL_HOST="${BASH_REMATCH[3]}"
    MYSQL_PORT="${BASH_REMATCH[5]:-3306}"
    MYSQL_DB="${BASH_REMATCH[6]}"
    return 0
  fi

  return 1
}

dump_postgres() {
  local dsn="$1"
  local dump_path="$2"
  local pg_cid="$3"

  if container_exists "$pg_cid"; then
    docker exec "$pg_cid" pg_dump "$dsn" | gzip >"$dump_path"
  else
    docker run --rm --network host postgres:15 pg_dump "$dsn" | gzip >"$dump_path"
  fi
  log "created postgres dump $dump_path"
}

restore_postgres() {
  local dsn="$1"
  local dump_path="$2"
  local pg_cid="$3"

  if container_exists "$pg_cid"; then
    gunzip -c "$dump_path" | docker exec -i "$pg_cid" psql "$dsn"
  else
    gunzip -c "$dump_path" | docker run --rm -i --network host postgres:15 psql "$dsn"
  fi
  log "restored postgres dump from $dump_path"
}

dump_mysql() {
  local dsn="$1"
  local dump_path="$2"
  local mysql_cid="$3"

  parse_mysql_dsn "$dsn" || die "unable to parse MySQL SQL_DSN, please back up the database manually: $dsn"

  if container_exists "$mysql_cid"; then
    docker exec -e MYSQL_PWD="$MYSQL_PASSWORD" "$mysql_cid" \
      mysqldump --single-transaction -h 127.0.0.1 -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DB" \
      | gzip >"$dump_path"
  else
    docker run --rm --network host -e MYSQL_PWD="$MYSQL_PASSWORD" mysql:8.2 \
      mysqldump --single-transaction -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DB" \
      | gzip >"$dump_path"
  fi
  log "created MySQL dump $dump_path"
}

restore_mysql() {
  local dsn="$1"
  local dump_path="$2"
  local mysql_cid="$3"

  parse_mysql_dsn "$dsn" || die "unable to parse MySQL SQL_DSN, please restore the database manually: $dsn"

  if container_exists "$mysql_cid"; then
    gunzip -c "$dump_path" | docker exec -i -e MYSQL_PWD="$MYSQL_PASSWORD" "$mysql_cid" \
      mysql -h 127.0.0.1 -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DB"
  else
    gunzip -c "$dump_path" | docker run --rm -i --network host -e MYSQL_PWD="$MYSQL_PASSWORD" mysql:8.2 \
      mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DB"
  fi
  log "restored MySQL dump from $dump_path"
}

export_extra_paths() {
  local extra_dir="$1"
  local manifest="$2"
  local path archive_name

  mkdir -p "$extra_dir"
  : >"$manifest"
  while IFS= read -r path; do
    [[ -n "$path" ]] || continue
    [[ -e "$path" ]] || { warn "extra backup path not found: $path"; continue; }
    archive_name="$(sanitize_name "$path").tar.gz"
    tar -C "$(dirname "$path")" -czf "$extra_dir/$archive_name" "$(basename "$path")"
    printf '%s\t%s\n' "$path" "$archive_name" >>"$manifest"
    log "archived extra path $path"
  done < <(csv_to_lines "$EXTRA_BACKUP_PATHS")
}

restore_extra_paths() {
  local extra_dir="$1"
  local manifest="$2"
  local source_path archive_name

  [[ -f "$manifest" ]] || return
  while IFS=$'\t' read -r source_path archive_name; do
    [[ -n "$source_path" ]] || continue
    mkdir -p "$(dirname "$source_path")"
    tar -C "$(dirname "$source_path")" -xzf "$extra_dir/$archive_name"
    log "restored extra path $source_path"
  done <"$manifest"
}

export_mounts() {
  local app_cid="$1"
  local bundle_dir="$2"
  local mounts_manifest="$bundle_dir/mounts.tsv"
  local destination_root="$bundle_dir/mounts"
  local idx=0
  local type name source destination archive_name archive_path relative_source

  : >"$mounts_manifest"
  mkdir -p "$destination_root"

  while IFS=$'\t' read -r type name source destination; do
    [[ -n "$type" ]] || continue
    if [[ "$INCLUDE_LOGS" != "true" && "$destination" == *log* ]]; then
      continue
    fi

    archive_name="$(printf '%03d__%s.tar.gz' "$idx" "$(sanitize_name "$destination")")"
    relative_source=""
    if [[ "$type" == "bind" ]]; then
      archive_path="$destination_root/bind/$archive_name"
      if ! archive_bind_mount "$source" "$destination" "$archive_path"; then
        idx=$((idx + 1))
        continue
      fi
      if [[ "$source" == "$DEPLOY_DIR"/* ]]; then
        relative_source="${source#"$DEPLOY_DIR"/}"
      fi
    elif [[ "$type" == "volume" ]]; then
      archive_path="$destination_root/volume/$archive_name"
      archive_named_volume "$name" "$archive_path"
    else
      idx=$((idx + 1))
      continue
    fi

    printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$type" "$name" "$source" "$relative_source" "$destination" "$archive_name" >>"$mounts_manifest"
    idx=$((idx + 1))
  done < <(docker inspect --format '{{range .Mounts}}{{printf "%s\t%s\t%s\t%s\n" .Type .Name .Source .Destination}}{{end}}' "$app_cid")
}

restore_mounts() {
  local bundle_dir="$1"
  local mounts_manifest="$bundle_dir/mounts.tsv"
  local type name source relative_source destination archive_name archive_path

  [[ -f "$mounts_manifest" ]] || return

  while IFS=$'\t' read -r type name source relative_source destination archive_name; do
    [[ -n "$type" ]] || continue
    if [[ "$type" == "bind" ]]; then
      archive_path="$bundle_dir/mounts/bind/$archive_name"
      restore_bind_mount "$source" "$relative_source" "$archive_path"
    elif [[ "$type" == "volume" ]]; then
      archive_path="$bundle_dir/mounts/volume/$archive_name"
      restore_named_volume "$name" "$archive_path"
    fi
  done <"$mounts_manifest"
}

stop_app_for_export() {
  if [[ "$STOP_APP_ON_EXPORT" != "true" ]]; then
    log "STOP_APP_ON_EXPORT=false, leaving app service running"
    return
  fi

  if compose_service_exists "$APP_SERVICE"; then
    compose stop "$APP_SERVICE"
  else
    local app_cid
    app_cid="$(container_id_for_name "$APP_CONTAINER_NAME")"
    container_exists "$app_cid" && docker stop "$app_cid" >/dev/null
  fi
  log "application service stopped for export"
}

stop_stack_after_export() {
  if [[ "$FULL_STOP_AFTER_EXPORT" != "true" ]]; then
    return
  fi
  compose down
  log "full stack stopped after export"
}

stop_target_stack() {
  if [[ "$STOP_TARGET_STACK_ON_IMPORT" != "true" ]]; then
    return
  fi
  if [[ -f "$COMPOSE_FILE_PATH" ]]; then
    compose down || true
    log "target stack stopped before import"
  fi
}

start_db_dependencies_for_import() {
  local db_type="$1"
  local services=()

  if [[ "$db_type" == "postgres" ]] && compose_service_exists "$POSTGRES_SERVICE"; then
    services+=("$POSTGRES_SERVICE")
  fi
  if [[ "$db_type" == "mysql" ]] && compose_service_exists "$MYSQL_SERVICE"; then
    services+=("$MYSQL_SERVICE")
  fi
  if compose_service_exists "$REDIS_SERVICE"; then
    services+=("$REDIS_SERVICE")
  fi

  if [[ ${#services[@]} -gt 0 ]]; then
    compose up -d "${services[@]}"
  fi
}

wait_for_postgres() {
  local pg_cid="$1"
  local attempt

  container_exists "$pg_cid" || return
  for attempt in $(seq 1 60); do
    if docker exec "$pg_cid" pg_isready >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done
  die "postgres container did not become ready in time"
}

wait_for_mysql() {
  local mysql_cid="$1"
  local dsn="$2"
  local attempt

  container_exists "$mysql_cid" || return
  parse_mysql_dsn "$dsn" || die "unable to parse MySQL SQL_DSN while waiting for readiness: $dsn"
  for attempt in $(seq 1 60); do
    if docker exec -e MYSQL_PWD="$MYSQL_PASSWORD" "$mysql_cid" \
      mysqladmin ping -h 127.0.0.1 -P "$MYSQL_PORT" -u "$MYSQL_USER" --silent >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done
  die "mysql container did not become ready in time"
}

start_full_stack() {
  compose up -d
}

build_bundle_readme() {
  local path="$1"
  local bundle_name="$2"
  cat >"$path" <<EOF
Migration bundle: $bundle_name

Suggested target-server steps:
  1. Copy $bundle_name to the new server.
  2. Load the repo or copy scripts/server-migrate.sh to the new server.
  3. Run:
       DEPLOY_DIR=/path/to/deploy ./scripts/server-migrate.sh import /path/to/$bundle_name

If you also need to restore host-level files such as nginx or certificates, rerun export with:
  EXTRA_BACKUP_PATHS=/etc/nginx,/etc/letsencrypt
EOF
}

export_bundle() {
  local output_path="${1:-$DEPLOY_DIR/new-api-migration-$(date +%Y%m%d-%H%M%S).tar.gz}"
  local output_abs
  local work_dir bundle_name bundle_dir manifest_path readme_path
  local app_cid sql_dsn db_type db_dump_path
  local pg_cid mysql_cid

  require_cmd docker
  require_cmd tar
  require_cmd gzip
  detect_compose

  [[ -f "$COMPOSE_FILE_PATH" ]] || die "compose file not found: $COMPOSE_FILE_PATH"

  app_cid="$(container_id_for_service "$APP_SERVICE")"
  if ! container_exists "$app_cid"; then
    app_cid="$(container_id_for_name "$APP_CONTAINER_NAME")"
  fi
  container_exists "$app_cid" || die "app container not found; adjust APP_SERVICE or APP_CONTAINER_NAME"

  output_abs="$output_path"
  if [[ "$output_abs" != /* ]]; then
    output_abs="$PWD/$output_abs"
  fi

  work_dir="$(mktemp -d "${TMPDIR:-/tmp}/new-api-migration.XXXXXX")"
  bundle_name="$(basename "$output_abs" .tar.gz)"
  bundle_dir="$work_dir/$bundle_name"
  manifest_path="$bundle_dir/manifest.env"
  readme_path="$bundle_dir/README.txt"

  mkdir -p "$bundle_dir"/{config,dumps,mounts,images,extra}

  sql_dsn="$(env_value_from_container "$app_cid" "SQL_DSN" || true)"
  db_type="$(detect_db_type "$sql_dsn")"
  pg_cid="$(container_id_for_service "$POSTGRES_SERVICE")"
  mysql_cid="$(container_id_for_service "$MYSQL_SERVICE")"

  log "detected deployment directory: $DEPLOY_DIR"
  log "detected database type: $db_type"

  stop_app_for_export

  case "$db_type" in
    sqlite)
      ;;
    postgres)
      db_dump_path="$bundle_dir/dumps/postgres.sql.gz"
      dump_postgres "$sql_dsn" "$db_dump_path" "$pg_cid"
      ;;
    mysql)
      db_dump_path="$bundle_dir/dumps/mysql.sql.gz"
      dump_mysql "$sql_dsn" "$db_dump_path" "$mysql_cid"
      ;;
    *)
      die "unsupported database type: $db_type"
      ;;
  esac

  capture_default_config_files "$bundle_dir/config"
  export_mounts "$app_cid" "$bundle_dir"
  export_extra_paths "$bundle_dir/extra" "$bundle_dir/extra-paths.tsv"
  export_images "$bundle_dir" "$app_cid"
  build_bundle_readme "$readme_path" "$bundle_name.tar.gz"

  write_manifest "$manifest_path" \
    "BUNDLE_NAME=$(shell_quote "$bundle_name")" \
    "SOURCE_DEPLOY_DIR=$(shell_quote "$DEPLOY_DIR")" \
    "COMPOSE_FILE_BASENAME=$(shell_quote "$(basename "$COMPOSE_FILE_PATH")")" \
    "APP_SERVICE=$(shell_quote "$APP_SERVICE")" \
    "APP_CONTAINER_NAME=$(shell_quote "$APP_CONTAINER_NAME")" \
    "POSTGRES_SERVICE=$(shell_quote "$POSTGRES_SERVICE")" \
    "MYSQL_SERVICE=$(shell_quote "$MYSQL_SERVICE")" \
    "REDIS_SERVICE=$(shell_quote "$REDIS_SERVICE")" \
    "DB_TYPE=$(shell_quote "$db_type")" \
    "SQL_DSN=$(shell_quote "$sql_dsn")" \
    "INCLUDE_LOGS=$(shell_quote "$INCLUDE_LOGS")"

  tar -C "$work_dir" -czf "$output_abs" "$bundle_name"
  stop_stack_after_export
  rm -rf "$work_dir"

  log "migration bundle created: $output_abs"
  log "restore on target with: DEPLOY_DIR=/path/to/deploy $SCRIPT_NAME import $output_abs"
}

restore_config_files() {
  local bundle_dir="$1"
  local config_dir="$bundle_dir/config"
  local file rel

  [[ -d "$config_dir" ]] || return
  while IFS= read -r -d '' file; do
    rel="${file#"$config_dir"/}"
    mkdir -p "$DEPLOY_DIR/$(dirname "$rel")"
    cp -a "$file" "$DEPLOY_DIR/$rel"
  done < <(find "$config_dir" -type f -print0)
}

import_bundle() {
  local bundle_path="${1:-}"
  local bundle_abs temp_dir bundle_root manifest_path db_type sql_dsn
  local pg_cid mysql_cid

  [[ -n "$bundle_path" ]] || die "import requires a bundle path"
  require_cmd docker
  require_cmd tar
  require_cmd gzip
  detect_compose

  bundle_abs="$bundle_path"
  if [[ "$bundle_abs" != /* ]]; then
    bundle_abs="$PWD/$bundle_abs"
  fi
  [[ -f "$bundle_abs" ]] || die "bundle not found: $bundle_abs"

  temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/new-api-restore.XXXXXX")"
  tar -C "$temp_dir" -xzf "$bundle_abs"
  bundle_root="$(find "$temp_dir" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  [[ -n "$bundle_root" ]] || die "invalid bundle structure"

  manifest_path="$bundle_root/manifest.env"
  load_manifest "$manifest_path"
  db_type="${DB_TYPE:-sqlite}"
  sql_dsn="${SQL_DSN:-}"

  mkdir -p "$DEPLOY_DIR"
  restore_config_files "$bundle_root"

  if [[ -n "${COMPOSE_FILE_BASENAME:-}" ]]; then
    COMPOSE_FILE_PATH="$DEPLOY_DIR/$COMPOSE_FILE_BASENAME"
  fi
  [[ -f "$COMPOSE_FILE_PATH" ]] || die "restored compose file not found: $COMPOSE_FILE_PATH"

  stop_target_stack

  if [[ -f "$bundle_root/images/docker-images.tar.gz" ]]; then
    gunzip -c "$bundle_root/images/docker-images.tar.gz" | docker load
    log "docker images loaded"
  else
    warn "bundle does not contain docker images"
  fi

  restore_mounts "$bundle_root"
  restore_extra_paths "$bundle_root/extra" "$bundle_root/extra-paths.tsv"

  start_db_dependencies_for_import "$db_type"
  pg_cid="$(container_id_for_service "$POSTGRES_SERVICE")"
  mysql_cid="$(container_id_for_service "$MYSQL_SERVICE")"
  wait_for_postgres "$pg_cid"
  wait_for_mysql "$mysql_cid" "$sql_dsn"

  case "$db_type" in
    sqlite)
      ;;
    postgres)
      if [[ -f "$bundle_root/dumps/postgres.sql.gz" ]]; then
        restore_postgres "$sql_dsn" "$bundle_root/dumps/postgres.sql.gz" "$pg_cid"
      else
        warn "postgres deployment detected but dump file is missing"
      fi
      ;;
    mysql)
      if [[ -f "$bundle_root/dumps/mysql.sql.gz" ]]; then
        restore_mysql "$sql_dsn" "$bundle_root/dumps/mysql.sql.gz" "$mysql_cid"
      else
        warn "mysql deployment detected but dump file is missing"
      fi
      ;;
    *)
      die "unsupported database type in bundle: $db_type"
      ;;
  esac

  start_full_stack
  rm -rf "$temp_dir"
  log "import completed. Verify with: curl http://127.0.0.1:3000/api/status"
}

main() {
  local command="${1:-}"
  case "$command" in
    export)
      shift
      export_bundle "${1:-}"
      ;;
    import)
      shift
      import_bundle "${1:-}"
      ;;
    help|-h|--help|"")
      usage
      ;;
    *)
      die "unknown command: $command"
      ;;
  esac
}

main "$@"
