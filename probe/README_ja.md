[English](README.md) | 日本語

# Probe Worker

SSNP の testnet 運用向け最小 external probe worker。

## クイックスタート

```sh
go run ./cmd/probe-worker --config ./config.example.yaml run
```

1 回だけ実行する場合:

```sh
go run ./cmd/probe-worker --config ./config.example.yaml run-once
```

## 設定項目

- `portal_base_url`: portal の base URL
- `region_id`: この worker インスタンスの固定 region ID
- `source_endpoint`: 参照用 chain height を取る Symbol REST endpoint
- `request_timeout_seconds`: リクエスト単位の timeout
- `poll_interval_seconds`: 固定 poll 間隔。`measurement_window_seconds` にも使う
- `targets[].node_id`: portal seed config に存在する node ID
- `targets[].endpoint`: probe 対象の Symbol REST endpoint

## 運用境界

- 単一 process worker のみ
- queue、scheduler、永続 retry state は持たない
- target 側失敗は `availability_up = false` として送信する
- source endpoint 失敗時は target 側 failure を捏造せず、その cycle を落とす

運用手順全体は `docs/testnet_runbook.md` を見ること。
