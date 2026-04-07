[English](README.md) | 日本語

SSNP は、既存のハーベスト報酬を変更することなく、Voting Node 向けの
性能ベースランキング、寡占防止ルール、運用アラートを導入することで、
Symbol ネットワークの安定性向上を目指す提案である。

このリポジトリには、SIP ドラフト、図表、議論用ドキュメントを含む。

# Symbol Super Node Program (SSNP) SIP ドラフト

このリポジトリは、提案中の **Symbol Super Node Program (SSNP)** に関する
ドラフト一式を収録している。
**v0.1 では、SSNP 参加条件として Program Agent 実行を必須とする。**

## 言語運用方針

- `README.md` をデフォルトの入口とし、英語を基準文書として扱う。
- SIP と分割 spec は英語版を主系統とし、日本語版は翻訳または補助文書として維持する。
- 日本語文書が存在しない場合は、英語文書を参照する。
- GitHub の Issue / Pull Request テンプレートは英語のみで運用する。
- 英語版と日本語版に差異が出た場合は、1ファイル混在ではなく、どちらを正とするかを明示して修正する。

## Contents

- `README.md` — 英語版リポジトリ概要
- `README_ja.md` — 日本語版リポジトリ概要
- `.github/ISSUE_TEMPLATE/general.md` — 一般議論用 Issue テンプレート
- `.github/ISSUE_TEMPLATE/scoring.md` — スコアリング議論用 Issue テンプレート
- `.github/ISSUE_TEMPLATE/anti_concentration.md` — 寡占防止ルール議論用 Issue テンプレート
- `.github/PULL_REQUEST_TEMPLATE.md` — Pull Request テンプレート
- `sip/ssnp_sip.md` — 英語版 SIP ドラフト、主参照文書
- `sip/ssnp_sip_ja.md` — 日本語版 SIP ドラフト、翻訳・参考版
- `docs/community_explainer.md` — 英語版コミュニティ向け説明資料
- `docs/community_explainer_ja.md` — 日本語版コミュニティ向け説明資料
- `docs/faq.md` — 反対論点とカウンターをまとめた英語版 FAQ
- `docs/faq_ja.md` — 反対論点とカウンターをまとめた日本語版 FAQ
- `docs/specs/` — 英語版の基本設計 v0.1 分割仕様
- `docs/specs_ja/` — 日本語版の基本設計 v0.1 分割仕様
- `docs/diagrams/architecture.mmd` — Mermaid アーキテクチャ図
- `docs/diagrams/reward_flow.mmd` — Mermaid 報酬フロー図
- `docs/diagrams/anti_concentration.mmd` — Mermaid 寡占防止図
- `docs/diagrams/*.svg` — 各図の静的 SVG 版

## Positioning

SSNP は以下として位置づけるべきである。

- **ネットワーク安定化支援プログラム**
- コンセンサスルール変更ではない
- 既存ハーベスト報酬の削減ではない
- まずは外部資金による段階的パイロットとして扱う

## Governance Note

SSNP は、意図的に非コンセンサス型の外部レイヤープログラムとして設計されている。

以下を行ってはならない。

- ハーベスト報酬の変更
- トランザクション手数料の削減
- プロトコルレベルの強制執行の導入

プロトコル経済に関わる将来の変更は、別個のガバナンス議論を必要とする。

## Known Open Questions

このリポジトリでは、**v0.1 の Program Agent 必須要否は未解決ではない**。
これは SSNP 参加条件として固定であり、Symbol node 運用一般に対する必須要件ではない。

- 報酬原資の確定（最重要）
- スコア閾値の調整
- 寡占防止の証拠ルール
- 通知実装スコープ
- 監視インフラ自体の分散性
