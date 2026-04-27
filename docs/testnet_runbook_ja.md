[English](testnet_runbook.md) | 日本語

# Testnet Runbook

## 目的

この runbook は、現在の SSNP stub を Symbol testnet に対して運用するための、最低限サポートされる手順を定義する。

これは rollout/proving 用の基盤であり、本番向け hardening ではない。

## サポート範囲

- portal seed config による静的 node onboarding
- Program Agent enrollment と heartbeat
- recurring probe event を投稿する単純な external probe worker
- ranking、qualification、reward-eligibility、reward-allocation の read view
- SMTP 通知、または `stdout` dry-run 通知

## 明示的に延期しているもの

- self-service registration UI または registration write API
- multi-region probe fleet management
- production-grade retry/error taxonomy
- webhook、Discord、Telegram 通知 backend
- reward-funding governance の完了
- deep validation hardening

## 必須ファイル

- Go release bundle:
  - `portal-server-linux-amd64`
  - `probe-worker-linux-amd64`
  - `program_agent_policy.v2026-04.yaml`
  - `nodes.testnet.example.yaml`
  - `probe.config.testnet.example.yaml`
  - `ssnp-portal.service`
  - `ssnp-probe.service`
  - `install-go-release.sh`
  - `SHA256SUMS`
- agent config: `agent/config.testnet.example.yaml`

portal nodes config、agent config、probe config では同じ `node_id` を使うこと。

## Operator Flow

1. CI または開発機で Go release bundle を作る。
2. server に release bundle を転送する。
3. `install-go-release.sh` で portal/probe binary と systemd unit を配置する。
4. portal nodes config に node を追加する。
5. portal を起動する。
6. agent key を生成する。
7. enrollment challenge を発行する。
8. agent を enroll する。
9. agent loop を開始する。
10. `region_id` が異なる probe-worker instance を最低 2 つ起動する。
11. read view にデータが入っていることを確認する。

## Example Commands

Go release bundle を作る。これは server ではなく CI または開発機で実行する:

```sh
./scripts/build-go-release.sh
```

server 側で bundle を検証する:

```sh
cd /path/to/go-release
sha256sum -c SHA256SUMS
```

server 側に portal/probe binary と systemd unit を install する:

```sh
sudo ./install-go-release.sh /path/to/go-release
```

portal の listen address を設定する。外部 probe/agent から直接接続させる場合は `0.0.0.0:<port>` を使う:

```sh
sudo editor /etc/ssnp-portal/portal.env
```

例:

```sh
SSNP_PORTAL_LISTEN=0.0.0.0:18080
```

firewall ではこの port だけを開ける:

```sh
sudo ufw allow 18080/tcp
sudo ufw status
```

portal node seed を編集する:

```sh
sudo editor /etc/ssnp-portal/nodes.testnet.yaml
```

probe config を編集する。2 worker 以上を使う場合、instance ごとに `region_id` を変える:

```sh
sudo editor /etc/ssnp-probe/config.yaml
```

portal を dry-run 通知モードで起動する。現在の unit は `--notifier-mode stdout` 固定:

```sh
sudo systemctl start ssnp-portal
sudo systemctl status ssnp-portal
```

現在の `ssnp-portal.service` は `/etc/ssnp-portal/portal.env` の `SSNP_PORTAL_LISTEN` を `--listen` に渡す。port を変えた場合は probe config の `portal_base_url` も同じ port に合わせる。

enrollment challenge を発行する:

```sh
curl -sS \
  -H 'Content-Type: application/json' \
  -d '{"node_id":"node-testnet-001"}' \
  http://127.0.0.1:18080/api/v1/agent/enrollment-challenges
```

Python agent client は、オペレーター node 側で動く CLI/service である。portal から発行された enrollment challenge に対して agent key で署名し、その後 heartbeat、hardware check、telemetry を portal に送る。

testnet server では repository clone や `pip install -e .` を使わず、GitHub Release から agent wheel bundle を取得して install する。agent wheel bundle の作成と配布は `docs/agent_py_distribution.md` を見ること。

agent wheel bundle を取得する:

```sh
mkdir -p /tmp/ssnp-agent-release
cd /tmp/ssnp-agent-release

gh release download agent-v<version> \
  --repo u2yasan/ssnp_sip \
  --dir .
```

checksum を確認する:

```sh
sha256sum -c SHA256SUMS
```

bundle に含まれる installer で agent wheel を install する:

```sh
chmod +x install-agent-py-wheel.sh
sudo ./install-agent-py-wheel.sh ./ssnp_agent_client-<version>-py3-none-any.whl
```

agent config を編集する:

```sh
sudo editor /etc/ssnp-agent/config.yaml
```

agent key を生成する:

```sh
sudo -u ssnp-agent /opt/ssnp-agent/.venv/bin/ssnp-agent \
  --config /etc/ssnp-agent/config.yaml \
  gen-key \
  --out-dir /etc/ssnp-agent/keys
```

agent を enroll する:

```sh
sudo -u ssnp-agent /opt/ssnp-agent/.venv/bin/ssnp-agent \
  --config /etc/ssnp-agent/config.yaml \
  enroll \
  --challenge-id <challenge-id>
```

agent loop を開始する:

```sh
sudo systemctl start ssnp-agent
sudo systemctl status ssnp-agent
```

probe worker を起動する:

```sh
sudo systemctl start ssnp-probe
sudo systemctl status ssnp-probe
```

Qualification には、同一 UTC 日内に最低 2 region からの evidence が必要。worker instance 1 つでは足りない。2 instance 目は別 server へ同じ bundle を install し、`/etc/ssnp-probe/config.yaml` の `region_id` だけを別値にする。

## Validation Reads

次を確認する:

- `GET /api/v1/public-node-status/{date_utc}`
- `GET /api/v1/operator-node-status/{node_id}/{date_utc}`
- `GET /api/v1/rankings/{date_utc}`
- `GET /api/v1/reward-eligibility/{date_utc}`
- `GET /api/v1/reward-allocations/{date_utc}`

## Notification Modes

- 実際に operator へ mail delivery する場合は `smtp` を使う
- dry-run verification では `stdout` を使う
- alert を観測したい場合は `noop` を使わない

## Operational Boundary

現在の probe worker は意図的に最小実装である:

- single process
- fixed poll interval
- fixed request timeout
- persisted retry queue なし
- source endpoint failure は cycle を abort する
- target endpoint failure は `availability_up = false` として投稿する

この境界が許容できない場合、この stub はその use case にはまだ使えない。
