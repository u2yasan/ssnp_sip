#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  printf '%s\n' "usage: $0 /path/to/go-release-dir" >&2
  exit 1
fi

RELEASE_DIR=$1
TARGET_SUFFIX=${INSTALL_TARGET_SUFFIX:-linux-amd64}
PORTAL_USER=ssnp-portal
PROBE_USER=ssnp-probe
PORTAL_ROOT=/opt/ssnp-portal
PROBE_ROOT=/opt/ssnp-probe
PORTAL_CONFIG_DIR=/etc/ssnp-portal
PROBE_CONFIG_DIR=/etc/ssnp-probe
PORTAL_STATE_DIR=/var/lib/ssnp-portal

if [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' "run as root" >&2
  exit 1
fi

if [ ! -d "$RELEASE_DIR" ]; then
  printf '%s\n' "release dir not found: $RELEASE_DIR" >&2
  exit 1
fi

for required in \
  "portal-server-$TARGET_SUFFIX" \
  "probe-worker-$TARGET_SUFFIX" \
  program_agent_policy.v2026-04.yaml \
  nodes.testnet.example.yaml \
  probe.config.testnet.example.yaml \
  ssnp-portal.service \
  ssnp-probe.service
do
  if [ ! -f "$RELEASE_DIR/$required" ]; then
    printf '%s\n' "missing release file: $RELEASE_DIR/$required" >&2
    exit 1
  fi
done

if ! getent group "$PORTAL_USER" >/dev/null 2>&1; then
  groupadd --system "$PORTAL_USER"
fi

if ! id "$PORTAL_USER" >/dev/null 2>&1; then
  useradd --system --gid "$PORTAL_USER" --home-dir "$PORTAL_ROOT" --shell /usr/sbin/nologin "$PORTAL_USER"
fi

if ! getent group "$PROBE_USER" >/dev/null 2>&1; then
  groupadd --system "$PROBE_USER"
fi

if ! id "$PROBE_USER" >/dev/null 2>&1; then
  useradd --system --gid "$PROBE_USER" --home-dir "$PROBE_ROOT" --shell /usr/sbin/nologin "$PROBE_USER"
fi

mkdir -p "$PORTAL_ROOT/bin" "$PROBE_ROOT/bin" "$PORTAL_CONFIG_DIR" "$PROBE_CONFIG_DIR" "$PORTAL_STATE_DIR"

install -m 0755 "$RELEASE_DIR/portal-server-$TARGET_SUFFIX" "$PORTAL_ROOT/bin/portal-server"
install -m 0755 "$RELEASE_DIR/probe-worker-$TARGET_SUFFIX" "$PROBE_ROOT/bin/probe-worker"
install -m 0644 "$RELEASE_DIR/program_agent_policy.v2026-04.yaml" "$PORTAL_CONFIG_DIR/program_agent_policy.v2026-04.yaml"

if [ ! -f "$PORTAL_CONFIG_DIR/nodes.testnet.yaml" ]; then
  install -m 0644 "$RELEASE_DIR/nodes.testnet.example.yaml" "$PORTAL_CONFIG_DIR/nodes.testnet.yaml"
fi

if [ ! -f "$PROBE_CONFIG_DIR/config.yaml" ]; then
  install -m 0644 "$RELEASE_DIR/probe.config.testnet.example.yaml" "$PROBE_CONFIG_DIR/config.yaml"
fi

install -m 0644 "$RELEASE_DIR/ssnp-portal.service" /etc/systemd/system/ssnp-portal.service
install -m 0644 "$RELEASE_DIR/ssnp-probe.service" /etc/systemd/system/ssnp-probe.service

chown -R "$PORTAL_USER:$PORTAL_USER" "$PORTAL_ROOT" "$PORTAL_CONFIG_DIR" "$PORTAL_STATE_DIR"
chown -R "$PROBE_USER:$PROBE_USER" "$PROBE_ROOT" "$PROBE_CONFIG_DIR"

systemctl daemon-reload
systemctl enable ssnp-portal.service
systemctl enable ssnp-probe.service

printf '%s\n' "installed portal binary into $PORTAL_ROOT/bin"
printf '%s\n' "installed probe binary into $PROBE_ROOT/bin"
printf '%s\n' "edit $PORTAL_CONFIG_DIR/nodes.testnet.yaml and $PROBE_CONFIG_DIR/config.yaml before starting services"
