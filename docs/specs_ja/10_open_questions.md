# 10. 未解決論点

## Critical
1. 初期の外部資金源は何か
2. Same Operator Group 判定に十分な証拠基準は何か

## v0.1 で確定済み
- external probe 閾値は canonical policy file に固定する
  - finalized lag: `<= 2` blocks
  - chain lag: `<= 5` blocks
- Program Agent は v0.1 の SSNP 参加条件として必須とする
- v0.1 以後に optional 化できるかは未解決だが、その前に heartbeat 保証と補助テレメトリを置き換える同等手段の定義が必要である

## Secondary
3. ASN diversity は hard cap にすべきか、scoring factor に留めるか
4. raw scoring data をどこまで公開するか
5. 未配分報酬 reserve の扱いをどうするか
6. オペレーターリスクを増やさずに raw endpoint / probe data をどこまで公開できるか

## 初期版の対象外
- 別ガバナンス承認なしの fee-based SSNP funding
- delegated influence
- protocol rule changes
- CPU や ICMP ping を主軸にした score input
