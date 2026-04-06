# 02. Goals and Non-Goals

## Goals
- make voting-node continuity visible to operators and the community;
- verify whether a node is currently eligible to participate in voting;
- monitor reachability, chain tracking, and finalization tracking;
- create a registration base that can later support reward allocation;
- reduce silent qualification loss from voting-key expiry or neglected operations;
- discourage concentration by the same operator or managed provider.

## Non-Goals
- modifying Symbol consensus rules;
- reducing existing harvesting rewards;
- changing transaction-fee economics;
- creating a CPU benchmark ranking;
- treating ICMP ping as a primary score input;
- making pass/fail decisions from a single monitoring point;
- paying all voting nodes equally regardless of quality;
- introducing protocol-level enforcement or on-chain punishment.

## Explicitly Excluded Evaluation Methods
These should be cut from the initial design rather than debated endlessly:

- self-reported CPU benchmark claims without external verification;
- ICMP ping as a headline quality metric;
- single-region single-point pass/fail judgment;
- vague "fast node" claims without finalization or sync evidence.

## Design Rule
If a metric does not improve confidence in current voting continuity,
it should not be promoted into the core qualification path.
