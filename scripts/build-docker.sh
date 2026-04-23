#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-${IMAGE:-new-api-local:latest}}"
TARGET_PLATFORM="${PLATFORM:-linux/amd64}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
ARCHIVE_NAME="$(printf "%s" "$IMAGE" | tr '/:' '--').tar.gz"
IMAGE_ARCHIVE="${IMAGE_ARCHIVE:-$ROOT_DIR/$ARCHIVE_NAME}"
EXPORT_IMAGE="${EXPORT_IMAGE:-true}"

platform_args=(--platform "$TARGET_PLATFORM")

cd "$ROOT_DIR"

echo "Building Docker image: $IMAGE"
echo "Using platform: $TARGET_PLATFORM"

docker build "${platform_args[@]}" -t "$IMAGE" .

echo "Built Docker image: $IMAGE"
if [[ "$EXPORT_IMAGE" == "true" ]]; then
  echo "Exporting Docker image archive: $IMAGE_ARCHIVE"
  docker save "$IMAGE" | gzip > "$IMAGE_ARCHIVE"
  echo "Exported Docker image archive: $IMAGE_ARCHIVE"
fi
echo "Start without rebuilding data volumes: docker compose up -d"
