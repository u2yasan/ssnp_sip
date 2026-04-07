# 03. Program Architecture

## Core Components
- Registration Portal
- Program API
- Multi-region Probe Workers
- Program Agent
- Qualification Engine
- Scoring and Ranking Engine
- Public Portal UI
- Notification Engine
- Reward Eligibility Filter

## Source of Truth
External monitoring remains primary.
Program Agent data is supplemental, but Agent execution is still required in this v0.1 design
for participation, heartbeat assurance, and operational telemetry.

The system must never allow self-reported agent data to override contradictory external probe data.

## High-Level Data Flow
1. operator registers a node and agrees to program terms;
2. operator submits endpoint and node metadata;
3. operator proves control by signed challenge;
4. Program Agent is installed and linked to the registration;
5. the node enters the observation window;
6. probe workers collect external availability, sync, and finalization data;
7. the qualification engine evaluates baseline eligibility;
8. the scoring engine computes ranking among qualified nodes;
9. the anti-concentration filter determines reward eligibility;
10. the notification engine sends operational alerts.

## Architectural Requirements
- the system must work before reward distribution is activated;
- multi-region probing must be standard, not optional;
- failure of one probe region must not invalidate the entire evaluation set;
- qualification and reward-eligibility decisions must be separable.

## External Probe Data Model
External probe ingestion must use a two-stage model:
- raw probe events are stored as immutable observation records;
- daily node qualification summaries are derived from those raw events.

The Qualification Engine should consume the daily summary, not ad hoc probe reads.

Required raw-event properties:
- one event corresponds to one node, one region, one observed endpoint, and one observation timestamp;
- duplicate `probe_id` values must be rejected or treated as idempotent replays;
- negative lag values are invalid input;
- a probe region outage must reduce available evidence, not silently rewrite the node result.

Required daily-summary properties:
- aggregation window is one UTC day;
- availability uses all valid probe events in the window;
- finalized-lag compliance uses only valid and measurable finalized-lag events;
- chain-lag compliance uses only valid and measurable chain-lag events;
- insufficient multi-region evidence must be visible as insufficient evidence, not coerced into pass.
