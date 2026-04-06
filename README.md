SSNP is a proposal to improve Symbol network stability by introducing
performance-based ranking, anti-concentration rules, and operational alerts
for voting nodes — without modifying existing harvesting rewards.

This repository contains draft SIP documents, diagrams, and discussion materials.

# Symbol Super Node Program (SSNP) SIP Draft

This repository contains a draft package for the proposed **Symbol Super Node Program (SSNP)**.

## Contents

- `README.md` — repository overview
- `README_ja.md` — Japanese repository overview
- `.github/ISSUE_TEMPLATE/general.md` — general discussion issue template
- `.github/ISSUE_TEMPLATE/scoring.md` — scoring model discussion issue template
- `.github/ISSUE_TEMPLATE/anti_concentration.md` — anti-concentration issue template
- `.github/PULL_REQUEST_TEMPLATE.md` — pull request template
- `sip/ssnp_sip_en.md` — English SIP draft
- `sip/ssnp_sip_ja.md` — Japanese SIP draft
- `docs/community_explainer_en.md` — English community explainer
- `docs/community_explainer_ja.md` — Japanese community explainer
- `docs/faq_ja.md` — Japanese FAQ with objections and counters
- `docs/specs/` — working basic-design specification set
- `docs/diagrams/architecture.mmd` — Mermaid architecture diagram
- `docs/diagrams/reward_flow.mmd` — Mermaid reward flow diagram
- `docs/diagrams/anti_concentration.mmd` — Mermaid anti-concentration diagram
- `docs/diagrams/*.svg` — static SVG versions of the diagrams

## Positioning

SSNP should be presented as:

- a **network stability support program**
- **not** a change to consensus rules
- **not** a reduction of existing harvesting rewards
- a gradual, externally funded pilot first

## Governance Note

SSNP is intentionally designed as a non-consensus, external-layer program.

It must not:

- modify harvesting rewards
- reduce transaction fees
- introduce protocol-level enforcement

Any future changes involving protocol economics require separate governance discussion.

## Known Open Questions

- reward funding source (critical)
- scoring thresholds tuning
- anti-concentration evidence rules
- notification implementation scope
- monitoring infrastructure decentralization
