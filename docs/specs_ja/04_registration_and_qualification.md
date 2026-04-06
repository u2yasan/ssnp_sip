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
- 推奨 RAM / CPU / storage 要件の simple hardware check 結果

## Qualified 判定の思想
重要なのは「紙の上で強いマシン」ではない。
重要なのは「そのノードが今、安全かつ可観測な voting を継続できること」である。

## 観測期間
新規登録ノードは、Qualified になる前に最低 72 時間の観測期間に入る。

## ハードウェア簡易チェック
SSNP は、Super Node 条件に必要な推奨ハードウェア要件を
満たすことの簡易チェックを要求してよい。

ルール:
- チェックは Program Agent が node 側の信頼境界で生成する
- これは cryptographic proof ではなく simple operational check である
- 公開ポータルには、生値ではなく pass/fail のみを出す
- 基準は Symbol の Recommended Dual & Voting node specification を採用する
  - CPU >= 8 cores
  - RAM >= 32GB
  - Disk >= 750GB SSD
  - Disk performance >= 1500 IOPS
- hardware simple check は eligibility gate であり、scoring input ではない
- Program Agent チェックのない自己申告ハードウェア情報は不十分である

## チェック実施タイミング
Hardware simple check は以下で実施する。
- 初回登録時
- voting key 更新時
- enrolled agent identity の再設定後や大きな環境変更後の再チェック時
- 上記の同タイミングで bounded CPU load test も合格しなければならない

## Bounded CPU 負荷テスト
CPU 負荷テストは、公開ベンチマーク競争ではなく簡易運用チェックとして扱う。

推奨方法:
- Program Agent に同梱した SSNP 定義の固定 workload profile を使う
- すべての node が同じ policy version では同じ workload を実行する
- テスト時間は合計 180 秒に固定する
  - 30 秒 warm-up
  - 120 秒 measured interval
  - 30 秒 cool-down と結果確定
- worker thread 数は 8 を上限とし、runtime から見える CPU 資源数で cap する
- テスト中は外部ネットワーク依存を持たない
- 公開ポータルに送るのは pass/fail のみとする

合格ルール:
- 実行エラーなく完走すること
- measured score が active test profile の policy-defined threshold 以上であること
- 結果が当該 registration、voting key 更新、または再チェックイベントに束縛されていること

設計メモ:
- 閾値の具体値は public specification に埋め込まず、SSNP policy 側で管理する
- 合格後の score を ranking に使ってはならない

推奨初期 profile:
- deterministic hashing と integer / matrix operation を組み合わせた mixed CPU workload を使う
- 単一命令セットに強く寄る microbenchmark は避ける
- `cpu-check-v1` のような internal profile ID を割り当てる
- 初期合格閾値は複数 VPS provider の pilot measurement から policy 側で定め、保守的に見直す

## Disk Performance 簡易チェック
Disk performance は provider の広告値ではなく、bounded local I/O test で確認すべきである。

推奨方法:
- 固定の SSNP disk profile を使った local SSD 向け random-read/random-write test を実行する
- block size は 4KiB とし、queue-depth と concurrency は policy で定義する
- テスト時間は合計 60 秒に固定する
  - 10 秒 warm-up
  - 40 秒 measured interval
  - 10 秒 cool-down と結果確定
- node runtime storage 領域上の一時ファイルに対して実行する
- 公開表示は pass/fail のみとする

合格ルール:
- measured result が active disk profile の policy-defined IOPS floor 以上であること
- I/O error なく完了すること
- 結果が hardware simple check と同じ registration、voting key 更新、または再チェックイベントに束縛されていること

## Qualified Node 条件
以下をすべて満たした場合のみ Qualified とする。
- 日次 availability >= 99.0%
- 有効 probe の 95%以上で finalized lag が閾値内
- 有効 probe の 95%以上で chain sync が閾値内
- 対象 current epoch に対して有効な voting key を持つ
- Program Agent heartbeat が有効
- 必須 hardware simple check が有効
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
