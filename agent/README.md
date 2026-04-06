# Program Agent Stub

SSNP Program Agent の最小 Go stub です。

対象機能:

- policy fetch
- heartbeat submit
- hardware simple check
- `cpu-check-v1`
- `disk-check-v1`
- payload submit

コマンド:

```sh
go run ./cmd/program-agent --config ./config.example.yaml run
go run ./cmd/program-agent --config ./config.example.yaml enroll --challenge-id enroll-001
go run ./cmd/program-agent --config ./config.example.yaml check --event-type registration --event-id check-001
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
- CPU 正規化スコアは stub 実装として weighted work units/sec をそのまま `normalized_cpu_score` として扱います
- disk I/O チェックは local temp file に対する bounded test です

テスト:

```sh
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./...
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go build ./...
```
