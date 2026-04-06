# Program Agent Stub

SSNP Program Agent の最小 Go stub です。

対象機能:

- policy fetch
- heartbeat submit
- hardware simple check
- `cpu-check-v1`
- `disk-check-v1`
- payload submit
- telemetry submit

コマンド:

```sh
go run ./cmd/program-agent --config ./config.example.yaml run
go run ./cmd/program-agent --config ./config.example.yaml enroll --challenge-id enroll-001
go run ./cmd/program-agent --config ./config.example.yaml check --event-type registration --event-id check-001
go run ./cmd/program-agent --config ./config.example.yaml telemetry --warning-flag voting_key_expiry_risk
```

注意:

- `go` 実行環境が必要です
- policy は portal の `GET /api/v1/agent/policy` から取得します
- policy fetch に失敗した場合、embedded default には fallback しません
- `GET /api/v1/agent/policy` が `5xx` を返した場合、agent は起動失敗します
- policy response が壊れた JSON の場合、agent は起動失敗します
- `state.json` が壊れている場合、heartbeat は失敗し、sequence を巻き戻したり再初期化したりしません
- portal が `policy_version mismatch` などの `4xx/5xx` で `checks` を reject した場合、agent はそのまま失敗を返します
- invalid signature や timeout のような transport / portal error も握り潰しません
- telemetry は `--warning-flag` の repeatable CLI 指定で送信します
- telemetry の自動生成は v0.1 では `portal_unreachable`, `local_check_execution_failed`, `voting_key_expiry_risk`, `certificate_expiry_risk` です
- `portal_unreachable` は portal 通信失敗が連続 3 回に達した後、回復時に 1 回だけ送信します
- `local_check_execution_failed` は hardware / CPU / disk check が測定不能だった時だけ送信します
- `voting_key_expiry_risk` は `voting_key_expiry_at` が 14 日未満の時に 1 回だけ送信します
- `certificate_expiry_risk` は `monitored_endpoint` が `https` の時だけ leaf certificate の期限を見て、14 日未満なら 1 回だけ送信します
- certificate expiry check は期限確認専用です。v0.1 では CA/hostname の妥当性検証は行いません
- CPU 正規化スコアは stub 実装として weighted work units/sec をそのまま `normalized_cpu_score` として扱います
- disk I/O チェックは local temp file に対する bounded test です

テスト:

```sh
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./...
```
