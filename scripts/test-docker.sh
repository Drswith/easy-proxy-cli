#!/usr/bin/env bash
# Run unit + e2e tests inside an isolated Docker environment.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

IMAGE_TAG="${EZP_TEST_IMAGE:-easy-proxy-cli-test:local}"
TARGET="${1:-all}"

log() { printf '+ %s\n' "$*" >&2; }

log "building test image ${IMAGE_TAG}"
docker build -f docker/Dockerfile.test -t "${IMAGE_TAG}" .

case "$TARGET" in
  unit)
    log "running unit tests"
    docker run --rm -t "${IMAGE_TAG}" make test-unit
    ;;
  e2e)
    log "running e2e tests"
    docker run --rm -t -e SHELL=/bin/bash "${IMAGE_TAG}" make test-e2e
    ;;
  all|*)
    log "running unit + e2e + vet"
    docker run --rm -t -e SHELL=/bin/bash "${IMAGE_TAG}" make test-all
    ;;
esac

log "docker tests OK"
