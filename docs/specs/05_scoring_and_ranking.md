# 05. Scoring and Ranking

## Design Priority
Scoring exists to rank already-qualified nodes.
It must not replace qualification gates.

## Total Score
S = 0.7 * B + 0.3 * D

Where:
- B = Base Performance Score
- D = Decentralization Score

## Base Performance Score (70%)
- Availability: 30
- Finalization tracking: 20
- Chain sync consistency: 10
- Voting key continuity: 10

## Decentralization Score (30%)
- Geographic diversity: 15
- ASN / infrastructure diversity: 10
- Country concentration avoidance: 5

## Ranking Rule
Qualified nodes are sorted in descending order by total score.

## Tie-Breakers
1. higher finalization score
2. higher availability score
3. earlier validated registration time

## Measurement Principles
- CPU claims are not a core score input;
- hardware simple check status is not a ranking multiplier;
- ICMP ping is not a core score input;
- single-region probe results must not dominate the score;
- finalization and sync tracking matter more than "fast response" optics.

## Constraint
Decentralization can improve the ranking of a Qualified node,
but cannot substitute for minimum operational quality.
