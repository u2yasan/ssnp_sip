# 08. Notifications and Operations

## Purpose
Reduce avoidable qualification loss and improve network continuity.

## Required Notification Types
- node outage alerts
- finalized-lag alerts
- voting-key expiry reminders
- TLS certificate expiry reminders
- domain expiry reminders
- software update advisories
- Program Agent heartbeat failure alerts

## Operational Principle
SSNP should help operators remain healthy rather than only penalizing them after failure.

## Delivery Channels
MVP notification channel policy:
- `email` is mandatory for every participating operator;
- `webhook`, `Discord`, and `Telegram` are optional supplemental channels;
- operators may configure multiple channels, but email remains the minimum baseline.

## Priority Levels
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

## Operational Rules
- alerts must be generated from externally observed conditions where possible;
- agent-originated status is useful, but not authoritative against external evidence;
- node-local reputation or peer-selection signals may be retained as operator reference information, but must not override external probe evidence;
- notification delivery failure should be observable as an operational risk.

## Delivery Policy
Severity-based delivery rules:
- `Critical`
  - send immediately;
  - resend every 15 minutes until the condition clears or is acknowledged by a future portal implementation;
- `Warning`
  - send once on state transition to active;
  - apply a 24-hour cooldown before sending the same warning again for the same node.

Dedupe rule:
- use `node_id + alert_code + severity` as the dedupe key.

Initial notification target set:
- heartbeat `stale`
- heartbeat `failed`
- `portal_unreachable`
- `voting_key_expiry_risk`
- `certificate_expiry_risk`
- `local_check_execution_failed`

Delivery failure rule:
- record delivery failure as a portal operational event;
- do not treat delivery failure itself as a qualification failure;
- make delivery failure visible to program operations for follow-up.

## Portal Stub Delivery Behavior In v0.1
The current portal stub implements notification delivery as follows:
- `email` is the only delivery channel exposed by configuration;
- the delivery backend is a stub notifier that writes structured notification output instead of sending real email;
- the portal records per-alert delivery attempts in memory;
- the portal records notification delivery failure as an operational event in memory;
- dedupe and cooldown state are in memory only, so restart clears alert-delivery history.

Current portal-side alert generation:
- telemetry warnings received from the agent:
  - `portal_unreachable`
  - `voting_key_expiry_risk`
  - `certificate_expiry_risk`
  - `local_check_execution_failed`
- portal-observed heartbeat alerts:
  - `heartbeat_stale`
  - `heartbeat_failed`

Current portal-side heartbeat thresholds:
- `heartbeat_stale`
  - triggered when the last accepted heartbeat is older than 15 minutes;
- `heartbeat_failed`
  - triggered when the last accepted heartbeat is older than 30 minutes.

## Program Agent Warning Inputs In v0.1
Agent-originated warnings are supplemental control-plane and operator signals only.

Current v0.1 input sources:
- voting-key expiry reminders
  - derived from local `config.yaml:voting_key_expiry_at`;
- TLS certificate expiry reminders
  - derived from local inspection of the `monitored_endpoint` leaf certificate expiry;
- portal-unreachable warning
  - derived from repeated agent-to-portal communication failures;
- Program Agent heartbeat failure alerts
  - derived from portal-side stale/failed heartbeat observation, not from the agent's own warning telemetry.

Operational distinction:
- `portal_unreachable` is an agent-to-portal control-plane warning;
- Program Agent heartbeat failure is a portal-observed liveness state;
- these must not be collapsed into a single operational category.

Implementation note:
- the portal stub currently evaluates heartbeat `stale` / `failed` by an internal scan loop and applies the same severity-based dedupe rules to those alerts.
