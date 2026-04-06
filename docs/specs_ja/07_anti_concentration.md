# 07. 寡占防止

## これが中核である理由
寡占防止が弱いと、SSNP は大規模オペレーター、ホスティング再販、運用代行の
報酬取得システムに劣化する。

## Same Operator Group
Same Operator Group とは、実質的に重なった運用支配下にあると合理的に判断されるノード群をいう。

含まれる例:
- 同一 operator address
- 同一 registrable domain
- 同一 managed provider
- 同一 operational contractor
- 共有された certificate administration
- 実質的に重なる operational authority

## 報酬適格制限
- Same Operator Group あたり reward-eligible node は最大 1 台
- registrable domain あたり reward-eligible node は最大 1 台

## Managed Provider
Managed node hosting provider や operational contractor は、
実質的な運用支配が重なる場合、同一グループとして扱う。

## Backfill Rule
同一グループから複数ノードが報酬対象圏内に入った場合、
最上位ノードだけを reward-eligible とし、
残り枠は次順位の独立ノードで補充する。

## 証拠原則
domain 単独ではすべての group 判定に十分ではない。
それでも same-domain exclusion は hard reward-selection filter として保持する。

## 設計警告
本当のリスクは「厳しすぎること」より、
別ラベルを使った obvious multi-slot capture を許すことである。
