[日本語](README_ja.md) | English

# Portal

Minimal Go portal stub for local SSNP verification and API contract testing.

For testnet-oriented operator flow, use `nodes.testnet.example.yaml` and `../docs/testnet_runbook.md`.

## Quickstart

Local verification mode without SMTP:

```sh
go run ./cmd/portal-server \
  --listen 127.0.0.1:8080 \
  --policy ../docs/policies/program_agent_policy.v2026-04.yaml \
  --nodes-config ./nodes.example.yaml \
  --state-path ./portal-state.json \
  --nominal-daily-pool 1000 \
  --notifier-mode stdout
```

Default SMTP-backed mode:

```sh
SSNP_SMTP_PASSWORD=secret \
go run ./cmd/portal-server \
  --listen 127.0.0.1:8080 \
  --policy ../docs/policies/program_agent_policy.v2026-04.yaml \
  --nodes-config ./nodes.example.yaml \
  --state-path ./portal-state.json \
  --nominal-daily-pool 1000 \
  --email-to ops@example.invalid \
  --smtp-host smtp.example.invalid \
  --smtp-port 587 \
  --smtp-username ssnp-notify \
  --smtp-from ssnp@example.invalid
```

## Notifier Modes

- `smtp`: default; requires SMTP configuration and password
- `stdout`: writes notifications as JSON to stdout; intended for local verification
- `noop`: accepts notifications and drops them; intended for local verification only

## Minimum API Surface

- `GET /api/v1/agent/policy`
- `GET /api/v1/agent/telemetry`
- `GET /api/v1/rankings/{date_utc}`
- `GET /api/v1/reward-eligibility/{date_utc}`
- `GET /api/v1/anti-concentration-evidence/{date_utc}`
- `GET /api/v1/reward-allocations/{date_utc}`
- `GET /api/v1/public-node-status/{date_utc}`
- `GET /api/v1/operator-node-status/{node_id}/{date_utc}`
- `POST /api/v1/agent/enrollment-challenges`
- `POST /api/v1/agent/enroll`
- `POST /api/v1/agent/heartbeat`
- `POST /api/v1/agent/checks`
- `POST /api/v1/agent/telemetry`
- `POST /api/v1/decentralization-evidence`
- `POST /api/v1/domain-evidence`
- `POST /api/v1/operator-group-evidence`
- `POST /api/v1/shared-control-plane-evidence`

## Runtime Constraints

- known nodes come from `--nodes-config`
- runtime state persists to `--state-path`
- policy load is fail-closed
- broken snapshot JSON blocks startup
- smoke seed data is documented in `../testdata/smoke/README.md`
- qualification requires:
  - valid probe evidence
  - two valid heartbeats within 15 minutes
  - hardware check pass
  - voting-key evidence pass
  - 72-hour observation window

## Verification

Use `make test` from the repository root for normal regression checks and `make smoke` for the canonical minimum-working end-to-end path.

```sh
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./...
```
