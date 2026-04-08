# Testing

## Purpose

Use the repository root commands as follows.

- `make test`: regression check across repository test suites
- `make build`: compile portal and probe worker, and syntax-check the Python agent client
- `make smoke`: canonical minimum-working end-to-end verification
- `make testnet-local`: testnet-style local integration verification

## Smoke

`make smoke` runs the end-to-end smoke test in `portal/internal/server` using the Python agent client from `agent_py/`.

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

## Testnet-Operable Coverage

The repository now also includes a testnet-style verification path:

- `probe/internal/symbol/*`: Symbol REST parsing and lag derivation
- `probe/internal/worker/*`: probe submission integration against local fixture servers
- `portal/internal/server/TestTestnetOperableE2E`: portal + agent + probe worker flow against controlled local Symbol fixtures

This is the CI-safe substitute for hitting public testnet nodes.

Run it locally with:

```sh
make testnet-local
```

Command roles are intentionally separate:

- `make smoke`: minimum-working gate
- `make testnet-local`: local testnet-style integration gate
- `make test`: broader regression gate

## CI

GitHub Actions workflow `.github/workflows/go-test.yml` runs:

- `agent_py`: `python3 -m unittest discover -s tests -v` and `python3 -m compileall ssnp_agent`
- `portal`: `go test ./...` and `go build ./...`
- `probe`: `go test ./...` and `go build ./...`
- `smoke`: `make smoke`

It runs on:

- `pull_request` targeting `main`
- `push` to `main`

If local behavior and CI behavior diverge, treat the workflow as broken and fix the command path rather than introducing a second smoke path.
