[English](README.md) | 日本語

# Program Agent

SSNP Program Agent の最小 Go stub。

## Quickstart

SSNP 専用の agent 鍵ペアを生成:

```sh
go run ./cmd/program-agent --config ./config.example.yaml gen-key --out-dir ./keys
```

`config.example.yaml` をベースに設定した後、次を実行:

```sh
go run ./cmd/program-agent --config ./config.example.yaml enroll --challenge-id enroll-001
go run ./cmd/program-agent --config ./config.example.yaml run
go run ./cmd/program-agent --config ./config.example.yaml check --event-type registration --event-id check-001
go run ./cmd/program-agent --config ./config.example.yaml telemetry --warning-flag portal_unreachable
```

testnet 向け導線は `config.testnet.example.yaml` と `../docs/testnet_runbook.md` を使うこと。

## コマンド

- `gen-key`: `--out-dir` に `agent_private_key.pem` と `agent_public_key.pem` を出力し、その path を JSON で表示する
- `enroll`: ローカル公開鍵を portal の enrollment challenge に紐付ける
- `run`: policy を取得し、定期チェックと heartbeat 送信を行う
- `check`: hardware / CPU / disk の bounded check を実行し、結果を送信する
- `telemetry`: warning telemetry を明示送信する

## 運用上の制約

- policy fetch は fail-closed であり、embedded fallback はない
- 壊れた `state.json` は自動修復しない
- portal 側の `4xx` / `5xx` はそのまま返す
- v0.1 の telemetry 自動生成は次に限定する
  - `portal_unreachable`
  - `local_check_execution_failed`
  - `voting_key_expiry_risk`
  - `certificate_expiry_risk`
- certificate check は期限メタデータだけを見る。CA や hostname の信頼性検証はしない

## 検証

リポジトリ共通の検証を回す場合は root で `make test` と `make smoke` を使う。

```sh
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./...
```
