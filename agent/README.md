[日本語](README_ja.md) | English

# Program Agent

Recommended operator client: Python package in `../agent_py`.

This Go implementation remains only as a reference stub during migration.
Do not treat it as the primary operator path.

## Quickstart

Generate a dedicated SSNP agent keypair:

```sh
cd ../agent_py
python3 -m venv .venv
. .venv/bin/activate
pip install -e .
python -m ssnp_agent --config ../agent/config.example.yaml gen-key --out-dir ../agent/keys
```

Use a config based on `config.example.yaml`, then run:

```sh
cd ../agent_py
. .venv/bin/activate
python -m ssnp_agent --config ../agent/config.example.yaml enroll --challenge-id enroll-001
python -m ssnp_agent --config ../agent/config.example.yaml run
python -m ssnp_agent --config ../agent/config.example.yaml check --event-type registration --event-id check-001
python -m ssnp_agent --config ../agent/config.example.yaml telemetry --warning-flag portal_unreachable
```

For testnet-oriented setup, start from `config.testnet.example.yaml` and follow `../docs/testnet_runbook.md`.

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
cd ../agent_py
python3 -m unittest discover -s tests -v
python3 -m compileall ssnp_agent
```
