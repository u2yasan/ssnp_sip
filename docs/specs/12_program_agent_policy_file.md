# 12. Program Agent Policy File

## Purpose
The Program Agent design defines policy-driven behavior such as heartbeat interval,
CPU and disk check profiles, hardware thresholds, probe thresholds, and reference-environment metadata.

Those values must exist as a concrete repo-managed policy file so implementers do not
hard-code local constants.

## Canonical File
The initial v0.1 policy file is:

- `docs/policies/program_agent_policy.v2026-04.yaml`

This file is the static source for:
- `GET /api/v1/agent/policy` responses;
- Program Agent local execution parameters;
- hardware simple check profile identifiers;
- policy-version validation during payload submission.

## Format
The policy file format is YAML.

Reasons:
- human-readable for operations and review;
- suitable for repo-managed versioning;
- aligned with the documentation-first nature of v0.1;
- easy to translate into database-backed storage later without changing wire semantics.

## Required Top-Level Fields
- `policy_version`
- `heartbeat_interval_seconds`
- `cpu_profile`
- `disk_profile`
- `hardware_thresholds`
- `probe_thresholds`
- `reference_environment`

## `cpu_profile`
Required fields:
- `id`
- `duration_seconds`
- `warmup_seconds`
- `measured_seconds`
- `cooldown_seconds`
- `worker_cap`
- `workload_mix`
- `acceptance_floor`

### `workload_mix`
Required fields:
- `hashing`
- `integer`
- `matrix`

The values are fractions that should sum to `1.00`.

### `acceptance_floor`
Required fields:
- `type`
- `minimum`

For v0.1:
- `type` must be `normalized_score`
- `minimum` is `1.00`

## `disk_profile`
Required fields:
- `id`
- `duration_seconds`
- `warmup_seconds`
- `measured_seconds`
- `cooldown_seconds`
- `block_size_bytes`
- `queue_depth`
- `concurrency`
- `read_ratio`
- `write_ratio`
- `acceptance_floor`

For v0.1:
- `block_size_bytes` is `4096`
- `queue_depth` is `32`
- `concurrency` is `4`
- `read_ratio` is `0.70`
- `write_ratio` is `0.30`
- `acceptance_floor.type` must be `measured_iops`
- `acceptance_floor.minimum` is `1500`

## `hardware_thresholds`
Required fields:
- `cpu_cores_min`
- `ram_gb_min`
- `storage_gb_min`
- `ssd_required`

For v0.1:
- `cpu_cores_min` is `8`
- `ram_gb_min` is `32`
- `storage_gb_min` is `750`
- `ssd_required` is `true`

## `probe_thresholds`
Required fields:
- `finalized_lag_max_blocks`
- `chain_lag_max_blocks`

For v0.1:
- `finalized_lag_max_blocks` is `2`
- `chain_lag_max_blocks` is `5`

These values are the canonical external-probe qualification limits for v0.1.
The agent and portal must consume the same repo-managed policy response rather
than duplicating local constants.

## `reference_environment`
Required fields:
- `id`
- `os_image_id`
- `agent_version`
- `cpu_profile_id`
- `disk_profile_id`
- `baseline_source_date`

This section exists to explain the meaning of `normalized_score >= 1.00`
and to prevent silent baseline drift.

## API and Validation Rules
- `GET /api/v1/agent/policy` should expose the active policy file semantics;
- portal must reject check payloads with mismatched `policy_version`;
- agent must not silently fall back to embedded defaults when the fetched policy is missing or incompatible;
- changing policy values requires a new `policy_version`.

## Compatibility Rule
The repo-managed YAML file is the initial storage format only.

If policy is later moved to a portal database:
- the field names must remain semantically compatible;
- the `GET /api/v1/agent/policy` response must preserve the same meaning;
- existing agent implementations must not need silent behavior changes.
