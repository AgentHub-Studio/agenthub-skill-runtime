#!/usr/bin/env bash
set -euo pipefail
GO_IMAGE="golang:1.25.12-alpine"
CACHE_VOL="$HOME/go/pkg/mod"
CMD="${1:-help}"
shift || true
case "$CMD" in
  compile)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" go build ./...
    ;;
  test)
    docker run --rm \
      -e CGO_ENABLED=1 \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -w /app \
      "${GO_IMAGE}" sh -c 'apk add --no-cache gcc musl-dev >/dev/null 2>&1 && go test -v -race -coverprofile=coverage.out ./... "$@"' sh "$@"
    ;;
  package)
    oci_created="${OCI_CREATED:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
    oci_revision="${OCI_REVISION:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}"
    oci_source="${OCI_SOURCE:-https://github.com/AgentHub-Studio/agenthub-skill-runtime}"
    oci_version="${OCI_VERSION:-local}"
    docker build \
      --build-arg "OCI_CREATED=${oci_created}" \
      --build-arg "OCI_REVISION=${oci_revision}" \
      --build-arg "OCI_SOURCE=${oci_source}" \
      --build-arg "OCI_VERSION=${oci_version}" \
      -t "agenthub-studio/agenthub-skill-runtime:local" .
    ;;
  lint)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      golangci/golangci-lint:latest golangci-lint run ./...
    ;;
  tidy)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" go mod tidy
    ;;
  *)
    echo "Usage: ./build.sh <compile|test|package|lint|tidy>"
    ;;
esac
