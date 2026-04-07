# Portal Stub

SSNP Program Agent と結合するための最小 Go portal stub です。

対象 endpoint:

- `GET /api/v1/agent/policy`
- `GET /api/v1/agent/telemetry`
- `GET /api/v1/rankings/{date_utc}`
- `GET /api/v1/reward-eligibility/{date_utc}`
- `GET /api/v1/public-node-status/{date_utc}`
- `GET /api/v1/operator-node-status/{node_id}/{date_utc}`
- `POST /api/v1/agent/enroll`
- `POST /api/v1/agent/heartbeat`
- `POST /api/v1/agent/checks`
- `POST /api/v1/agent/telemetry`

制約:

- known node は `--nodes-config` で与えた seed config から読みます
- runtime state は `--state-path` の JSON snapshot に保存します
- policy は `--policy` で与えた YAML を起動時に読みます
- policy 読込失敗時は起動失敗します
- nodes config 読込失敗時は起動失敗します
- snapshot が壊れていたら起動失敗します
- enrollment challenge は空でない文字列かどうかだけ見ます
- `policy_version`、profile ID、heartbeat sequence、signature は fail-closed で検証します
- telemetry は履歴一覧と latest view を返し、runtime state に保存されます
- read API contract は `../docs/openapi/portal_read_api.yaml` を正とします
- `rankings` / `reward-eligibility` / `public-node-status` は未計算日の場合でも `200` + empty `items` を返します
- `operator-node-status` は `unknown_node_id` と `missing_qualified_decision` を `404` で返します
- `public-node-status` は公開最小 view であり、`failure_reasons` や `operator_group_id` を返しません
- `operator-node-status` は node 単位の運営確認用 view であり、内部診断フィールドを返します
- notification channel は `email` のみです
- email delivery は SMTP + STARTTLS 前提です
- SMTP password は `SSNP_SMTP_PASSWORD` 環境変数から読みます
- node の `operator_email` があればそれを優先し、無ければ `--email-to` fallback を使います
- heartbeat `stale` / `failed` は portal 側 scan で検出します
- delivery failure は portal operational event として runtime state に記録します

起動:

```sh
SSNP_SMTP_PASSWORD=secret \
go run ./cmd/portal-server \
  --listen 127.0.0.1:8080 \
  --policy ../docs/policies/program_agent_policy.v2026-04.yaml \
  --nodes-config ./nodes.example.yaml \
  --state-path ./portal-state.json \
  --smtp-host smtp.example.invalid \
  --smtp-port 587 \
  --smtp-username ssnp-notify \
  --smtp-from ssnp@example.invalid \
  --email-to ops@example.invalid
```

通知関連 flag:

- `--nodes-config`
- `--state-path`
- `--email-to`
- `--smtp-host`
- `--smtp-port`
- `--smtp-username`
- `--smtp-from`
- `--heartbeat-stale-after-seconds`
- `--heartbeat-failed-after-seconds`
- `--alert-scan-interval-seconds`

seed config 例:

```yaml
nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
```

`operator_email` は optional です。空の場合は `--email-to` fallback に流れます。

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
curl "http://127.0.0.1:8080/api/v1/rankings/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/reward-eligibility/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/public-node-status/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/operator-node-status/node-abc/2026-04-07"
```
