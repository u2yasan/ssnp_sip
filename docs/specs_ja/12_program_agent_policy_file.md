# 12. Program Agent Policy File

## 目的
Program Agent 設計では、heartbeat 間隔、CPU / disk チェック profile、
hardware threshold、probe threshold、reference environment metadata のような値を
policy 駆動で扱う。

そのため、実装者がローカル定数を勝手に埋め込まないよう、
repo 管理の concrete な policy file が必要である。

## 正式ファイル
初期 v0.1 の policy file は以下とする。

- `docs/policies/program_agent_policy.v2026-04.yaml`

このファイルは以下の source になる。
- `GET /api/v1/agent/policy` の response
- Program Agent のローカル実行パラメータ
- hardware simple check の profile identifier
- payload 送信時の policy-version validation

## 形式
policy file の形式は YAML とする。

理由:
- 運用者とレビュー担当者が読みやすい
- repo 管理による versioning に向く
- v0.1 の documentation-first 運用に合う
- 将来 DB 管理へ移しても wire semantics を変えずに済む

## 必須 top-level field
- `policy_version`
- `heartbeat_interval_seconds`
- `cpu_profile`
- `disk_profile`
- `hardware_thresholds`
- `probe_thresholds`
- `reference_environment`

## `cpu_profile`
必須 field:
- `id`
- `duration_seconds`
- `warmup_seconds`
- `measured_seconds`
- `cooldown_seconds`
- `worker_cap`
- `workload_mix`
- `acceptance_floor`

### `workload_mix`
必須 field:
- `hashing`
- `integer`
- `matrix`

値は合計 `1.00` になる分率とする。

### `acceptance_floor`
必須 field:
- `type`
- `minimum`

v0.1 では以下に固定する。
- `type` は `normalized_score`
- `minimum` は `1.00`

## `disk_profile`
必須 field:
- `id`
- `duration_seconds`
- `warmup_seconds`
- `measured_seconds`
- `cooldown_seconds`
- `block_size_bytes`
- `queue_depth`
- `concurrency`
- `read_ratio`
- `write_ratio`
- `acceptance_floor`

v0.1 では以下に固定する。
- `block_size_bytes` は `4096`
- `queue_depth` は `32`
- `concurrency` は `4`
- `read_ratio` は `0.70`
- `write_ratio` は `0.30`
- `acceptance_floor.type` は `measured_iops`
- `acceptance_floor.minimum` は `1500`

## `hardware_thresholds`
必須 field:
- `cpu_cores_min`
- `ram_gb_min`
- `storage_gb_min`
- `ssd_required`

v0.1 では以下に固定する。
- `cpu_cores_min` は `8`
- `ram_gb_min` は `32`
- `storage_gb_min` は `750`
- `ssd_required` は `true`

## `probe_thresholds`
必須 field:
- `finalized_lag_max_blocks`
- `chain_lag_max_blocks`

v0.1 では以下に固定する。
- `finalized_lag_max_blocks` は `2`
- `chain_lag_max_blocks` は `5`

これらは v0.1 の external probe による Qualified 判定の正式閾値である。
agent と portal はローカル定数を複製せず、同じ repo 管理 policy response を使わなければならない。

## `reference_environment`
必須 field:
- `id`
- `os_image_id`
- `agent_version`
- `cpu_profile_id`
- `disk_profile_id`
- `baseline_source_date`

この section は `normalized_score >= 1.00` の意味を説明し、
baseline drift を黙って起こさないために存在する。

## API と validation ルール
- `GET /api/v1/agent/policy` は active policy file の semantics を出すべきである
- portal は `policy_version` 不一致の check payload を reject しなければならない
- agent は取得した policy が欠落または非互換でも embedded default に silent fallback してはならない
- policy 値を変える場合は新しい `policy_version` が必要である

## 互換性ルール
repo 管理の YAML file は初期 storage format にすぎない。

将来 portal DB へ移す場合でも:
- field 名の意味は互換に保つ
- `GET /api/v1/agent/policy` の意味は維持する
- 既存 agent 実装に silent behavior change を起こしてはならない
