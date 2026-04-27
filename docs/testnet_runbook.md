[日本語](testnet_runbook_ja.md) | English

# Testnet Runbook

## Purpose

This runbook defines the minimum supported path to operate the current SSNP stub against Symbol testnet.

This is rollout/proving infrastructure, not production hardening.

## Supported

- static node onboarding from portal seed config
- Program Agent enrollment and heartbeat
- simple external probe worker posting recurring probe events
- ranking, qualification, reward-eligibility, and reward-allocation read views
- SMTP notifications or `stdout` dry-run notifications

## Explicitly Deferred

- self-service registration UI or registration write API
- multi-region probe fleet management
- production-grade retry/error taxonomy
- webhook, Discord, or Telegram notification backends
- reward-funding governance completion
- deep validation hardening

## Required Files

- Go release bundle:
  - `portal-server-linux-amd64`
  - `probe-worker-linux-amd64`
  - `program_agent_policy.v2026-04.yaml`
  - `nodes.testnet.example.yaml`
  - `probe.config.testnet.example.yaml`
  - `ssnp-portal.service`
  - `ssnp-probe.service`
  - `install-go-release.sh`
  - `SHA256SUMS`
- agent config: `agent/config.testnet.example.yaml`

Keep the same `node_id` across the portal nodes config, agent config, and probe config.

## Operator Flow

1. Build the Go release bundle on CI or a development machine.
2. Copy the release bundle to the server.
3. Use `install-go-release.sh` to install portal/probe binaries and systemd units.
4. Add the node to the portal seed config.
5. Start the portal.
6. Generate agent keys.
7. Issue an enrollment challenge.
8. Enroll the agent.
9. Start the agent loop.
10. Start at least two probe-worker instances with different `region_id` values.
11. Confirm populated read views.

## Example Commands

Build the Go release bundle. Run this on CI or a development machine, not on the server:

```sh
./scripts/build-go-release.sh
```

Verify the bundle on the server:

```sh
cd /path/to/go-release
sha256sum -c SHA256SUMS
```

Install portal/probe binaries and systemd units on the server:

```sh
sudo ./install-go-release.sh /path/to/go-release
```

Edit the portal node seed:

```sh
sudo editor /etc/ssnp-portal/nodes.testnet.yaml
```

Edit the probe config. For multiple workers, change `region_id` per instance:

```sh
sudo editor /etc/ssnp-probe/config.yaml
```

Start the portal in dry-run notification mode. The current unit is fixed to `--notifier-mode stdout`:

```sh
sudo systemctl start ssnp-portal
sudo systemctl status ssnp-portal
```

The current `ssnp-portal.service` listens on `127.0.0.1:8080`. If agent/probe processes on other hosts must connect to it, use a TLS reverse proxy on the same host, a VPN, or an SSH tunnel. Do not bind the portal binary directly to the Internet.

Issue an enrollment challenge:

```sh
curl -sS \
  -H 'Content-Type: application/json' \
  -d '{"node_id":"node-testnet-001"}' \
  http://127.0.0.1:8080/api/v1/agent/enrollment-challenges
```

Prepare the Python agent client. Use wheel distribution for the agent side. Use editable install only for development:

```sh
cd agent_py
python3 -m venv .venv
. .venv/bin/activate
pip install -e .
```

Generate agent keys:

```sh
cd agent_py
. .venv/bin/activate
python -m ssnp_agent --config ../agent/config.testnet.example.yaml gen-key --out-dir ../agent/keys
```

Enroll the agent:

```sh
cd agent_py
. .venv/bin/activate
python -m ssnp_agent \
  --config ../agent/config.testnet.example.yaml \
  enroll \
  --challenge-id <challenge-id>
```

Start the agent loop:

```sh
cd agent_py
. .venv/bin/activate
python -m ssnp_agent --config ../agent/config.testnet.example.yaml run
```

Start the probe worker:

```sh
sudo systemctl start ssnp-probe
sudo systemctl status ssnp-probe
```

Qualification requires evidence from at least two regions in one UTC day. One worker instance is not enough. Install the same bundle on a second server and change only `/etc/ssnp-probe/config.yaml` `region_id` for the second instance.

## Validation Reads

Check:

- `GET /api/v1/public-node-status/{date_utc}`
- `GET /api/v1/operator-node-status/{node_id}/{date_utc}`
- `GET /api/v1/rankings/{date_utc}`
- `GET /api/v1/reward-eligibility/{date_utc}`
- `GET /api/v1/reward-allocations/{date_utc}`

## Notification Modes

- use `smtp` for actual operator mail delivery
- use `stdout` for dry-run verification
- do not use `noop` if you expect to observe alerts

## Operational Boundary

The current probe worker is intentionally minimal:

- one process
- fixed poll interval
- fixed request timeout
- no persisted retry queue
- source endpoint failure aborts the cycle
- target endpoint failure is posted as `availability_up = false`

If that boundary is unacceptable, this stub is not ready for your use case.
