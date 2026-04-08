# SSNP Python Agent Client

Recommended operator-side client for the SSNP Program Agent flow.

## Setup

```sh
python3 -m venv .venv
. .venv/bin/activate
pip install -e .
```

## Commands

```sh
python -m ssnp_agent --config ../agent/config.example.yaml gen-key --out-dir ../agent/keys
python -m ssnp_agent --config ../agent/config.example.yaml enroll --challenge-id enroll-001
python -m ssnp_agent --config ../agent/config.example.yaml run
python -m ssnp_agent --config ../agent/config.example.yaml check --event-type registration --event-id check-001
python -m ssnp_agent --config ../agent/config.example.yaml telemetry --warning-flag portal_unreachable
```

## Verification

```sh
python3 -m unittest discover -s tests -v
python3 -m compileall ssnp_agent
```
