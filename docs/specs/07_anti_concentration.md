# 07. Anti-Concentration

## Why This Is Core
Without strong anti-concentration rules, SSNP degenerates into a reward-capture system
for large operators, hosting resellers, or managed providers.

## Same Operator Group
A Same Operator Group includes nodes reasonably determined to be under materially overlapping operational control, including:
- same operator address;
- same registrable domain;
- same managed provider;
- same operational contractor;
- shared certificate administration;
- materially overlapping operational authority.

## Reward Eligibility Limits
- max 1 reward-eligible node per Same Operator Group;
- max 1 reward-eligible node per registrable domain.

## Managed Providers
Managed node hosting providers and operational contractors are treated as the same group
where operational control materially overlaps.

## Backfill Rule
If multiple nodes from the same group rank inside the reward range,
only the highest-ranked node remains reward-eligible.
Remaining slots are backfilled by the next highest-ranked independent nodes.

## Evidence Principle
Domain alone is not sufficient evidence for all group-classification cases,
but same-domain exclusion is still retained as a hard reward-selection filter.

Current v0.1 portal stub additionally accepts shared-control-plane evidence
with classifications such as `managed_provider`, `operational_contractor`,
`shared_certificate_admin`, and `material_overlap`, and applies a hard
`max 1 reward-eligible node per control_plane_id` filter.

## Design Warning
The real risk is not that the rule is "too strict."
The real risk is allowing obvious multi-slot capture under different labels.
