# 06. Reward Model

## Design Boundary
The monitoring and registration foundation must exist even if rewards are not yet active.
Reward design is downstream from qualification, not the other way around.

## Funding Principle
Initial SSNP deployment uses an independent external funding source
or a separately approved program budget.

## Hard Constraint
The initial design MUST NOT reduce:
- existing harvesting rewards;
- transaction fees;
- block rewards.

## Daily Pool
R_daily = nominal daily pool

## Participation Adjustment
Let N = number of Qualified nodes.

P(N):
- 1–9 -> 30%
- 10–19 -> 50%
- 20–29 -> 70%
- 30–39 -> 85%
- 40–49 -> 95%
- 50+ -> 100%

R_dist = R_daily * P(N)

Undistributed value remains in reserve.

## Rank Band Allocation
- Rank 1–5: 25%
- Rank 6–10: 20%
- Rank 11–20: 25%
- Rank 21–35: 18%
- Rank 36–50: 12%

## Reward Logic
- only Qualified nodes enter the ranking set;
- anti-concentration filtering is applied before final reward eligibility;
- equal payment to all voting nodes is intentionally rejected.

## Caution
If funding discussions move toward protocol-economics sources,
that is no longer a pure reward-parameter change.
It becomes a governance-boundary issue.
