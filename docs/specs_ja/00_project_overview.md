# 00. プロジェクト概要

[English](../specs/00_project_overview.md) | 日本語

## 文書セット
このディレクトリは **Symbol Super Node Program (SSNP) 基本設計 v0.1** の日本語版分割仕様である。

この設計の目的は、NEM型の高性能ノード認定サイトを再現することではない。
目的は、Symbol の voting node の継続性、到達性、投票継続性を可視化し、
将来の報酬配分根拠にも使える登録・監視基盤を作ることである。

## 位置づけ
SSNP は **非コンセンサス型の外部レイヤープログラム** である。

これは以下である。
- ネットワーク安定化支援レイヤー
- 初期段階では外部資金または別予算前提
- 測定可能な運用品質を重視する仕組み
- 報酬配布がなくても意味を持つ設計

これは以下ではない。
- ハーベスト報酬の再配分制度
- プロトコルレベルの強制執行システム
- CPU ベンチマーク競争
- 委任人気ランキング

## 固定する MVP 前提
- プログラム上の対象は Symbol mainnet の voting node 運用だが、rollout / proving のために実装を Symbol testnet で動かしてよい
- 生のマシン性能より current epoch に対する voting key 有効性を重視する
- 外部監視を主たる真実源とする
- この v0.1 設計では Program Agent 実行を参加条件とする
- 自己申告メトリクスは補助情報であり、probe データを上書きしてはならない

## ファイル一覧
- `00_project_overview.md`: スコープ、位置づけ、文書マップ
- `01_problem_definition.md`: SSNP が必要な理由
- `02_goals_and_non_goals.md`: 目的、非目的、除外する評価方式
- `03_program_architecture.md`: システム構成とデータフロー
- `04_registration_and_qualification.md`: 参加条件と Qualified 判定
- `05_scoring_and_ranking.md`: スコアリングと順位付けの考え方
- `06_reward_model.md`: 報酬設計制約と配分モデル
- `07_anti_concentration.md`: 寡占防止ルール
- `08_notifications_and_ops.md`: 通知と運用ルール
- `09_governance_and_rollout.md`: 段階導入とガバナンス境界
- `10_open_questions.md`: 未解決論点
- `11_program_agent_design.md`: Program Agent の責務、heartbeat 契約、セキュリティ境界
- `12_program_agent_policy_file.md`: 静的 YAML policy file の構造と互換性ルール

## 整合性メモ
このリポジトリでは、v0.1 における **SSNP 参加条件として Program Agent 必須** を正式前提とする。
これは SSNP 外の Symbol node 運用全般に対する必須要件ではない。
将来バージョンで optional 化するなら、heartbeat と補助テレメトリを置き換える、
同等に低信頼・低運用負荷の代替手段を先に定義しなければならない。
