[日本語](README_ja.md) | English

# Program Agent

Minimal Go stub for the SSNP Program Agent.

## Quickstart

Generate a dedicated SSNP agent keypair:

```sh
go run ./cmd/program-agent --config ./config.example.yaml gen-key --out-dir ./keys
```

Use a config based on `config.example.yaml`, then run:

```sh
go run ./cmd/program-agent --config ./config.example.yaml enroll --challenge-id enroll-001
go run ./cmd/program-agent --config ./config.example.yaml run
go run ./cmd/program-agent --config ./config.example.yaml check --event-type registration --event-id check-001
go run ./cmd/program-agent --config ./config.example.yaml telemetry --warning-flag portal_unreachable
```

## Commands

- `gen-key`: writes `agent_private_key.pem` and `agent_public_key.pem` to `--out-dir` and prints their paths as JSON
- `enroll`: binds the local public key to a portal enrollment challenge
- `run`: fetches policy, performs recurring checks, and submits heartbeats
- `check`: runs bounded local hardware / CPU / disk checks and submits the result
- `telemetry`: submits warning telemetry explicitly

## Operational Constraints

- policy fetch is fail-closed; no embedded fallback exists
- broken `state.json` is not auto-repaired
- portal-side `4xx` / `5xx` errors are returned as-is
- telemetry auto-generation in v0.1 is limited to:
  - `portal_unreachable`
  - `local_check_execution_failed`
  - `voting_key_expiry_risk`
  - `certificate_expiry_risk`
- certificate checking inspects expiry metadata only; it does not validate CA or hostname trust

## Verification

Use `make test` and `make smoke` from the repository root when you want the shared repository-level checks.

```sh
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./...
```
