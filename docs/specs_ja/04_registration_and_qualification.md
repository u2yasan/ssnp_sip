# 04. 登録と Qualified 判定

## 参加条件
参加ノードは最低でも以下を満たす。

- Symbol mainnet 上で運用されていること
- current epoch に有効な voting key を持つこと
- プログラム指定の monitored endpoint を公開していること
- Program Agent を実行していること
- プログラム利用規約に同意していること

## 登録入力
- operator address
- node identification data
- voting-key validation data
- endpoint information
- alert contact
- signed registration challenge
- Program Agent linkage data

## Qualified 判定の思想
重要なのは「紙の上で強いマシン」ではない。
重要なのは「そのノードが今、安全かつ可観測な voting を継続できること」である。

## 観測期間
新規登録ノードは、Qualified になる前に最低 72 時間の観測期間に入る。

## Qualified Node 条件
以下をすべて満たした場合のみ Qualified とする。
- 日次 availability >= 99.0%
- 有効 probe の 95%以上で finalized lag が閾値内
- 有効 probe の 95%以上で chain sync が閾値内
- 対象 current epoch に対して有効な voting key を持つ
- Program Agent heartbeat が有効
- 重大な異常や確認済み不正がない

## 検証メモ
- voting key 有効性は current epoch 条件に対して検証する
- 登録済み voting key は account-linked key 情報で確認可能であるべき
- Qualified 判定と報酬適格判定は別である

## 状態の分離
ノードは以下のいずれかになりうる。
- 登録済みだが未観測
- 観測済みだが未 Qualified
- Qualified だが寡占防止により報酬対象外
