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
- certificate expiry upcoming
- domain expiry upcoming

## 運用ルール
- 可能な限り、通知は外部観測条件から生成すべきである
- Agent 由来の状態情報は有用だが、外部証拠に優先してはならない
- node-local reputation や peer-selection signal はオペレーター向け参考情報として保持してよいが、外部 probe 証拠を上書きしてはならない
- 通知配送失敗自体も運用リスクとして観測可能であるべきである
