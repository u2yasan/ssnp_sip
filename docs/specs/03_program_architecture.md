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
