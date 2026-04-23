#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-${IMAGE:-new-api-local:latest}}"
TARGET_PLATFORM="${PLATFORM:-linux/amd64}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

platform_args=(--platform "$TARGET_PLATFORM")

cd "$ROOT_DIR"

echo "Building Docker image: $IMAGE"
echo "Using platform: $TARGET_PLATFORM"

docker build "${platform_args[@]}" -t "$IMAGE" .

echo "Built Docker image: $IMAGE"
echo "Start without rebuilding data volumes: docker compose up -d"
