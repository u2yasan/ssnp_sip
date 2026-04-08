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

- portal seed nodes: `portal/nodes.testnet.example.yaml`
- agent config: `agent/config.testnet.example.yaml`
- probe config: `probe/config.testnet.example.yaml`

Keep the same `node_id` across all three.

## Operator Flow

1. Add the node to the portal seed config.
2. Start the portal.
3. Generate agent keys.
4. Issue an enrollment challenge.
5. Enroll the agent.
6. Start the agent loop.
7. Start at least two probe-worker instances with different `region_id` values.
8. Confirm populated read views.

## Example Commands

Start the portal in dry-run notification mode:

```sh
cd portal
go run ./cmd/portal-server \
  --listen 127.0.0.1:8080 \
  --policy ../docs/policies/program_agent_policy.v2026-04.yaml \
  --nodes-config ./nodes.testnet.example.yaml \
  --state-path ./portal-state.json \
  --nominal-daily-pool 1000 \
  --notifier-mode stdout
```

Prepare the Python agent client:

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

Issue an enrollment challenge:

```sh
curl -sS \
  -H 'Content-Type: application/json' \
  -d '{"node_id":"node-testnet-001"}' \
  http://127.0.0.1:8080/api/v1/agent/enrollment-challenges
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

Start two probe workers. Duplicate the config and change only `region_id` per instance:

```sh
cd probe
go run ./cmd/probe-worker --config ./config.testnet.example.yaml run
```

Qualification requires evidence from at least two regions in one UTC day. One worker instance is not enough.

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
