#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  printf '%s\n' "usage: $0 /path/to/ssnp_agent-<version>-py3-none-any.whl" >&2
  exit 1
fi

WHEEL_PATH=$1
INSTALL_ROOT=/opt/ssnp-agent
VENV_PATH=$INSTALL_ROOT/.venv
CONFIG_DIR=/etc/ssnp-agent
STATE_DIR=/var/lib/ssnp-agent
SERVICE_NAME=ssnp-agent

if [ ! -f "$WHEEL_PATH" ]; then
  printf '%s\n' "wheel not found: $WHEEL_PATH" >&2
  exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' "run as root" >&2
  exit 1
fi

if ! getent group "$SERVICE_NAME" >/dev/null 2>&1; then
  groupadd --system "$SERVICE_NAME"
fi

if ! id "$SERVICE_NAME" >/dev/null 2>&1; then
  useradd --system --gid "$SERVICE_NAME" --home-dir "$INSTALL_ROOT" --shell /usr/sbin/nologin "$SERVICE_NAME"
fi

mkdir -p "$INSTALL_ROOT" "$CONFIG_DIR" "$CONFIG_DIR/keys" "$STATE_DIR"
python3 -m venv "$VENV_PATH"
"$VENV_PATH/bin/pip" install --upgrade pip
"$VENV_PATH/bin/pip" install "$WHEEL_PATH"

if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
  cp "$(dirname "$WHEEL_PATH")/config.example.yaml" "$CONFIG_DIR/config.yaml"
fi

if [ -f "$(dirname "$WHEEL_PATH")/ssnp-agent.service" ]; then
  cp "$(dirname "$WHEEL_PATH")/ssnp-agent.service" /etc/systemd/system/ssnp-agent.service
fi

chown -R "$SERVICE_NAME:$SERVICE_NAME" "$INSTALL_ROOT" "$CONFIG_DIR" "$STATE_DIR"

systemctl daemon-reload
systemctl enable ssnp-agent.service

printf '%s\n' "installed wheel into $VENV_PATH"
printf '%s\n' "edit $CONFIG_DIR/config.yaml before starting the service"
