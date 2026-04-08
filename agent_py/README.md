# SSNP Python Agent Client

Recommended operator-side client for the SSNP Program Agent flow.

## Setup

```sh
python3 -m venv .venv
. .venv/bin/activate
pip install --upgrade pip
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

## Private Wheel Distribution

Build release artifacts:

```sh
../scripts/build-agent-py-release.sh
```

The build output is written to `dist/release/` and includes:

- wheel
- source tarball
- `requirements-lock.txt`
- `config.example.yaml`
- `ssnp-agent.service`
- SHA256 checksums

Install a built wheel onto a server:

```sh
sudo ../scripts/install-agent-py-wheel.sh dist/release/ssnp_agent-0.1.0-py3-none-any.whl
```

The service file assumes:

- package install root: `/opt/ssnp-agent`
- config: `/etc/ssnp-agent/config.yaml`
- state: `/var/lib/ssnp-agent/state.json`
- runtime user/group: `ssnp-agent`

## Verification

```sh
python3 -m unittest discover -s tests -v
python3 -m compileall ssnp_agent
```
