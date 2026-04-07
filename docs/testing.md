# Testing

## Purpose

Use the repository root commands as follows.

- `make test`: regression check across repository test suites
- `make build`: compile both services
- `make smoke`: canonical minimum-working end-to-end verification

## Smoke

`make smoke` runs the Go end-to-end smoke test in `portal/internal/server`.

It verifies:

- portal startup
- agent enrollment
- heartbeat submission
- hardware check submission
- telemetry submission
- probe evidence ingestion
- voting-key evidence ingestion
- read API visibility for public and ranking views

Smoke seed data lives in `testdata/smoke/`.

- `policy.yaml`: fast-path policy for short smoke runtime
- `portal-state.json`: local verification seed that bypasses the normal 72-hour observation window

Do not treat smoke seed data as a production bootstrap example.

## Smoke Guard Rails

The repository includes dedicated smoke support tests:

- `TestSmokePolicyUsesExpectedFastPathSettings`
- `TestSmokeSeedMatchesNodesExampleConfig`
- `TestSmokeE2E`

These protect smoke assumptions before the full end-to-end path breaks.
