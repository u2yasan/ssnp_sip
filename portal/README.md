# Portal Stub

SSNP Program Agent と結合するための最小 Go portal stub です。

対象 endpoint:

- `GET /api/v1/agent/policy`
- `GET /api/v1/agent/telemetry`
- `POST /api/v1/agent/enroll`
- `POST /api/v1/agent/heartbeat`
- `POST /api/v1/agent/checks`
- `POST /api/v1/agent/telemetry`

制約:

- state は in-memory のみです
- known node は `node-abc` の固定 fixture だけです
- policy は `--policy` で与えた YAML を起動時に読みます
- policy 読込失敗時は起動失敗します
- enrollment challenge は空でない文字列かどうかだけ見ます
- `policy_version`、profile ID、heartbeat sequence、signature は fail-closed で検証します
- telemetry は履歴一覧と latest view を in-memory で参照できます

起動:

```sh
go run ./cmd/portal-server --listen 127.0.0.1:8080 --policy ../docs/policies/program_agent_policy.v2026-04.yaml
```

agent と疎通する例:

1. `agent/config.example.yaml` の `portal_base_url` を `http://127.0.0.1:8080` に変える
2. portal stub を起動する
3. agent を enroll する

```sh
go run ./cmd/program-agent --config ./config.example.yaml enroll --challenge-id enroll-001
```

4. heartbeat loop を起動する

```sh
go run ./cmd/program-agent --config ./config.example.yaml run
```

5. hardware simple check を送る

```sh
go run ./cmd/program-agent --config ./config.example.yaml check --event-type registration --event-id check-001
```

6. telemetry を参照する

```sh
curl "http://127.0.0.1:8080/api/v1/agent/telemetry?node_id=node-abc"
curl "http://127.0.0.1:8080/api/v1/agent/telemetry?view=latest"
```
