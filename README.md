Language: English | [日本語](README_ja.md)

# SSNP SIP Draft

This repository contains:

- SSNP SIP and design documents
- `agent_py`: Python Program Agent client for operators
- `agent`: deprecated Go reference implementation
- `portal`: portal/API stub
- `probe`: external probe worker stub

## Entry Points

- repository overview and common smoke entry: `make smoke`
- Python agent usage: `agent/README.md`
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
- `make build`: compile portal and probe worker, and syntax-check the Python agent client
- `make smoke`: canonical minimum-working check; runs the end-to-end smoke flow with the Python agent client

Smoke seed data is documented in `testdata/smoke/README.md`.

## Positioning

SSNP is:

- a network stability support program
- not a consensus change
- not a harvesting reward reduction
- intended to be useful even before reward distribution is active
