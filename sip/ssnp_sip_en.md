# SIP-XXXX: Symbol Super Node Program (SSNP)

## Status
Draft

## Author
TBD

## Type
Process / Ecosystem Incentive Proposal

## Created
2026-04-06

## Abstract

This proposal introduces the **Symbol Super Node Program (SSNP)**, a performance-based monitoring, ranking, and incentive framework designed to improve Symbol network stability, voting participation, and decentralization.

SSNP does **not** modify consensus rules, harvesting rewards, transaction fees, or block rewards. Instead, it operates as an **independent support layer** for high-quality voting node operators. The program evaluates nodes using externally verifiable operational metrics, applies anti-concentration controls, and distributes rewards to top-ranked qualified nodes from a separate funding pool.

In addition, SSNP provides operator alerts such as outage notifications, certificate expiry reminders, and voting key renewal reminders to reduce preventable voting drop-off.

## Motivation

Symbol finalization depends on sufficient participation from valid voting nodes. The network currently faces structural risks:

1. voting nodes receive no direct incentive tied to voting continuity;
2. voting keys can expire without timely renewal;
3. operational control may concentrate in a small number of parties or managed service providers;
4. geographic and infrastructure diversity may be insufficient;
5. there is no common, public, performance-based qualification framework.

A program that rewards **reliability, continuity, and decentralization** can improve network resilience without changing protocol-level economics.

## Goals

SSNP aims to:

1. increase the number of high-quality active voting nodes;
2. improve finalization resilience;
3. reduce silent drop-off from expired voting keys or neglected maintenance;
4. promote geographic and infrastructure diversity;
5. prevent reward capture by a single operator or managed provider;
6. create a transparent public ranking and operational quality signal.

## Non-Goals

SSNP does not:

- change Symbol consensus rules;
- slash nodes on-chain;
- guarantee rewards to all participants;
- replace harvesting incentives;
- reward popularity or delegated stake concentration.

## Definitions

### Qualified Node

A node is **Qualified** for a reward period only if it satisfies all minimum conditions:

- daily availability of at least 99.0%;
- finalized lag within threshold on at least 95% of valid probes;
- chain synchronization within threshold on at least 95% of valid probes;
- valid voting key for the relevant current epoch;
- current Program Agent heartbeat;
- no critical anomalies or confirmed abuse.

### Same Operator Group

A **Same Operator Group** is any set of nodes reasonably determined to be under materially overlapping operational control, including but not limited to:

- the same operator address;
- the same registrable domain;
- the same managed hosting or node operation service provider;
- the same contact or certificate administration;
- the same operational authority or delegated operations team.

For reward eligibility, nodes in the same Same Operator Group are treated as one group.

## System Overview

SSNP consists of:

- a public portal;
- a registration and challenge-signature flow;
- a multi-region external monitoring system;
- a Program Agent for heartbeat assurance and supplemental telemetry;
- a scoring and ranking engine;
- an operator notification system.

External monitoring is the primary source of truth. Program Agent data is supplemental and must not override contradictory external evidence.

## Registration

A participating operator must:

- operate on Symbol mainnet;
- hold a voting key valid for the current epoch;
- expose a monitored endpoint;
- run Program Agent;
- agree to the program terms.

An operator must also submit:

- operator address;
- node public identifier data;
- voting key data sufficient for validation;
- monitored endpoint information;
- contact information for alerts;
- signed challenge proving control;
- Program Agent linkage data.

A newly registered node enters an observation period of at least 72 hours before it can become Qualified.

The priority is not raw hardware strength in isolation, but whether the node is currently able to continue voting safely and observably.

## Scoring Model

The total node score is:

S = 0.7 * B + 0.3 * D

Where:

- `B` = Base Performance Score
- `D` = Decentralization Score

### Base Performance Score (70%)

Base Performance Score is composed of:

- Availability: 30
- Finalization tracking: 20
- Chain sync consistency: 10
- Voting key continuity: 10

### Decentralization Score (30%)

Decentralization Score is composed of:

- Geographic diversity: 15
- ASN / infrastructure diversity: 10
- Country concentration avoidance: 5

Decentralization is a modifier on qualified performance, not a substitute for minimum quality.

## Ranking

Qualified nodes are ranked by total score in descending order.

At most the top 50 nodes are considered reward-eligible for a reward period, subject to anti-concentration filtering.

Ties are resolved by:

1. higher finalization score;
2. higher availability score;
3. earlier validated registration time.

## Anti-Concentration Rules

### One Reward Slot per Same Operator Group

No more than one node from the same Same Operator Group may be selected as reward-eligible in a single reward period.

### One Reward Slot per Registrable Domain

No more than one node using the same registrable domain may be selected as reward-eligible in a single reward period.

### Managed Providers

Nodes operated by the same managed provider, node operation service, or delegated operational contractor are treated as part of the same Same Operator Group where material operational control overlaps.

If multiple nodes from the same group rank within the reward range, only the highest-ranked node remains reward-eligible and the remaining slots are backfilled by the next highest-ranked independent nodes.

## Reward Pool

SSNP uses an **independent funding source** and does not reduce existing harvesting rewards, fees, or block rewards.

Let `R_daily` be the nominal daily reward pool.

### Participation Adjustment

Let `N` be the number of Qualified nodes in the period.

The distributed portion of the nominal daily pool is adjusted as follows:

- N = 1–9  -> 30%
- N = 10–19 -> 50%
- N = 20–29 -> 70%
- N = 30–39 -> 85%
- N = 40–49 -> 95%
- N >= 50 -> 100%

Then:

`R_dist = R_daily * P(N)`

The undistributed amount is retained in reserve for future periods or approved program use.

This prevents early-stage overpayment and aligns incentives toward growing the number of qualified nodes.

## Reward Allocation by Rank Band

The distributed daily amount `R_dist` is allocated by rank band:

- Rank 1–5: 25%
- Rank 6–10: 20%
- Rank 11–20: 25%
- Rank 21–35: 18%
- Rank 36–50: 12%

Within each band, rewards are split evenly among eligible nodes in that band.

## Notifications

SSNP should provide registered operators with operational notifications, including:

- node outage alerts;
- finalized lag alerts;
- voting key expiry reminders;
- TLS certificate expiry reminders;
- domain expiry reminders;
- software update advisories;
- Program Agent heartbeat failure alerts.

The purpose of notifications is to reduce avoidable qualification loss and improve network continuity.

## Funding Principle

Initial deployment should use an external or separately approved program budget. A harvesting-reward reduction is out of scope for this draft.

## Deployment Plan

### Phase 1
Monitoring, registration, public ranking, and alerts only.

### Phase 2
Manual or semi-manual reward distribution from a separate budget.

### Phase 3
Automated reward calculation and distribution, subject to governance approval.

## Security Considerations

- external measurement is primary;
- signatures are required at registration;
- proxy or mirrored endpoint abuse should be monitored;
- Program Agent telemetry must not override contradictory external probe evidence;
- concentration detection must use multiple evidence sources rather than domain alone.

## Rationale

The program is intentionally designed to reward **network-supporting behavior**, not mere participation. It emphasizes measurable uptime, voting continuity, finalization support, and distribution across jurisdictions and infrastructure providers.

It also intentionally avoids rewarding delegated popularity, which would likely increase concentration rather than decentralization.

## Backward Compatibility

This proposal does not require protocol changes and is backward compatible with existing Symbol node operation.

## Reference Implementation

A reference implementation may include:

- registration portal;
- scoring engine;
- monitoring probes;
- notification service;
- public ranking UI.

## Conclusion

SSNP is a practical, non-consensus, decentralization-aware framework to strengthen Symbol’s operational resilience. It addresses real voting-node risks while minimizing conflict with existing harvesting incentives.
