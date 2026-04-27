#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
DIST_DIR="$ROOT_DIR/dist"
RELEASE_DIR="$DIST_DIR/go-release"
CACHE_DIR="$ROOT_DIR/.cache/go-build"
MODCACHE_DIR="$ROOT_DIR/.cache/go-mod"
GOOS_TARGET=${GOOS_TARGET:-linux}
GOARCH_TARGET=${GOARCH_TARGET:-amd64}
TARGET_SUFFIX="${GOOS_TARGET}-${GOARCH_TARGET}"

rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR" "$CACHE_DIR" "$MODCACHE_DIR"

(
  cd "$ROOT_DIR/portal"
  env CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" GOCACHE="$CACHE_DIR" GOMODCACHE="$MODCACHE_DIR" \
    go build -trimpath -ldflags="-s -w" -o "$RELEASE_DIR/portal-server-$TARGET_SUFFIX" ./cmd/portal-server
)

(
  cd "$ROOT_DIR/probe"
  env CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" GOCACHE="$CACHE_DIR" GOMODCACHE="$MODCACHE_DIR" \
    go build -trimpath -ldflags="-s -w" -o "$RELEASE_DIR/probe-worker-$TARGET_SUFFIX" ./cmd/probe-worker
)

cp "$ROOT_DIR/docs/policies/program_agent_policy.v2026-04.yaml" "$RELEASE_DIR"/
cp "$ROOT_DIR/portal/nodes.testnet.example.yaml" "$RELEASE_DIR"/nodes.testnet.example.yaml
cp "$ROOT_DIR/probe/config.testnet.example.yaml" "$RELEASE_DIR"/probe.config.testnet.example.yaml
cp "$ROOT_DIR/deploy/systemd/ssnp-portal.service" "$RELEASE_DIR"/
cp "$ROOT_DIR/deploy/systemd/ssnp-probe.service" "$RELEASE_DIR"/
cp "$ROOT_DIR/scripts/install-go-release.sh" "$RELEASE_DIR"/

(
  cd "$RELEASE_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./* > SHA256SUMS
  else
    shasum -a 256 ./* > SHA256SUMS
  fi
)

printf '%s\n' "go release bundle written to $RELEASE_DIR"
