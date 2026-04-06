# 11. Program Agent 設計

## 位置づけ
Program Agent は、SSNP v0.1 のための **ノード側ローカル補助プロセス** である。

役割は以下に限定する。
- 登録済みノードとローカル実行個体の enrollment binding
- 署名付き heartbeat による liveness 保証
- 運用用の限定的な補助テレメトリ送信
- 推奨閾値に対する simple hardware capability check
- voting key 失効予兆や証明書期限切れ予兆のようなローカル警告の生成

外部監視の代替ではない。Qualified 判定やランキングの主要真実源にしてはならない。

## 設計目標
- voting node 側で inbound port を要求しない
- 日常運用負荷を低く保つ
- 小規模オペレーターでも扱える単純な導入モデルにする
- 秘密情報の露出と障害半径を最小化する
- 監査可能な deterministic liveness signal を出す

## 非目的
- リモート shell 実行
- リモート設定変更
- 副作用を伴う自動復旧
- オペレーター秘密鍵やウォレット秘密の保管
- 外部 probe と矛盾するデータの上書き
- pass/fail で足りる場面での raw RAM / CPU / HDD 値やベンチスコアの公開
- SSNP 運用に不要な raw host 情報の収集

## 信頼境界
以下の主たる真実源は外部 probe インフラである。
- availability
- finalized lag
- chain sync quality
- 公開 endpoint 到達性

Program Agent が権威を持てるのは以下だけである。
- ローカルの SSNP runtime instance が生存しているか
- enrolled 済み agent key を現在も保持しているか
- ローカルで検知した運用警告を発したか

Program Agent データが外部 probe と衝突した場合は、外部 probe を優先する。

## 脅威モデル
設計は以下を前提にしなければならない。
- オペレーターが agent を誤設定する
- 攻撃者が古い heartbeat を replay する
- ノードホストが部分侵害される
- portal API に偽装 agent 通信や重複送信が来る
- テレメトリを取り過ぎると機密なインフラ情報が漏れる

そのため最低でも以下を要求する。
- agent メッセージ署名
- sequence または nonce による anti-replay
- 短命な enrollment credential
- outbound-only 通信
- 最小限テレメトリ
- 内部チェック情報と public pass/fail status の分離
- 特権的リモート操作の明示的禁止

## 配置モデル
Program Agent は以下のどちらかで動かす。
- voting node と同じホスト
- 同一運用信頼境界にある厳格管理ホスト

v0.1 の推奨モデルは以下である。
- 登録ノード 1 台につき agent 1 個体
- OS サービスプロセス 1 本
- ローカル service manager による自動再起動
- outbound HTTPS のみ

以下を前提にしてはならない。
- Kubernetes
- message broker
- VPN 前提の control plane
- inbound firewall 例外

## 登録と Enrollment
Enrollment は次の 3 つを束縛しなければならない。
1. 登録済み node record
2. オペレーター承認済み登録操作
3. その agent instance の公開鍵

最小フロー:
1. オペレーターが portal で node metadata を登録する
2. portal が短命な enrollment challenge を発行する
3. agent がローカルで agent keypair を生成する
4. agent が公開鍵と challenge response を送る
5. portal がその公開鍵を当該 node の active enrolled identity として保存する
6. 以後の heartbeat と telemetry はその agent key で署名する

制約:
- agent private key はローカルから出さない
- enrollment challenge は短時間で失効させる
- 再 enrollment 時は旧 active agent key を失効させる
- 1 node record に複数 active agent identity を標準では許可しない

## Agent Identity と鍵
Program Agent には専用の署名鍵が必要である。

要件:
- node private key や harvester key を使わず、SSNP agent 専用鍵を使う
- voting private key を要求しない
- 少なくとも OS ネイティブのファイル権限で鍵を保護する
- オペレーター主導の鍵 rotation を可能にする
- portal に出すのは公開鍵または fingerprint のみ

より強い安全なローカル保管が可能なら使ってよい。
ただし MVP 参加条件にしてはならない。

## ハードウェア簡易チェック
Program Agent は、Super Node 参加に必要な推奨ハードウェア条件を、
正確な値を公開せずに確認する用途に使ってよい。

対象カテゴリ:
- RAM 閾値
- CPU 閾値
- storage 閾値
- disk performance 閾値

設計ルール:
- cryptographic proof ではなく simple pass/fail check を使う
- チェック結果は enrolled node record と現在の check window に束縛する
- チェックは eligibility condition であり、ranking signal ではない
- 自己申告値で check を代替させない
- 基準は Symbol の Recommended Dual & Voting node specification を採用する
  - CPU >= 8 cores
  - RAM >= 32GB
  - Disk >= 750GB SSD
  - Disk performance >= 1500 IOPS

## CPU 負荷テスト方針
CPU 負荷テストは、以下の限定タイミングでのみ実行してよい。
- 初回登録時
- voting key 更新時
- 大きな環境変更や異議審査に伴う明示的な再チェック時

ルール:
- 日常運用で常時または高頻度に回してはならない
- テスト時間は上限付きで policy 定義にする
- 結果は public raw score ではなく、閾値充足または不合格として扱う
- ranking formula に組み込んではならない

推奨実行方法:
- Program Agent に固定の SSNP workload profile を同梱し、全 node が同じ test logic を実行する
- 外部ネットワーク依存のない deterministic な mixed workload を使い、hashing と integer / matrix operation を組み合わせる
- 実行時間は合計 180 秒とする
  - 30 秒 warm-up
  - 120 秒 measured interval
  - 30 秒 cool-down と結果確定
- worker thread 数は最大 8 とし、runtime から見える CPU 資源数で cap する
- local profile ID、policy version、execution timestamp、pass/fail result を記録する
- raw score を保存する場合でも operator または運営内部用途に留める

推奨合格条件:
- runtime error や早期終了がないこと
- measured score が active workload profile の policy-defined floor 以上であること
- 要求された test profile と報告された test profile に不一致がないこと
- 結果が registration、voting key 更新、または再チェックイベントに束縛されていること

推奨初期 policy 方向:
- まず `cpu-check-v1` のような単一 profile から始める
- 初期合格閾値は複数 VPS provider と region の pilot measurement から決める
- 明らかに非力な環境を落としつつ、推奨クラスの健全 node を過度に排除しない保守的な floor にする
- floor を変える場合は必ず policy version を上げ、無言で変えない

推奨 `cpu-check-v1` 初期値:
- score unit: 120 秒の measured interval における completed work units per second
- workload mix:
  - 50% deterministic hashing work
  - 30% deterministic integer arithmetic work
  - 20% deterministic matrix-operation work
- worker count: `min(8, visible_cpu_threads)`
- 初期合格 floor:
  - `normalized_score >= 1.00`
  - ここで `1.00` は同一 policy version における SSNP 承認済み reference environment の基準値とする

reference environment ルール:
- reference environment は policy version ごとに公開する
- reference environment は Symbol の Recommended Dual & Voting node baseline に追随させる
- reference environment を変える場合は policy version を上げる

推奨 reference environment 定義:
- third-party provider の marketing SKU 名ではなく、SSNP 運営管理下の benchmark environment を使う
- その環境は最低でも以下を満たす
  - 8 dedicated vCPU または同等の compute entitlement
  - 32GB RAM
  - 750GB 以上の SSD-backed storage
  - `disk-check-v1` に合格できる storage
- OS image、agent version、workload profile version は policy version ごとに固定する
- 運営は最低でも以下を公開する
  - reference environment ID
  - OS image identifier
  - agent version
  - workload profile ID
  - baseline normalized score の採取日

推奨運用方法:
- 新しい baseline を公開する前に、少なくとも 3 回の reference run を別 region または別 provider で取る
- `1.00` の normalization source には measured result の median を使う
- raw reference measurement は内部保持でもよいが、再現に必要な policy metadata は公開する
- たまたま速かった単発 instance を baseline にしてはならない

## Disk Performance チェック方針
Disk performance は bounded local I/O check で確認する。

推奨方法:
- `disk-check-v1` のような固定 SSNP disk profile を使う
- local の一時ファイルに対する random-read/random-write の SSD 向け workload を実行する
- block size は 4KiB とし、初期値は以下とする
  - queue depth: 32
  - concurrency: 4
  - read/write mix: 70/30
- 実行時間は合計 60 秒とする
  - 10 秒 warm-up
  - 40 秒 measured interval
  - 10 秒 cool-down と結果確定
- 外部 storage や network dependency を持たない
- 公開ポータルには pass/fail のみ記録する

推奨合格条件:
- I/O error や早期終了がないこと
- measured result が active disk profile の policy-defined IOPS floor 以上であること
- 要求された disk profile と報告された disk profile に不一致がないこと
- 結果が registration、voting key 更新、または再チェックイベントに束縛されていること

推奨 `disk-check-v1` 初期 floor:
- `measured_iops >= 1500`
- `measured_latency_p95` は内部運用用途として保持してよいが、v0.1 では公開せず、pass/fail 条件にも入れない

## Heartbeat 契約
Heartbeat が答えるべき問いは 1 つだけである。
「enrolled 済み agent instance は今も生きていて、この node record に結び付いているか」

### 必須 heartbeat 項目
- agent public key fingerprint
- registered node ID
- heartbeat timestamp
- monotonic sequence number
- agent software version
- enrollment generation または key version
- local observation summary flags

### 送信ルール
- 5 分ごとに小さな startup jitter を入れて送る
- 一時障害時は上限付き backoff で retry する
- オフライン backlog を無制限に溜めない

### v0.1 の liveness ルール
- `healthy`: 直近 15 分に有効 heartbeat が 2 回以上ある
- `stale`: 直近 15 分では 2 回未満だが、直近 30 分には 1 回以上ある
- `failed`: 直近 30 分に有効 heartbeat が ない

### 検証ルール
Heartbeat は以下をすべて満たす場合のみ有効である。
- 署名が active enrolled agent key で検証できる
- timestamp が許容 skew の範囲内である
- sequence number が最後に受理した値より新しい
- node record が still active である

## 補助テレメトリ
テレメトリは最小限かつ運用上必要なものに限る。

許可カテゴリ:
- agent process uptime
- ローカル node process presence check
- 現在の software version string
- voting-key expiry または epoch-validity status
- monitored endpoint 用 certificate expiry timestamp
- ローカル設定された domain expiry reminder data
- hardware simple check validity status
- 粗い disk-pressure / resource-warning flag

送る形式は以下を優先する。
- boolean
- 上限のある enum
- 粗い bucket
- 期限判定に必要な明示 timestamp

送ってはならないもの:
- private key
- wallet seed
- raw config file
- 全 process list
- shell command output
- ローカル簡易チェック実行に厳密に必要でない詳細 hardware inventory

## ローカルチェック
Program Agent は警告生成のためにローカルチェックをしてよい。
ただし、それは補助情報に留まる。

許可するローカルチェック:
- node process present
- local API responding
- configured certificate near expiry
- voting key nearing invalid epoch
- hardware simple check execution
- agent cannot reach portal API

禁止するローカル挙動:
- 明示的 opt-in なしの自動 service restart
- リモートからの node 設定変更
- portal 指示による任意コマンド実行
- オペレーター承認なしの update 自動適用

## Warning Telemetry Semantics
v0.1 の telemetry warning set は次の 4 種に固定する。
- `portal_unreachable`
- `local_check_execution_failed`
- `voting_key_expiry_risk`
- `certificate_expiry_risk`

ルール:
- warning telemetry は補助情報に留まり、外部 probe 証拠を上書きしてはならない
- warning telemetry を ranking input に使ってはならない
- warning telemetry を cryptographic truth source と説明してはならない

v0.1 の warning 生成ルール:
- `portal_unreachable`
  - portal 通信失敗が連続 3 回に達したら pending にする
  - portal 通信回復後に 1 回だけ送信する
- `local_check_execution_failed`
  - hardware / CPU / disk check が正常な pass/fail ではなく、実行不能だった時だけ送信する
- `voting_key_expiry_risk`
  - v0.1 では `monitored_endpoint` の Symbol node API を入力源に使う
  - current node account と active voting key の寿命を chain data から導出する
  - 最も早く切れる active voting key が 14 日以内なら送信する
  - node API failure、JSON 異常、期待フィールド欠落、active voting key 不在は silent no-op とする
- `certificate_expiry_risk`
  - `monitored_endpoint` が `https` の場合だけ使う
  - leaf certificate の `NotAfter` だけを見る
  - expiry が 14 日以内なら送信する
  - これは期限確認専用であり、PKI trust validation ではない

現在の portal stub における portal 側処理ルール:
- accept した warning telemetry は portal 側の operator notification handling を起動してよい
- notification dedupe には `node_id + alert_code + severity` を使う
- `warning` severity には 24 時間の cooldown を使う
- known node は seed config file から読む
- v0.1 の delivery / dedupe state は JSON snapshot file に保存する
- notification delivery failure は operational event として記録し、それ自体で qualification を変えてはならない

## Portal API 契約
portal 側の agent interface は最小に保つ。
- agent enroll
- agent identity revoke / rotate
- signed heartbeat 受信
- signed warning telemetry 受信
- accept した telemetry の history / latest view 読み出し
- hardware simple check result 受信
- 必要なら静的 agent policy metadata 配布

可能な限り idempotent に設計する。
長時間の対話セッションを要求してはならない。

## Hardware Simple Check Payload Schema
hardware simple check result は、上限のある versioned payload として送るべきである。

必須項目:
- `schema_version`: payload schema version
- `node_id`: 登録済み node identifier
- `agent_key_fingerprint`: enrolled agent identity の fingerprint
- `event_type`: `registration`、`voting_key_renewal`、`recheck` のいずれか
- `event_id`: 当該チェックイベントの identifier
- `policy_version`: active SSNP policy version
- `cpu_profile_id`: active CPU workload profile。初期値は `cpu-check-v1`
- `disk_profile_id`: active disk workload profile。初期値は `disk-check-v1`
- `checked_at`: 結果確定時刻の UTC timestamp
- `cpu_check_passed`: boolean
- `disk_check_passed`: boolean
- `ram_check_passed`: boolean
- `storage_size_check_passed`: boolean
- `ssd_check_passed`: boolean
- `cpu_load_test_passed`: boolean
- `overall_passed`: 必須 sub-check 全体から導出される boolean
- `agent_version`: Program Agent version
- `signature`: payload 全体に対する agent signature

内部利用向けの任意項目:
- `normalized_cpu_score`
- `measured_iops`
- `measured_latency_p95`
- `visible_cpu_threads`
- `visible_memory_bytes`
- `visible_storage_bytes`
- `error_code`
- `error_detail`

公開ポータルのルール:
- 必要に応じて high-level の pass/fail status と policy/profile identifier だけを出す
- v0.1 では raw score や raw hardware 値を公開してはならない

検証ルール:
- portal は schema version 不一致、必須項目欠落、invalid signature、`overall_passed` 導出不整合の payload を reject しなければならない

参考 payload:
```json
{
  "schema_version": "1",
  "node_id": "node-abc",
  "agent_key_fingerprint": "agent-fp-123",
  "event_type": "registration",
  "event_id": "check-2026-04-06-0001",
  "policy_version": "2026-04",
  "cpu_profile_id": "cpu-check-v1",
  "disk_profile_id": "disk-check-v1",
  "checked_at": "2026-04-06T10:30:00Z",
  "cpu_check_passed": true,
  "disk_check_passed": true,
  "ram_check_passed": true,
  "storage_size_check_passed": true,
  "ssd_check_passed": true,
  "cpu_load_test_passed": true,
  "overall_passed": true,
  "agent_version": "1.0.0",
  "signature": "base64-signature"
}
```

## Program Agent API Endpoints
v0.1 の portal 側 API は最小かつ明示的に保つべきである。

active policy value は `docs/specs/12_program_agent_policy_file.md` で定義した
repo 管理の YAML policy file から供給される前提とする。

### `POST /api/v1/agent/enroll`
目的:
- 登録済み node record に agent identity を束縛する。

request body:
- `node_id`
- `enrollment_challenge_id`
- `agent_public_key`
- `agent_version`
- `signature`

success response:
```json
{
  "status": "ok",
  "node_id": "node-abc",
  "agent_key_fingerprint": "agent-fp-123",
  "policy_version": "2026-04"
}
```

### `POST /api/v1/agent/heartbeat`
目的:
- enrolled 済み agent から signed heartbeat を受け取る。

request body:
- heartbeat contract で定義した payload。

success response:
```json
{
  "status": "accepted",
  "node_id": "node-abc",
  "received_at": "2026-04-06T10:35:00Z"
}
```

portal 側の運用挙動:
- portal は accept した heartbeat timestamp から `heartbeat_stale` と `heartbeat_failed` alert を導出してよい
- 現在の stub は次を使う
  - accept した heartbeat が 15 分無ければ `stale`
  - accept した heartbeat が 30 分無ければ `failed`

### `POST /api/v1/agent/checks`
目的:
- hardware simple check result と bounded CPU / disk test result を受け取る。

request body:
- 上記 hardware check payload。

success response:
```json
{
  "status": "accepted",
  "node_id": "node-abc",
  "event_id": "check-2026-04-06-0001",
  "overall_passed": true,
  "received_at": "2026-04-06T10:30:05Z"
}
```

### `POST /api/v1/agent/telemetry`
目的:
- heartbeat や hardware check には含めない補助 warning telemetry を受け取る。

request body:
- signed warning field のみを持つ versioned telemetry payload。

success response:
```json
{
  "status": "accepted",
  "node_id": "node-abc",
  "received_at": "2026-04-06T10:40:00Z"
}
```

portal 側の運用挙動:
- accept した telemetry warning は notification delivery handling を起動してよい
- 現在の stub で設定できるチャネルは `email` のみ
- 現在の stub の notifier backend は実メール配送ではなく stub 実装である

### `GET /api/v1/agent/telemetry`
目的:
- オペレーターと program operations のために保存済み warning telemetry を返す。

query parameter:
- 任意の `node_id`
- 任意の `warning_code`
- `node_id + warning_code` ごとの最新状態だけ返す `view=latest`

response 挙動:
- default view は telemetry history item を返す
- `view=latest` は current latest view を返す
- portal stub は v0.1 では telemetry と関連 alert state を local JSON snapshot file に保存する

### `GET /api/v1/agent/policy`
目的:
- agent が active policy version と profile identifier を取得できるようにする。

query parameter:
- `node_id`
- `agent_key_fingerprint`

success response:
```json
{
  "policy_version": "2026-04",
  "heartbeat_interval_seconds": 300,
  "cpu_profile": {
    "id": "cpu-check-v1"
  },
  "disk_profile": {
    "id": "disk-check-v1"
  },
  "hardware_thresholds": {
    "cpu_cores_min": 8,
    "ram_gb_min": 32,
    "storage_gb_min": 750,
    "ssd_required": true
  },
  "reference_environment": {
    "id": "ref-env-2026-04"
  }
}
```

## API Error Handling
portal は、狭く machine-readable な error を返すべきである。

推奨 error code:
- `invalid_schema_version`
- `missing_required_field`
- `invalid_signature`
- `unknown_node_id`
- `agent_not_enrolled`
- `policy_version_mismatch`
- `invalid_profile_id`
- `duplicate_event_id`
- `invalid_overall_passed`
- `stale_timestamp`
- `rate_limited`

推奨 error response:
```json
{
  "status": "error",
  "error_code": "invalid_signature",
  "message": "signature verification failed"
}
```

## API 設計ルール
- すべての endpoint は TLS を必須にする
- すべての write endpoint は `event_id` または同等の request identity で idempotent でなければならない
- portal は accept / reject 判定を reason code 付きで記録しなければならない
- 欠落 field を pass/fail 推定で補ってはならない
- policy/profile mismatch は無言補正せず明示 reject にしなければならない

## 障害時の意味づけ
Program Agent 障害は参加条件に影響するが、外部証拠を書き換えない。

ルール:
- agent heartbeat failed は、policy が active liveness を要求するなら `Qualified` を阻害または剥奪しうる
- invalid または expired な hardware simple check は registration、voting key 更新、Qualified 維持を阻害しうる
- agent が healthy を主張しても、外部 outage は outage のまま
- portal ingestion failure は node failure と別に観測可能でなければならない
- operator 向け reason code は最低でも以下を分離する
  - agent missing
  - agent stale
  - invalid signature
  - invalid hardware simple check
  - enrollment revoked
  - portal delivery failure

## セキュリティ要件
- デフォルト outbound-only 通信
- 少なくとも message signature 層での相互識別
- transport に TLS
- replay 耐性
- portal ingestion の strict input validation
- node 単位 rate limit
- enrollment、key rotation、revocation、invalid heartbeat rejection、hardware-check rejection の audit log

portal は署名なし agent message を信じてはならない。

## プライバシーとデータ最小化
SSNP は Program Agent を host surveillance にしてはならない。

従って:
- liveness と運用警告に直接必要なデータだけを集める
- 可能な限り raw hardware 値やベンチスコアではなく public pass/fail status を出す
- raw なインフラ詳細を公開しない
- operator-only detail と public portal data を分離する
- すべての telemetry field に存在理由を文書化する

## 運用モデル
低コスト運用のために以下を優先する。
- platform ごとの単機能 agent package または binary
- 平坦で明示的な設定ファイル形式
- enrollment 後は pull-free に近い運用
- plain text または structured text のローカルログ
- 非専門オペレーターでも失敗原因が分かる故障モード

日常運用の大半は以下で足りるべきである。
- service health 確認
- 必要時の鍵 rotation
- version update
- voting key 更新や hardware check 更新への対応
- warning alert 対応

## ガバナンス境界
Program Agent が補助できるのは以下である。
- liveness 保証
- 運用警告
- operator 向け診断

Program Agent に最終権限を持たせてはならない領域:
- reward eligibility
- Same Operator Group 判定
- operator disqualification
- payment approval

## v0.1 の推奨判断
v0.1 では Program Agent は必須のままにすべきである。理由は以下。
- deterministic な liveness signal が取れる
- 外部監視とローカル運用警告のギャップを埋められる
- 手動証明より低コストで継続運用できる

これを将来 optional にするなら、同等の低信頼・低運用負荷な代替手段が先に必要である。

## 実装でまだ詰めるべき点
- timestamp skew 許容値
- agent public key format と signature scheme
- node ID canonicalization
- enrollment challenge format
- CPU cores、RAM、disk size、SSD、IOPS のローカル簡易チェック方法
- voting key 更新イベント時の再チェック意味定義
- `cpu-check-v1` 用の承認済み reference environment の公開
- telemetry schema versioning
- 再 enrollment の grace period と通知フロー
