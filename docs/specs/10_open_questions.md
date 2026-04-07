# 10. Open Questions

## Critical
1. What is the initial external funding source?
2. What evidence standard is sufficient for Same Operator Group classification?

## Resolved For v0.1
- External probe thresholds are fixed in the canonical policy file:
  - finalized lag: `<= 2` blocks
  - chain lag: `<= 5` blocks
- Program Agent is required for SSNP participation in v0.1.
- Whether Program Agent can become optional after v0.1 remains open, but only if an equivalent alternative for liveness assurance and supplemental telemetry is defined first.

## Secondary
3. Should ASN diversity be used as a hard cap or a scoring factor only?
4. What transparency level should raw scoring data have?
5. What reserve policy should be applied to undistributed rewards?
6. How much raw endpoint and probe data can be disclosed without creating operator risk?

## Out of Scope for Initial Version
- fee-based SSNP funding without separate governance approval
- delegated influence
- protocol rule changes
- score inputs based primarily on CPU or ICMP ping
