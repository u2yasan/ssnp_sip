# 05. スコアリングとランキング

## 設計上の優先順位
スコアリングは、すでに Qualified になったノードを順位付けするためのものである。
Qualification gate の代用であってはならない。

## 総合スコア
S = 0.7 * B + 0.3 * D

ここで:
- B = Base Performance Score
- D = Decentralization Score

## ベース性能スコア (70%)
- Availability: 30
- Finalization tracking: 20
- Chain sync consistency: 10
- Voting key continuity: 10

## 分散スコア (30%)
- Geographic diversity: 15
- ASN / infrastructure diversity: 10
- Country concentration avoidance: 5

## ランキングルール
Qualified node を総合スコアの降順で並べる。

## 同点解消順
1. finalization score が高い方
2. availability score が高い方
3. validated registration time が早い方

## 計測原則
- CPU claim は中核スコア入力にしない
- hardware simple check status を ranking multiplier にしない
- ICMP ping は中核スコア入力にしない
- node-local reputation signal は参考情報に留め、中核的な公開スコア入力にしてはならない
- 単一リージョン probe 結果が支配的になってはならない
- 「速い見た目」より finalization と sync 追従を優先する

## 制約
分散性は Qualified node の順位を改善しうるが、
最低限の運用品質の代替にはならない。
