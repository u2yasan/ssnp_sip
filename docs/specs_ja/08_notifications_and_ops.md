# 08. 通知と運用

## 目的
回避可能な qualification 脱落を減らし、ネットワーク継続性を高める。

## 必須通知種別
- node outage alerts
- finalized-lag alerts
- voting-key expiry reminders
- TLS certificate expiry reminders
- domain expiry reminders
- software update advisories
- Program Agent heartbeat failure alerts

## 運用原則
SSNP は、障害後に罰するだけでなく、運用者が健全性を維持できるよう支援すべきである。

## Delivery Channels
MVP の通知チャネル方針:
- `email` を全参加 operator の必須チャネルにする
- `webhook`, `Discord`, `Telegram` は任意の補助チャネルにする
- 複数チャネル設定は許可するが、最低基準は email のままにする

## 優先度
### Critical
- node down
- finalized lag critical
- voting key expired
- certificate expiry near term
- Program Agent missing or stale beyond threshold

### Warning
- sync lag
- stale heartbeat
- portal unreachable
- local check execution failed
- voting key expiry upcoming
- certificate expiry upcoming
- domain expiry upcoming

## 運用ルール
- 可能な限り、通知は外部観測条件から生成すべきである
- Agent 由来の状態情報は有用だが、外部証拠に優先してはならない
- node-local reputation や peer-selection signal はオペレーター向け参考情報として保持してよいが、外部 probe 証拠を上書きしてはならない
- 通知配送失敗自体も運用リスクとして観測可能であるべきである

## Delivery Policy
severity ごとの配送ルール:
- `Critical`
  - 即時送信する
  - 将来の portal 実装で acknowledge されるか、状態が解消するまで 15 分ごとに再送する
- `Warning`
  - active への状態遷移時に 1 回だけ送信する
  - 同じ node の同じ warning には 24 時間の cooldown をかける

dedupe ルール:
- `node_id + alert_code + severity` を dedupe key に使う

初期通知対象:
- heartbeat `stale`
- heartbeat `failed`
- `portal_unreachable`
- `voting_key_expiry_risk`
- `certificate_expiry_risk`
- `local_check_execution_failed`

delivery failure ルール:
- delivery failure は portal の運用イベントとして記録する
- delivery failure 自体を qualification failure として扱わない
- program operations が追跡できるよう可視化する

## v0.1 における Portal Stub の Delivery 挙動
現在の portal stub の通知配送は次のように実装されている。
- 設定で露出する配送チャネルは `email` のみ
- 実際の配送 backend は実メール送信ではなく、構造化通知出力を書くだけの stub notifier
- known node は別の seed config file から読む
- runtime state は JSON snapshot file に保存する
- portal は alert ごとの delivery attempt を runtime state に記録する
- portal は notification delivery failure を runtime state の operational event として記録する
- dedupe と cooldown の状態は snapshot 保存に成功すれば restart 後も維持される

現在の portal 側 alert 生成:
- agent から受けた telemetry warning:
  - `portal_unreachable`
  - `voting_key_expiry_risk`
  - `certificate_expiry_risk`
  - `local_check_execution_failed`
- portal 側で観測する heartbeat alert:
  - `heartbeat_stale`
  - `heartbeat_failed`

現在の portal 側 heartbeat threshold:
- `heartbeat_stale`
  - 最後に accept した heartbeat から 15 分超で発火
- `heartbeat_failed`
  - 最後に accept した heartbeat から 30 分超で発火

## Program Agent Warning Inputs In v0.1
Agent 由来 warning は、control-plane と operator 向けの補助 signal に留まる。

v0.1 の入力源:
- voting-key expiry reminders
  - ローカル `config.yaml:voting_key_expiry_at` から導出する
- TLS certificate expiry reminders
  - `monitored_endpoint` の leaf certificate expiry をローカルに確認して導出する
- portal-unreachable warning
  - agent-to-portal 通信失敗の連続から導出する
- Program Agent heartbeat failure alerts
  - agent 自身の warning telemetry ではなく、portal 側の stale/failed heartbeat 観測から導出する

運用上の区別:
- `portal_unreachable` は agent-to-portal control-plane warning である
- Program Agent heartbeat failure は portal が観測する liveness state である
- この 2 つを 1 つの運用カテゴリに潰してはならない

実装メモ:
- portal stub は内部 scan loop で heartbeat `stale` / `failed` を評価し、同じ severity-based dedupe ルールを適用する
