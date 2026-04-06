SSNP is a proposal to improve Symbol network stability by introducing
performance-based ranking, anti-concentration rules, and operational alerts
for voting nodes — without modifying existing harvesting rewards.

This repository contains draft SIP documents, diagrams, and discussion materials.

# Symbol Super Node Program (SSNP) SIP Draft

This repository contains a private-draft package for the proposed **Symbol Super Node Program (SSNP)**.

## Contents

- `README.md` — repository overview
- `sip/ssnp_sip_en.md` — English SIP draft
- `sip/ssnp_sip_ja.md` — Japanese SIP draft
- `docs/community_explainer_en.md` — English community explainer
- `docs/community_explainer_ja.md` — Japanese community explainer
- `docs/diagrams/architecture.mmd` — Mermaid architecture diagram
- `docs/diagrams/reward_flow.mmd` — Mermaid reward flow diagram
- `docs/diagrams/anti_concentration.mmd` — Mermaid anti-concentration diagram
- `docs/diagrams/*.svg` — static SVG versions of the diagrams

## Recommended private GitHub workflow

1. Create a **private** repository.
2. Upload this package as the initial commit.
3. Review the reward source section before sharing with others.
4. Open issues for:
   - reward source
   - scoring thresholds
   - anti-concentration evidence standard
   - notification channels
5. After internal review, publish a redacted public version if needed.

## Positioning

SSNP should be presented as:

- a **network stability support program**
- **not** a change to consensus rules
- **not** a reduction of existing harvesting rewards
- a gradual, externally funded pilot first
