[English](README.md) | 日本語

# Portal

ローカル SSNP 検証と API 契約確認のための最小 Go portal stub。

testnet 向け運用手順は `nodes.testnet.example.yaml` と `../docs/testnet_runbook.md` を使うこと。

## Quickstart

SMTP なしのローカル検証モード:

```sh
go run ./cmd/portal-server \
  --listen 127.0.0.1:8080 \
  --policy ../docs/policies/program_agent_policy.v2026-04.yaml \
  --nodes-config ./nodes.example.yaml \
  --state-path ./portal-state.json \
  --nominal-daily-pool 1000 \
  --notifier-mode stdout
```

デフォルトの SMTP 利用モード:

```sh
SSNP_SMTP_PASSWORD=secret \
go run ./cmd/portal-server \
  --listen 127.0.0.1:8080 \
  --policy ../docs/policies/program_agent_policy.v2026-04.yaml \
  --nodes-config ./nodes.example.yaml \
  --state-path ./portal-state.json \
  --nominal-daily-pool 1000 \
  --email-to ops@example.invalid \
  --smtp-host smtp.example.invalid \
  --smtp-port 587 \
  --smtp-username ssnp-notify \
  --smtp-from ssnp@example.invalid
```

## Notifier Mode

- `smtp`: デフォルト。SMTP 設定と password が必要
- `stdout`: 通知を JSON として stdout に書く。ローカル確認専用
- `noop`: 通知を受けて捨てる。ローカル確認専用

## 最低限の API Surface

- `GET /api/v1/agent/policy`
- `GET /api/v1/agent/telemetry`
- `GET /api/v1/rankings/{date_utc}`
- `GET /api/v1/reward-eligibility/{date_utc}`
- `GET /api/v1/anti-concentration-evidence/{date_utc}`
- `GET /api/v1/reward-allocations/{date_utc}`
- `GET /api/v1/public-node-status/{date_utc}`
- `GET /api/v1/operator-node-status/{node_id}/{date_utc}`
- `POST /api/v1/agent/enrollment-challenges`
- `POST /api/v1/agent/enroll`
- `POST /api/v1/agent/heartbeat`
- `POST /api/v1/agent/checks`
- `POST /api/v1/agent/telemetry`
- `POST /api/v1/decentralization-evidence`
- `POST /api/v1/domain-evidence`
- `POST /api/v1/operator-group-evidence`
- `POST /api/v1/shared-control-plane-evidence`

## 実行時の制約

- known node は `--nodes-config` から読む
- runtime state は `--state-path` に保存する
- policy 読み込みは fail-closed
- snapshot JSON が壊れていると起動失敗する
- smoke 用 seed データの説明は `../testdata/smoke/README.md` を参照
- qualification には次が必要
  - 有効な probe evidence
  - 15 分以内の有効 heartbeat 2 件
  - hardware check pass
  - voting-key evidence pass
  - 72 時間 observation window

## 検証

通常の回帰検知は root の `make test`、最低限動作の end-to-end 確認は `make smoke` を使う。

```sh
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./...
```
