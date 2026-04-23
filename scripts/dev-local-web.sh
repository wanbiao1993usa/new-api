#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
DEV_IMAGE="${DEV_IMAGE:-new-api-local:latest}"
DEV_CONTAINER_NAME="${DEV_CONTAINER_NAME:-new-api-dev}"
DEV_BACKEND_PORT="${DEV_BACKEND_PORT:-3001}"
BACKEND_BASE_URL="${BACKEND_BASE_URL:-http://127.0.0.1:${DEV_BACKEND_PORT}}"
BACKEND_STATUS_URL="${BACKEND_STATUS_URL:-${BACKEND_BASE_URL}/api/status}"
START_BACKEND="${START_BACKEND:-true}"
REBUILD_DEV_IMAGE="${REBUILD_DEV_IMAGE:-false}"

if ! command -v bun >/dev/null 2>&1; then
  echo "Bun is not available. Install Bun first: https://bun.sh" >&2
  exit 1
fi

cd "$ROOT_DIR"

backend_ready=false
if curl -fsS --max-time 2 "$BACKEND_STATUS_URL" >/dev/null 2>&1; then
  backend_ready=true
fi

if [[ "$backend_ready" == "true" ]]; then
  echo "Using existing dev backend: $BACKEND_STATUS_URL"
elif [[ "$START_BACKEND" == "false" ]]; then
  echo "Backend is not reachable: $BACKEND_STATUS_URL" >&2
  echo "Start a dev backend first, or run with START_BACKEND=true." >&2
  exit 1
else
  if command -v lsof >/dev/null 2>&1 && lsof -nP -iTCP:"$DEV_BACKEND_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port $DEV_BACKEND_PORT is already in use, but backend health check failed: $BACKEND_STATUS_URL" >&2
    echo "Check the existing process/container, or choose another DEV_BACKEND_PORT." >&2
    exit 1
  fi

  case "$(uname -m)" in
    arm64 | aarch64)
      expected_image_arch="arm64"
      target_platform="linux/arm64"
      ;;
    x86_64 | amd64)
      expected_image_arch="amd64"
      target_platform="linux/amd64"
      ;;
    *)
      expected_image_arch=""
      target_platform="linux/amd64"
      ;;
  esac

  image_exists=false
  if docker image inspect "$DEV_IMAGE" >/dev/null 2>&1; then
    image_exists=true
  fi

  image_arch=""
  if [[ "$image_exists" == "true" ]]; then
    image_arch="$(docker image inspect "$DEV_IMAGE" --format '{{.Architecture}}')"
  fi

  needs_image_build=false
  if [[ "$REBUILD_DEV_IMAGE" == "true" ]]; then
    needs_image_build=true
  elif [[ "$image_exists" != "true" ]]; then
    needs_image_build=true
  elif [[ -n "$expected_image_arch" && "$image_arch" != "$expected_image_arch" ]]; then
    needs_image_build=true
  fi

  image_rebuilt=false
  if [[ "$needs_image_build" == "true" ]]; then
    if [[ "$image_exists" != "true" ]]; then
      echo "Dev backend image does not exist; building it now: $DEV_IMAGE"
    elif [[ "$REBUILD_DEV_IMAGE" == "true" ]]; then
      echo "REBUILD_DEV_IMAGE=true; rebuilding dev backend image: $DEV_IMAGE"
    else
      echo "Dev backend image architecture is $image_arch, but this machine needs $expected_image_arch; rebuilding."
    fi

    PLATFORM="$target_platform" IMAGE="$DEV_IMAGE" EXPORT_IMAGE=false "$SCRIPT_DIR/build-docker.sh"
    image_rebuilt=true
  fi

  mkdir -p "$ROOT_DIR/data-dev" "$ROOT_DIR/logs-dev"

  container_exists=false
  if docker ps -a --format '{{.Names}}' | grep -qx "$DEV_CONTAINER_NAME"; then
    container_exists=true
  fi

  if [[ "$container_exists" == "true" && "$image_rebuilt" == "true" ]]; then
    echo "Recreating dev backend container to use rebuilt image: $DEV_CONTAINER_NAME"
    docker rm -f "$DEV_CONTAINER_NAME" >/dev/null
    container_exists=false
  fi

  if [[ "$container_exists" == "true" ]]; then
    echo "Starting existing dev backend container: $DEV_CONTAINER_NAME"
    docker start "$DEV_CONTAINER_NAME" >/dev/null
  else
    echo "Creating isolated dev backend container: $DEV_CONTAINER_NAME"
    docker run -d \
      --name "$DEV_CONTAINER_NAME" \
      -p "${DEV_BACKEND_PORT}:${DEV_BACKEND_PORT}" \
      -v "$ROOT_DIR/data-dev:/data" \
      -v "$ROOT_DIR/logs-dev:/app/logs" \
      -e PORT="$DEV_BACKEND_PORT" \
      -e SQL_DSN="" \
      -e REDIS_CONN_STRING="" \
      -e TZ=Asia/Shanghai \
      "$DEV_IMAGE" \
      --log-dir /app/logs >/dev/null
  fi

  echo "Waiting for dev backend: $BACKEND_STATUS_URL"
  for _ in {1..30}; do
    if curl -fsS --max-time 2 "$BACKEND_STATUS_URL" >/dev/null 2>&1; then
      backend_ready=true
      break
    fi
    sleep 1
  done

  if [[ "$backend_ready" != "true" ]]; then
    echo "Dev backend did not become ready. Check logs: docker logs --tail=100 $DEV_CONTAINER_NAME" >&2
    exit 1
  fi
fi

echo "Starting Vite dev server with hot reload."
echo "Open the frontend dev URL printed by Vite, usually http://localhost:5173"
echo "API requests are proxied to $BACKEND_BASE_URL by web/vite.config.js"

cd "$ROOT_DIR/web"
VITE_DEV_PROXY_TARGET="$BACKEND_BASE_URL" bun run dev
