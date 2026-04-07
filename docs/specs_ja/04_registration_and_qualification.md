# 04. 登録と Qualified 判定

## 参加条件
参加ノードは最低でも以下を満たす。

- Symbol mainnet 上で運用されていること、または rollout / proving モードでは Symbol testnet 上で運用されていること
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

## External Probe 入力契約
external probe system は availability、finalized lag、chain synchronization quality に関する主たる真実源である。

各 probe event は最低でも以下を含まなければならない。
- `schema_version`
- `probe_id`
- `node_id`
- `region_id`
- `observed_at`
- `endpoint`
- `availability_up`
- `measurement_window_seconds`

利用可能かつ測定可能な probe event では、さらに以下を含まなければならない。
- `finalized_lag_blocks`
- `chain_lag_blocks`
- `source_height`
- `peer_height`

operator/debug 用の optional field としては以下を含めてよい。
- `http_status`
- `error_code`
- `resolver_ip`
- `notes`

validation ルール:
- 1 event は 1 node、1 region、1 endpoint、1 timestamped observation を表す
- `probe_id` は観測単位で一意でなければならず、idempotent replay 処理を可能にしなければならない
- `finalized_lag_blocks` と `chain_lag_blocks` は存在する場合、0 以上の整数でなければならない
- `availability_up = false` の場合、lag field は省略または `null` を許容する
- 必須 field 欠落の probe event は不正であり、Qualified 判定計算に入れてはならない

Qualified 判定用の集計ルール:
- 集計窓は UTC の 1 日単位とする
- 日次 availability は、その日の全 valid probe event を母集団に使う
- finalized lag 適合率は `availability_up = true` かつ `finalized_lag_blocks` が測定可能な valid probe event のみを母集団に使う
- chain lag 適合率は `availability_up = true` かつ `chain_lag_blocks` が測定可能な valid probe event のみを母集団に使う
- 1 日の中で最低 2 probe region から valid event が来なければ、pass ではなく insufficient evidence とする
- v0.1 の正式上限値は policy file の `probe_thresholds` を使う

## Daily Qualification Summary 契約
Qualification Engine は、raw probe event から導出した日次 summary record を読むものとする。

各 daily summary は最低でも以下を含まなければならない。
- `node_id`
- `date_utc`
- `policy_version`
- `finalized_lag_threshold_blocks`
- `chain_lag_threshold_blocks`
- `valid_probe_count`
- `availability_up_count`
- `availability_ratio`
- `finalized_lag_measurable_count`
- `finalized_lag_within_threshold_count`
- `finalized_lag_ratio`
- `chain_lag_measurable_count`
- `chain_lag_within_threshold_count`
- `chain_lag_ratio`
- `region_count`
- `availability_passed`
- `finalized_lag_passed`
- `chain_lag_passed`
- `multi_region_evidence_passed`
- `qualified_probe_evidence_passed`

optional field:
- `insufficient_evidence_reason`
- `generated_at`

summary ルール:
- `availability_ratio = availability_up_count / valid_probe_count`
- `finalized_lag_ratio = finalized_lag_within_threshold_count / finalized_lag_measurable_count`
- `chain_lag_ratio = chain_lag_within_threshold_count / chain_lag_measurable_count`
- `availability_passed` は `availability_ratio >= 0.99` を満たす場合のみ true
- `finalized_lag_passed` は `finalized_lag_ratio >= 0.95` を満たす場合のみ true
- `chain_lag_passed` は `chain_lag_ratio >= 0.95` を満たす場合のみ true
- `multi_region_evidence_passed` は `region_count >= 2` を満たす場合のみ true
- `qualified_probe_evidence_passed` は probe 側 pass flag がすべて true で、insufficient-evidence 条件がない場合のみ true

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
- 有効 probe の 95%以上で finalized lag が canonical policy file の `probe_thresholds.finalized_lag_max_blocks` 以下
- 有効 probe の 95%以上で chain sync が canonical policy file の `probe_thresholds.chain_lag_max_blocks` 以下
- 対象 current epoch に対して有効な voting key を持つ
- Program Agent heartbeat が有効
- 必須 hardware simple check が有効
- 重大な異常や確認済み不正がない

## 検証メモ
- voting key 有効性は current epoch 条件に対して検証する
- 登録済み voting key は account-linked key 情報で確認可能であるべき
- v0.1 の正式値は `docs/policies/program_agent_policy.v2026-04.yaml` に定義された finalized lag `2` blocks、chain lag `5` blocks を使う
- raw probe event と日次 qualification summary は分けて保存すべきである
- Qualification Engine の入力は raw event 走査ではなく日次 summary record にすべきである
- Qualified 判定と報酬適格判定は別である

## 状態の分離
ノードは以下のいずれかになりうる。
- 登録済みだが未観測
- 観測済みだが未 Qualified
- Qualified だが寡占防止により報酬対象外
