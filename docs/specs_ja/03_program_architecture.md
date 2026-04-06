# 03. プログラム構成

## 中核コンポーネント
- Registration Portal
- Program API
- Multi-region Probe Workers
- Program Agent
- Qualification Engine
- Scoring and Ranking Engine
- Public Portal UI
- Notification Engine
- Reward Eligibility Filter

## 真実源
外部監視を主とする。
Program Agent データは補助情報だが、この v0.1 設計では
参加条件、heartbeat 保証、運用テレメトリのために Program Agent 実行を必須とする。

自己申告の Agent データが、矛盾する外部 probe データを上書きしてはならない。

## 高レベルデータフロー
1. オペレーターがノード登録し利用規約に同意する
2. endpoint とノードメタデータを提出する
3. 署名付き challenge で支配証明を行う
4. Program Agent を導入し登録へ紐付ける
5. ノードは観測期間に入る
6. probe worker が availability、sync、finalization を外部計測する
7. qualification engine が基礎参加条件を判定する
8. scoring engine が Qualified node の順位を算出する
9. anti-concentration filter が報酬対象を絞る
10. notification engine が運用通知を送る

## アーキテクチャ要件
- 報酬配布が未開始でもシステムは成立しなければならない
- マルチリージョン probe は標準であり optional ではない
- 1リージョンの障害で評価全体が無効化されてはならない
- Qualified 判定と報酬適格判定は分離されていなければならない
