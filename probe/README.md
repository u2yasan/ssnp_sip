English | [日本語](README_ja.md)

# Probe Worker

Minimal external probe worker for SSNP testnet operation.

## Quickstart

```sh
go run ./cmd/probe-worker --config ./config.example.yaml run
```

Single cycle:

```sh
go run ./cmd/probe-worker --config ./config.example.yaml run-once
```

## Config Contract

- `portal_base_url`: portal base URL
- `region_id`: fixed region identifier for this worker instance
- `source_endpoint`: Symbol REST endpoint used as the reference chain height
- `request_timeout_seconds`: per-request timeout
- `poll_interval_seconds`: fixed polling interval and `measurement_window_seconds`
- `targets[].node_id`: node identifier already present in portal seed config
- `targets[].endpoint`: Symbol REST endpoint to probe

## Operating Boundary

- single-process worker only
- no queue, scheduler, or persistent retry state
- target failures are submitted as `availability_up = false`
- source endpoint failures abort the cycle instead of fabricating target-side failures

Use `docs/testnet_runbook.md` for the end-to-end operator flow.
