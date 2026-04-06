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
- certificate expiry upcoming
- domain expiry upcoming

## Operational Rules
- alerts must be generated from externally observed conditions where possible;
- agent-originated status is useful, but not authoritative against external evidence;
- node-local reputation or peer-selection signals may be retained as operator reference information, but must not override external probe evidence;
- notification delivery failure should be observable as an operational risk.

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
