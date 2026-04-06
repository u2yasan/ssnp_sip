# SSNP Community Explainer (English)

Language: English | [日本語](community_explainer_ja.md)

Status: English explainer for external sharing.

## One-line summary

SSNP is a **network stability support program** for high-quality Symbol voting nodes.

## Why this matters

Today, Symbol depends on voting-node continuity for finalization, but there is no standard public framework that:

- measures operator quality,
- encourages timely voting key renewal,
- discourages concentration under one operator or managed provider,
- helps operators avoid preventable failures.

## What SSNP does

SSNP introduces:

- public node registration,
- independent monitoring,
- performance-based ranking,
- anti-concentration filtering,
- operator alerts,
- and a separate reward pool for top qualified nodes.

## What SSNP does *not* do

SSNP does **not**:

- reduce existing harvesting rewards,
- change block rewards,
- change consensus rules,
- reward popularity or delegated stake concentration.

## Why not reward all voting nodes equally?

Equal distribution sounds fair, but it creates the wrong incentives:

- low-quality nodes get paid anyway;
- concentration remains;
- improvement pressure is weak.

SSNP instead rewards **qualified performance** and **decentralization**.

## Why anti-concentration matters

Without strong anti-concentration rules, one operator or one managed service provider could capture many reward slots. That would make the ranking look decentralized while leaving the network operationally fragile.

That is why the draft limits reward eligibility to one node per Same Operator Group and one node per registrable domain.

## Why alerts matter

A large amount of damage is preventable:

- voting key expiry,
- TLS certificate expiry,
- silent node downtime,
- synchronization lag.

SSNP should help operators stay qualified instead of only punishing them after failure.

## Initial rollout

Start with:

1. monitoring,
2. ranking,
3. alerts,

and only then move to a separate reward pool.

## Core message

SSNP is not primarily about giving out money.

It is about **reducing operational fragility in Symbol**.
