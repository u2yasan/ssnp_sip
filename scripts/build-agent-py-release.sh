#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
AGENT_DIR="$ROOT_DIR/agent_py"
DIST_DIR="$AGENT_DIR/dist"
RELEASE_DIR="$DIST_DIR/release"

cd "$AGENT_DIR"

python3 -m pip install --upgrade pip >/dev/null
python3 -m pip install build >/dev/null

rm -rf "$DIST_DIR"
python3 -m build

mkdir -p "$RELEASE_DIR"
cp "$DIST_DIR"/ssnp_agent-* "$RELEASE_DIR"/
cp "$AGENT_DIR/requirements-lock.txt" "$RELEASE_DIR"/
cp "$ROOT_DIR/agent/config.example.yaml" "$RELEASE_DIR"/config.example.yaml
cp "$ROOT_DIR/deploy/systemd/ssnp-agent.service" "$RELEASE_DIR"/

(
  cd "$RELEASE_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./* > SHA256SUMS
  else
    shasum -a 256 ./* > SHA256SUMS
  fi
)

printf '%s\n' "release bundle written to $RELEASE_DIR"
