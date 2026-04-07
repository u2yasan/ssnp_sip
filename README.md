Language: English | [日本語](README_ja.md)

# SSNP SIP Draft

This repository contains:

- SSNP SIP and design documents
- `agent`: Program Agent stub
- `portal`: portal/API stub
- `probe`: external probe worker stub

## Entry Points

- repository overview and common smoke entry: `make smoke`
- agent usage: `agent/README.md`
- portal usage: `portal/README.md`
- probe usage: `probe/README.md`
- testing guide: `docs/testing.md`
- testnet operator flow: `docs/testnet_runbook.md`
- design overview: `docs/specs/00_project_overview.md`
- open questions: `docs/specs/10_open_questions.md`

## Minimum Verification

Use these commands from the repository root:

```sh
make smoke
make test
make build
```

Command roles:

- `make test`: regression check for the repository test suites
- `make build`: compile portal, agent, and probe worker
- `make smoke`: canonical minimum-working check; runs the Go end-to-end smoke flow

Smoke seed data is documented in `testdata/smoke/README.md`.

## Positioning

SSNP is:

- a network stability support program
- not a consensus change
- not a harvesting reward reduction
- intended to be useful even before reward distribution is active
