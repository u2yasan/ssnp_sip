# Python Agent Distribution

## Purpose

`agent_py` is distributed as a private wheel bundle.

Do not deploy it by cloning the repository onto operator servers.
Do not use `pip install -e .` outside development.

## Release Artifact Shape

Build the release bundle from the repository root:

```sh
./scripts/build-agent-py-release.sh
```

The bundle is written to `agent_py/dist/release/` and contains:

- `ssnp_agent-<version>-py3-none-any.whl`
- `ssnp_agent-<version>.tar.gz`
- `requirements-lock.txt`
- `config.example.yaml`
- `ssnp-agent.service`
- `SHA256SUMS`

This bundle is the artifact to upload to a private GitHub Release.

## Server Install

Copy the release bundle to the server, then run:

```sh
sudo ./scripts/install-agent-py-wheel.sh /path/to/ssnp_agent-<version>-py3-none-any.whl
```

The installer creates:

- package root: `/opt/ssnp-agent`
- virtualenv: `/opt/ssnp-agent/.venv`
- config root: `/etc/ssnp-agent`
- state root: `/var/lib/ssnp-agent`
- runtime user/group: `ssnp-agent`

If `config.yaml` does not yet exist, the installer copies `config.example.yaml`
into `/etc/ssnp-agent/config.yaml`.

## Service Model

The bundled unit file runs:

```sh
/opt/ssnp-agent/.venv/bin/ssnp-agent --config /etc/ssnp-agent/config.yaml run
```

Before starting the service:

1. edit `/etc/ssnp-agent/config.yaml`
2. set `state_path` to `/var/lib/ssnp-agent/state.json`
3. place agent keys under `/etc/ssnp-agent/keys/`
4. update `agent_key_path` and `agent_public_key_path`

Then start:

```sh
sudo systemctl start ssnp-agent
sudo systemctl status ssnp-agent
```

## Upgrade And Rollback

Upgrade:

```sh
sudo systemctl stop ssnp-agent
sudo ./scripts/install-agent-py-wheel.sh /path/to/ssnp_agent-<new-version>-py3-none-any.whl
sudo systemctl start ssnp-agent
```

Rollback is the same operation with the older wheel.

## CI Release Path

The release workflow builds the bundle on version tags matching `agent-v*`.

It:

1. runs Python unit tests
2. builds the wheel and source tarball
3. assembles the release bundle
4. publishes the bundle files to GitHub Releases

If the release bundle format changes, update both:

- `scripts/build-agent-py-release.sh`
- `scripts/install-agent-py-wheel.sh`
