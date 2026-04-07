# 00. Project Overview

Language: English | [日本語](../specs_ja/00_project_overview.md)

## Document Set
This directory is the working specification set for **Symbol Super Node Program (SSNP) Basic Design v0.1**.

The purpose of this design is not to recreate a NEM-style high-performance node certification site.
The purpose is to build a registration and monitoring foundation that makes Symbol voting-node continuity,
reachability, and voting continuity visible, and that can later support reward allocation decisions.

## Positioning
SSNP is a **non-consensus external-layer program**.

It is:
- a network-stability support layer;
- initially external-funded or separately budgeted;
- focused on measurable operational quality;
- designed to remain useful even before any reward distribution starts.

It is not:
- a harvesting-reward redistribution mechanism;
- a protocol-level enforcement system;
- a CPU benchmark contest;
- a delegated popularity ranking.

## Fixed MVP Assumptions
- the target is Symbol mainnet voting-node operations;
- current-epoch voting-key validity matters more than raw machine performance;
- external monitoring is the primary source of truth;
- Program Agent execution is required for participation in this v0.1 design;
- self-reported metrics are supplemental and must not override probe data.

## File Map
- `00_project_overview.md`: scope, positioning, document map
- `01_problem_definition.md`: why SSNP exists
- `02_goals_and_non_goals.md`: goals, non-goals, excluded evaluation methods
- `03_program_architecture.md`: system components and data flow
- `04_registration_and_qualification.md`: participation requirements and qualification gates
- `05_scoring_and_ranking.md`: ranking model and measurement priorities
- `06_reward_model.md`: reward design constraints and allocation model
- `07_anti_concentration.md`: anti-capture rules
- `08_notifications_and_ops.md`: alerts and operational handling
- `09_governance_and_rollout.md`: rollout phases and governance boundary
- `10_open_questions.md`: unresolved design decisions
- `11_program_agent_design.md`: Program Agent responsibilities, heartbeat contract, and security boundaries
- `12_program_agent_policy_file.md`: static YAML policy file shape and compatibility rules

## Consistency Note
For v0.1, this repository now treats **Program Agent as required for SSNP participation**.
This is a program-participation requirement, not a requirement for Symbol node operation outside SSNP.
If a future version makes Program Agent optional, it must first define an equivalent low-trust,
low-operations alternative for liveness assurance and supplemental telemetry.
