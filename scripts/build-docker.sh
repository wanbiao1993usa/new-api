#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-${IMAGE:-new-api-local:latest}}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

platform_args=()
if [[ -n "${PLATFORM:-}" ]]; then
  platform_args=(--platform "$PLATFORM")
fi

cd "$ROOT_DIR"

echo "Building Docker image: $IMAGE"
if [[ ${#platform_args[@]} -gt 0 ]]; then
  echo "Using platform: $PLATFORM"
fi

docker build "${platform_args[@]}" -t "$IMAGE" .

echo "Built Docker image: $IMAGE"
echo "Start without rebuilding data volumes: docker compose up -d"
