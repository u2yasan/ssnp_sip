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
