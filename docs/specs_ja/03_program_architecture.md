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
- マルチリージョン証拠は定常要件だが、rollout 初期は専用 fleet ではなく複数の単純 worker instance から始めてよい
- 1リージョンの障害で評価全体が無効化されてはならない
- Qualified 判定と報酬適格判定は分離されていなければならない

## External Probe データモデル
external probe の取り込みは 2 段モデルで扱わなければならない。
- raw probe event は immutable な観測記録として保存する
- 日次の node qualification summary は raw event から導出する

Qualification Engine は、その場の probe 生読みに依存せず日次 summary を参照すべきである。

raw event の必須特性:
- 1 event は 1 node、1 region、1 observed endpoint、1 observation timestamp に対応する
- 重複 `probe_id` は reject するか idempotent replay として扱う
- 負の lag 値は不正入力として reject する
- 1 probe region の障害は evidence の欠落として扱い、node 結果を書き換えてはならない

日次 summary の必須特性:
- 集計窓は UTC の 1 日単位とする
- availability はその窓の全 valid probe event を母集団に使う
- finalized lag 適合率は valid かつ measurable な finalized lag event だけを母集団に使う
- chain lag 適合率は valid かつ measurable な chain lag event だけを母集団に使う
- マルチリージョン証拠が不足する場合は pass に丸めず、insufficient evidence として見えるようにする
