# Smoke Test Data

This directory contains local verification seed data for the canonical smoke flow.

## Purpose

These files exist only to keep `make smoke` fast and deterministic.

- `policy.yaml`: a short-duration smoke policy so the end-to-end test finishes quickly
- `portal-state.json`: a portal runtime snapshot seed that pre-populates `validated_registration_at`

## What This Bypasses

`portal-state.json` sets `validated_registration_at` to an old timestamp so the smoke flow does not need to wait for the normal 72-hour observation window.

This is a local verification shortcut only.

## Constraints

- this data is not a production bootstrap example
- this data must stay consistent with `portal/nodes.example.yaml`
- `portal-state.json` assumes the smoke node id is `node-abc`
- changing the smoke node seed without updating the test and example config will break `make smoke`

## Maintenance Rule

If smoke behavior changes, update these files first and then adjust:

- `portal/internal/server/smoke_e2e_test.go`
- `README.md`
- `README_ja.md`
- `portal/README.md`
- `portal/README_ja.md`
