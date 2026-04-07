# Portal Stub

SSNP Program Agent と結合するための最小 Go portal stub です。

対象 endpoint:

- `GET /api/v1/agent/policy`
- `GET /api/v1/agent/telemetry`
- `GET /api/v1/rankings/{date_utc}`
- `GET /api/v1/reward-eligibility/{date_utc}`
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

制約:

- known node は `--nodes-config` で与えた seed config から読みます
- runtime state は `--state-path` の JSON snapshot に保存します
- policy は `--policy` で与えた YAML を起動時に読みます
- policy 読込失敗時は起動失敗します
- nodes config 読込失敗時は起動失敗します
- snapshot が壊れていたら起動失敗します
- enrollment challenge は portal 側で発行される short-lived challenge を使います
- `policy_version`、profile ID、heartbeat sequence、signature は fail-closed で検証します
- heartbeat は `enrollment_generation` を fail-closed で検証します
- heartbeat qualification は「15分内に有効 heartbeat 2件」を使います
- qualification には minimum 72-hour observation window を適用します
- telemetry は履歴一覧と latest view を返し、runtime state に保存されます
- read API contract は `../docs/openapi/portal_read_api.yaml` を正とします
- `rankings` / `reward-eligibility` / `reward-allocations` / `public-node-status` は未計算日の場合でも `200` + empty `items` を返します
- ranking の同点解消順は `finalization score`、`availability score`、`validated registration time`、`node_id` です
- ranking は `S = 0.7 * B + 0.3 * D` を使います
- `reward-eligibility` は same operator group と same registrable domain を hard filter として使います
- `reward-allocations` は `--nominal-daily-pool` と participation adjustment / rank band allocation を使って計算されます
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
  --nominal-daily-pool 1000 \
  --smtp-host smtp.example.invalid \
  --smtp-port 587 \
  --smtp-username ssnp-notify \
  --smtp-from ssnp@example.invalid \
  --email-to ops@example.invalid
```

通知関連 flag:

- `--nodes-config`
- `--state-path`
- `--nominal-daily-pool`
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
3. enrollment challenge を発行する

```sh
curl -X POST "http://127.0.0.1:8080/api/v1/agent/enrollment-challenges" \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-abc"}'
```

4. agent を enroll する

```sh
go run ./cmd/program-agent --config ./config.example.yaml enroll --challenge-id <issued-challenge-id>
```

5. heartbeat loop を起動する

```sh
go run ./cmd/program-agent --config ./config.example.yaml run
```

6. hardware simple check を送る

```sh
go run ./cmd/program-agent --config ./config.example.yaml check --event-type registration --event-id check-001
```

7. telemetry と reward views を参照する

```sh
curl "http://127.0.0.1:8080/api/v1/agent/telemetry?node_id=node-abc"
curl "http://127.0.0.1:8080/api/v1/agent/telemetry?view=latest"
curl "http://127.0.0.1:8080/api/v1/rankings/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/reward-eligibility/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/reward-allocations/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/public-node-status/2026-04-07"
curl "http://127.0.0.1:8080/api/v1/operator-node-status/node-abc/2026-04-07"
```

開発メモ:

- `internal/server/server.go` は HTTP entrypoint と route wiring のみです
- `internal/server/agent_handlers.go` は agent write/read endpoint を持ちます
- `internal/server/evidence_handlers.go` は probe/evidence write と read endpoint を持ちます
- `internal/server/qualification.go` は qualification / ranking / reward 計算を持ちます
- `internal/server/alerts.go` は alert scan と notification delivery を持ちます
- `internal/server/read_views.go` と `internal/server/http_helpers.go` は read model / HTTP helper を分離しています
