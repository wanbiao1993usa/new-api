#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-${IMAGE:-new-api-local:latest}}"
SERVER_HOST="${SERVER_HOST:-43.160.196.110}"
SERVER_USER="${SERVER_USER:-root}"
SERVER="${SERVER:-${SERVER_USER}@${SERVER_HOST}}"
SSH_PORT="${SSH_PORT:-22}"
REMOTE_PROJECT_DIR="${REMOTE_PROJECT_DIR:-}"
REMOTE_COMPOSE_CMD="${REMOTE_COMPOSE_CMD:-docker compose}"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "Docker image not found locally: $IMAGE" >&2
  echo "Build it first: ./scripts/build-docker.sh $IMAGE" >&2
  exit 1
fi

echo "Pushing Docker image: $IMAGE"
echo "Target server: $SERVER"
echo "SSH port: $SSH_PORT"
echo "This script does not remove containers, volumes, or data."

docker save "$IMAGE" | gzip | ssh -p "$SSH_PORT" "$SERVER" "gunzip | docker load"

echo "Loaded Docker image on server: $IMAGE"

if [[ -n "$REMOTE_PROJECT_DIR" ]]; then
  remote_project_dir_quoted="$(printf "%q" "$REMOTE_PROJECT_DIR")"
  echo "Restarting only the new-api service in: $REMOTE_PROJECT_DIR"
  ssh -p "$SSH_PORT" "$SERVER" "cd $remote_project_dir_quoted && $REMOTE_COMPOSE_CMD up -d --no-deps --no-build new-api"
else
  echo "REMOTE_PROJECT_DIR is not set; skipped remote service restart."
  echo "To restart later on the server: docker compose up -d --no-deps --no-build new-api"
fi
