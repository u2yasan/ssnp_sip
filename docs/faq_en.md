# SSNP FAQ: 10 Objections and Counters

Language: English | [日本語](faq_ja.md)

This FAQ organizes major objections that may be raised against SSNP
and provides practical counters for each.

SSNP is assumed to be a **non-consensus external-layer** program.
It does not modify existing harvesting rewards, transaction fees, or block rewards.

## 1. "Isn't this just harvesting-reward reduction under another name?"

No. SSNP explicitly assumes that **existing harvesting rewards are untouched**.

Counter:

- SSNP rewards are handled from a separate funding source.
- Consensus rewards and external program rewards are structurally different.
- Any proposal to reduce existing harvesting rewards is a separate governance issue, not part of the current SSNP draft.

If someone opposes SSNP on this basis, they are mixing up two different issues.

## 2. "If voting nodes get rewards, centralization will increase."

That happens when reward design is weak. SSNP is built to combine **performance conditions** and **anti-concentration conditions**.

Counter:

- It does not use simple equal distribution.
- Only nodes that satisfy Qualified conditions are considered.
- Reward eligibility is capped at one slot per Same Operator Group.
- Same-domain multi-slot capture is also restricted.
- Decentralization is part of the score so concentration is not structurally favored.

If someone raises centralization risk, the real question is whether they have a concrete alternative anti-capture rule set.

## 3. "Large operators will dominate and small operators will be pushed out."

That is only true in a badly designed system. SSNP is intentionally designed to reduce that outcome.

Counter:

- One operator should not be able to capture many slots.
- The design emphasizes continuity, monitoring response, and voting-key maintenance over raw hardware spending.
- Even before rewards, monitoring and alerts already provide value.

The problem is not that large operators exist.
The problem is allowing multiple nodes under the same operational control to capture multiple slots.

## 4. "Monitoring scores will be gamed. Operators will just fake low latency."

Correct. That is why single-metric scoring is not acceptable.

Counter:

- SSNP uses combined evaluation across availability, finalization tracking, chain sync consistency, and voting-key continuity.
- It is not a simple fastest-response contest.
- Multi-region monitoring reduces the value of optimizing for a single observation point.
- Proxy or relay-based score manipulation must be explicitly treated as a monitoring-design and anomaly-detection problem.

"This can be gamed" is not a reason to reject the program.
It is a reason to tighten the measurement design.

## 5. "Same Operator Group classification is subjective and dangerous."

Ignoring operational overlap is more dangerous. A system that refuses to detect shared control becomes trivial to exploit.

Counter:

- The draft uses multiple types of evidence: operator address, domain, managed provider, contact, certificate administration, and operational authority.
- It does not assume that one weak signal is enough in every case.
- The real issue is not whether classification exists, but whether evidence standards and appeal procedures are well defined.

The right response is to define evidence levels, review procedure, and re-evaluation flow.

## 6. "One slot per domain is too strict and will catch legitimate operators."

It is strict. That is not the same as wrong.

Counter:

- Domain restriction is not perfect identity proof; it is a practical filter against cheap multi-slot capture.
- It is intended to work together with Same Operator Group analysis, not alone.
- If it proves overly strict later, it can be relaxed. Starting too loose is harder to recover from.

Early-stage design should avoid the harder-to-repair failure mode: weak capture prevention.

## 7. "An external ranking has no legitimacy. It is just an arbitrary hierarchy."

This misunderstands the source of legitimacy.
SSNP does **not** claim protocol authority. It publishes external evaluation criteria.

Counter:

- It does not change consensus participation rights.
- It does not have node-exclusion authority.
- It is an external support program operating by public rules.

If the community rejects the ranking, its influence remains limited.
This is an explanation-and-transparency problem, not a coercive-power problem.

## 8. "Adding notifications will make operators dependent on the program."

Notifications are a support tool, not a control mechanism.

Counter:

- Notifications do not change consensus rules.
- Node operation remains possible without them.
- They reduce preventable failures such as voting-key expiry or certificate expiry.

If someone opposes notifications, they need to show a credible alternative operational discipline that prevents the same failures.

## 9. "If the monitoring infrastructure is concentrated, the whole program is self-contradictory."

That criticism is valid. This is one of the largest implementation risks in SSNP.

Counter:

- Monitoring decentralization is already listed as an open question.
- Multi-region probing should be a baseline requirement.
- If possible, measurement operators and data publication should also be decentralized.

The right conclusion is not "cancel SSNP."
The right conclusion is "do not allow centralized monitoring design."

## 10. "The priority should be increasing node count, not designing a program."

Node count alone is a crude target.

Counter:

- SSNP is designed to value continuity and independence, not raw count.
- Low-quality or commonly controlled nodes do not improve finalization resilience as much as they appear to.
- Monitoring, alerts, and ranking can produce more real operational value than simply increasing node count.

The real target is not "more nodes."
It is **more voting nodes that are stable and not operationally clustered**.

## Conclusion

Most serious objections reduce to two real issues:

- how reward funding should work
- how rigorous anti-concentration and monitoring design can be made

Most other objections confuse SSNP's purpose or boundary.

The real question is not whether SSNP should exist at all.
The real question is whether it can be designed to:

- avoid touching existing rewards,
- make operational quality visible,
- suppress concentration,
- reduce avoidable failures through monitoring and alerts.
