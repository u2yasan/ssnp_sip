# 01. Problem Definition

## Primary Problem
Symbol currently depends on the continuity of valid voting nodes for stable finalization,
but lacks a shared operational framework that makes voting-node health visible and actionable.

## Current Operational Gaps
1. Voting-node continuity is important, but continuity-focused incentives are weak.
2. Voting keys can expire without timely renewal and silently remove effective voting participation.
3. There is no shared public qualification framework for node health.
4. Operational control can become concentrated behind domains, providers, or contractors.
5. Monitoring and notification practices differ widely by operator.

## Why Existing NEM Framing Does Not Transfer Cleanly
The old NEM Supernode Program emphasized controller-driven performance tests such as
computing power, ping, and bandwidth.

That is not the right center of gravity for Symbol.
For Symbol, the critical question is not "how powerful is the machine?"
The critical question is "can this node actually sustain valid voting participation now?"

## What Matters Most in Symbol
- valid voting key for the current epoch
- chain-following continuity
- finalization-following continuity
- endpoint reachability
- ongoing operational health

## Risk Statement
This is not primarily a reward-design problem.
It is a **network-operational resilience problem** with a future reward-allocation use case.
