# 11. Program Agent Design

## Position
Program Agent is a **local node-side companion process** for SSNP v0.1.

It exists to provide:
- enrollment binding between a registered node and a local runtime instance;
- signed heartbeat for liveness assurance;
- limited supplemental telemetry for operations;
- simple hardware capability checks against recommended threshold requirements;
- local pre-failure signals such as voting-key expiry risk or certificate expiry risk.

It does **not** replace external monitoring, and it must never become the main truth source for qualification or ranking.

## Design Goals
- Require no inbound port on the voting node host
- Operate with low daily maintenance burden
- Keep the deployment model simple enough for small operators
- Minimize secret exposure and blast radius
- Produce deterministic liveness signals that can be audited

## Non-Goals
- remote shell execution;
- remote configuration mutation;
- automatic remediation with side effects;
- custody of operator private keys or wallet secrets;
- authority to override contradictory probe evidence;
- publication of raw RAM / CPU / HDD values or benchmark scores when pass/fail is sufficient;
- collection of raw host data that is not needed for SSNP operations.

## Trust Boundary
External probe infrastructure remains the primary source of truth for:
- availability;
- finalized lag;
- chain sync quality;
- public endpoint reachability.

Program Agent is only authoritative for:
- whether the local SSNP runtime instance is alive;
- whether the agent still possesses its enrolled agent key;
- whether locally observed operational warning signals were produced.

If Program Agent data conflicts with external probe data, the external probe wins.

## Threat Model
The design must assume:
- an operator can misconfigure the agent;
- an attacker can replay old heartbeat messages;
- a node host can be partially compromised;
- the portal API can receive spoofed or duplicated agent traffic;
- telemetry can leak sensitive infrastructure details if over-collected.

Therefore the design must enforce:
- signed agent messages;
- sequence or nonce-based anti-replay protection;
- short-lived enrollment credentials;
- outbound-only communication;
- minimal telemetry collection;
- separation between internal check data and public-facing pass/fail status;
- explicit rejection of privileged remote actions.

## Deployment Model
Program Agent should run:
- on the same host as the voting node; or
- on a tightly controlled host in the same operational trust boundary.

The preferred model for v0.1 is:
- one agent instance per registered node;
- one OS service process;
- automatic restart by the local service manager;
- outbound HTTPS only.

The design should not require:
- Kubernetes;
- a message broker;
- VPN-only control planes;
- inbound firewall exceptions.

## Registration and Enrollment
Enrollment must bind three things:
1. the registered node record;
2. the operator-approved registration action;
3. the specific agent instance public key.

Minimum flow:
1. Operator registers node metadata in the portal.
2. Portal issues a short-lived enrollment challenge.
3. Agent generates a local agent keypair.
4. Agent sends the public key and challenge response.
5. Portal stores the agent public key as the active enrolled identity for that node.
6. All later heartbeat and telemetry messages are signed by that agent key.

Constraints:
- the private agent key must stay local;
- the enrollment challenge must expire quickly;
- re-enrollment must revoke the previous active agent key;
- one node record must not accept simultaneous active agent identities by default.

## Agent Identity and Keys
Program Agent needs its own dedicated signing key.

Requirements:
- use a key dedicated to SSNP agent identity, not the node private key;
- never require the voting private key or harvester key;
- store the key with OS-native file permissions at minimum;
- support operator-driven key rotation;
- expose only the public key or its fingerprint to the portal.

If stronger local secure storage is available, it may be used.
It must not be required for MVP participation.

## Hardware Capability Simple Check
Program Agent may be used to check that the node environment satisfies the
recommended hardware conditions for Super Node participation without exposing exact values publicly.

Target categories:
- RAM threshold;
- CPU threshold;
- storage threshold;
- disk performance threshold.

Design rules:
- use a simple pass/fail check rather than a cryptographic proof model;
- bind the check result to the enrolled node record and current check window;
- treat the check as an eligibility condition, not a ranking signal;
- do not allow a self-declared hardware statement to replace the check;
- use the recommended Symbol Dual & Voting node baseline:
  - CPU >= 8 cores
  - RAM >= 32GB
  - Disk >= 750GB SSD
  - Disk performance >= 1500 IOPS

## CPU Load Test Policy
CPU load testing may be executed only at bounded control points:
- initial registration;
- voting-key renewal;
- explicit re-check triggered by major environment change or dispute review.

Rules:
- do not run constant or high-frequency load tests during normal operations;
- keep the test duration bounded and policy-defined;
- encode the result as pass/fail, not as a public raw score;
- do not fold load-test output into the ranking formula.

Recommended execution method:
- ship a fixed SSNP workload profile with Program Agent so every node runs the same test logic;
- use a deterministic mixed workload combining hashing and integer / matrix operations, with no external network dependency;
- run for 180 seconds total:
  - 30 seconds warm-up;
  - 120 seconds measured interval;
  - 30 seconds cool-down and result finalization;
- use up to 8 worker threads, capped by the CPU resources visible to the runtime;
- record the local profile ID, policy version, execution timestamp, and pass/fail result;
- keep detailed raw scores internal to operators and program operations if stored at all.

Recommended pass criteria:
- no runtime error or early termination;
- measured score at or above the policy-defined floor for the active workload profile;
- no mismatch between the requested test profile and the reported test profile;
- result bound to the specific registration, voting-key-renewal, or re-check event.

Recommended initial policy direction:
- start with a single profile ID such as `cpu-check-v1`;
- derive the first acceptance floor from pilot measurements across multiple VPS providers and regions;
- set the floor conservatively to reject clearly underpowered environments without excluding healthy recommended-class nodes;
- revise the floor only by policy version change, never silently.

Recommended `cpu-check-v1` defaults:
- score unit: completed work units per second during the 120-second measured interval;
- workload mix:
  - 50% deterministic hashing work;
  - 30% deterministic integer arithmetic work;
  - 20% deterministic matrix-operation work;
- worker count: `min(8, visible_cpu_threads)`;
- initial policy floor:
  - `normalized_score >= 1.00`;
  - where `1.00` is the SSNP reference baseline recorded from the program's approved reference environment for the same policy version.

Reference-environment rule:
- the reference environment must be published by policy version;
- the reference environment should track the recommended Symbol Dual & Voting node baseline;
- changing the reference environment requires a policy version bump.

Recommended reference-environment definition:
- use an SSNP-operated benchmark environment rather than a third-party provider marketing SKU;
- the environment must satisfy at least:
  - 8 dedicated vCPU or equivalent compute entitlement;
  - 32GB RAM;
  - 750GB or larger SSD-backed storage;
  - storage capable of passing `disk-check-v1`;
- the operating system image, agent version, and workload profile version must be fixed per policy version;
- the program should publish:
  - reference environment ID;
  - operating system image identifier;
  - agent version;
  - workload profile ID;
  - baseline normalized score source date.

Recommended operational approach:
- maintain at least 3 reference runs in separate regions or providers before publishing a new baseline;
- use the median measured result as the `1.00` normalization source;
- keep the raw reference measurements internal, but publish the policy metadata needed for reproducibility;
- do not redefine the baseline from a one-off exceptionally fast instance.

## Disk Performance Check Policy
Disk performance should be verified with a bounded local I/O check.

Recommended method:
- use a fixed SSNP disk profile such as `disk-check-v1`;
- run a local random-read/random-write SSD-oriented workload against a temporary file;
- use 4KiB blocks and the following initial defaults:
  - queue depth: 32;
  - concurrency: 4;
  - read/write mix: 70/30;
- run for 60 seconds total:
  - 10 seconds warm-up;
  - 40 seconds measured interval;
  - 10 seconds cool-down and result finalization;
- avoid external storage or network dependency during the test;
- record only pass/fail for the public portal.

Recommended pass criteria:
- no I/O error or early termination;
- measured result at or above the policy-defined IOPS floor for the active disk profile;
- no mismatch between the requested disk profile and the reported disk profile;
- result bound to the specific registration, voting-key-renewal, or re-check event.

Recommended initial `disk-check-v1` floor:
- `measured_iops >= 1500`;
- `measured_latency_p95` may be retained for internal operations, but it should not be exposed publicly and should not block pass/fail in v0.1 unless policy later adds it.

## Heartbeat Contract
Heartbeat exists to answer one question: "is the enrolled agent instance currently alive and still bound to this node record?"

### Required heartbeat fields
- agent public key fingerprint;
- registered node ID;
- heartbeat timestamp;
- monotonic sequence number;
- agent software version;
- enrollment generation or key version;
- local observation summary flags.

### Transmission rule
- send heartbeat every 5 minutes with small startup jitter;
- retry on transient failure with bounded backoff;
- never queue unlimited offline backlog.

### Liveness rule for v0.1
- `healthy`: at least 2 valid heartbeats received within the last 15 minutes;
- `stale`: fewer than 2 valid heartbeats in 15 minutes but at least 1 valid heartbeat in 30 minutes;
- `failed`: no valid heartbeat in the last 30 minutes.

### Validation rule
A heartbeat is valid only if:
- the signature verifies against the active enrolled agent key;
- the timestamp is within acceptable skew;
- the sequence number is newer than the last accepted sequence;
- the node record is still active.

## Supplemental Telemetry
Telemetry must be minimal and operationally relevant.

Allowed categories:
- agent process uptime;
- local node process presence check;
- current software version string;
- voting-key expiry or epoch-validity status;
- certificate expiry timestamp for the monitored endpoint;
- domain expiry reminder data if locally configured;
- hardware-check validity status;
- coarse disk-pressure or resource-warning flags.

Telemetry should be sent as:
- booleans;
- bounded enums;
- coarse buckets;
- explicit timestamps when needed for expiry logic.

Telemetry should not include:
- private keys;
- wallet seeds;
- raw config files;
- full process lists;
- shell command output;
- detailed hardware inventory unless strictly required for local check execution.

## Local Checks
Program Agent may perform local checks for warning generation, but those checks remain supplemental.

Permitted local checks:
- "node process present" check;
- "local API responding" check;
- "configured certificate near expiry" check;
- "voting key nearing invalid epoch" check;
- "hardware simple check execution" check;
- "agent cannot reach portal API" check.

Forbidden local behaviors:
- restarting services automatically without explicit operator opt-in;
- changing node configuration remotely;
- executing arbitrary commands from portal instructions;
- downloading and applying updates without operator approval.

## Warning Telemetry Semantics
The v0.1 telemetry warning set is fixed to:
- `portal_unreachable`
- `local_check_execution_failed`
- `voting_key_expiry_risk`
- `certificate_expiry_risk`

Rules:
- warning telemetry is supplemental only and must not override external probe evidence;
- warning telemetry must not be used as a ranking input;
- warning telemetry must not be described as a cryptographic truth source.

v0.1 warning generation rules:
- `portal_unreachable`
  - mark pending after 3 consecutive portal communication failures;
  - emit once after portal communication recovers;
- `local_check_execution_failed`
  - emit only when hardware / CPU / disk checks are execution failures, not normal pass/fail results;
- `voting_key_expiry_risk`
  - use `config.yaml:voting_key_expiry_at` as the v0.1 input source;
  - emit when the configured expiry is within 14 days;
- `certificate_expiry_risk`
  - use `monitored_endpoint` only when it is `https`;
  - inspect the leaf certificate `NotAfter` timestamp only;
  - emit when the expiry is within 14 days;
  - treat this as expiry metadata inspection only, not PKI trust validation.

Portal-side handling rules in the current stub:
- accepted warning telemetry may trigger operator notification handling on the portal side;
- notification dedupe uses `node_id + alert_code + severity`;
- `warning` severity uses a 24-hour cooldown;
- known nodes come from a seed config file;
- delivery and dedupe state are stored in a JSON snapshot file in v0.1;
- notification delivery failure is recorded as an operational event and must not change qualification by itself.

## Portal API Contract
The portal-side agent interface should stay minimal:
- enroll agent;
- revoke or rotate agent identity;
- receive signed heartbeat;
- receive signed warning telemetry;
- receive hardware-check result;
- fetch static agent policy metadata if needed.

Keep the protocol idempotent where possible.
Do not require long-lived interactive sessions.

## Hardware Check Payload Schema
The hardware simple check result should be sent as a bounded, versioned payload.

Required fields:
- `schema_version`: payload schema version;
- `node_id`: registered node identifier;
- `agent_key_fingerprint`: enrolled agent identity fingerprint;
- `event_type`: one of `registration`, `voting_key_renewal`, `recheck`;
- `event_id`: identifier for the specific check event;
- `policy_version`: active SSNP policy version;
- `cpu_profile_id`: active CPU workload profile, initially `cpu-check-v1`;
- `disk_profile_id`: active disk workload profile, initially `disk-check-v1`;
- `checked_at`: UTC timestamp of result finalization;
- `cpu_check_passed`: boolean;
- `disk_check_passed`: boolean;
- `ram_check_passed`: boolean;
- `storage_size_check_passed`: boolean;
- `ssd_check_passed`: boolean;
- `cpu_load_test_passed`: boolean;
- `overall_passed`: boolean derived from all required sub-checks;
- `agent_version`: Program Agent version;
- `signature`: agent signature over the payload.

Optional internal-use fields:
- `normalized_cpu_score`;
- `measured_iops`;
- `measured_latency_p95`;
- `visible_cpu_threads`;
- `visible_memory_bytes`;
- `visible_storage_bytes`;
- `error_code`;
- `error_detail`.

Public portal rule:
- expose only high-level pass/fail status and policy/profile identifiers where needed;
- do not expose raw score or raw hardware values on the public portal in v0.1.

Validation rule:
- the portal must reject payloads with mismatched schema version, missing required fields, invalid signature, or inconsistent `overall_passed` derivation.

Illustrative payload:
```json
{
  "schema_version": "1",
  "node_id": "node-abc",
  "agent_key_fingerprint": "agent-fp-123",
  "event_type": "registration",
  "event_id": "check-2026-04-06-0001",
  "policy_version": "2026-04",
  "cpu_profile_id": "cpu-check-v1",
  "disk_profile_id": "disk-check-v1",
  "checked_at": "2026-04-06T10:30:00Z",
  "cpu_check_passed": true,
  "disk_check_passed": true,
  "ram_check_passed": true,
  "storage_size_check_passed": true,
  "ssd_check_passed": true,
  "cpu_load_test_passed": true,
  "overall_passed": true,
  "agent_version": "1.0.0",
  "signature": "base64-signature"
}
```

## Program Agent API Endpoints
The v0.1 portal-side API should remain minimal and explicit.

Active policy values are expected to come from the repo-managed YAML policy file
described in `docs/specs/12_program_agent_policy_file.md`.

### `POST /api/v1/agent/enroll`
Purpose:
- bind an agent identity to a registered node record.

Request body:
- `node_id`
- `enrollment_challenge_id`
- `agent_public_key`
- `agent_version`
- `signature`

Success response:
```json
{
  "status": "ok",
  "node_id": "node-abc",
  "agent_key_fingerprint": "agent-fp-123",
  "policy_version": "2026-04"
}
```

### `POST /api/v1/agent/heartbeat`
Purpose:
- receive a signed heartbeat from the enrolled agent.

Request body:
- heartbeat payload defined by the heartbeat contract.

Success response:
```json
{
  "status": "accepted",
  "node_id": "node-abc",
  "received_at": "2026-04-06T10:35:00Z"
}
```

Portal-side operational behavior:
- the portal may derive `heartbeat_stale` and `heartbeat_failed` alerts from accepted heartbeat timestamps;
- the current stub uses:
  - `stale` after 15 minutes without an accepted heartbeat;
  - `failed` after 30 minutes without an accepted heartbeat.

### `POST /api/v1/agent/checks`
Purpose:
- receive the hardware simple check result and bounded CPU / disk test result.

Request body:
- hardware check payload defined above.

Success response:
```json
{
  "status": "accepted",
  "node_id": "node-abc",
  "event_id": "check-2026-04-06-0001",
  "overall_passed": true,
  "received_at": "2026-04-06T10:30:05Z"
}
```

### `POST /api/v1/agent/telemetry`
Purpose:
- receive supplemental warning telemetry that is not part of heartbeat or hardware checks.

Request body:
- versioned telemetry payload with signed warning fields only.

Success response:
```json
{
  "status": "accepted",
  "node_id": "node-abc",
  "received_at": "2026-04-06T10:40:00Z"
}
```

Portal-side operational behavior:
- accepted telemetry warnings may trigger notification delivery handling;
- the current stub exposes `email` as the only configured channel;
- the current stub uses a notifier backend stub rather than real email transport.

### `GET /api/v1/agent/telemetry`
Purpose:
- return stored warning telemetry for operator and program operations.

Query parameters:
- optional `node_id`;
- optional `warning_code`;
- optional `view=latest` to return only the latest warning per `node_id + warning_code`.

Response behavior:
- default view returns telemetry history items;
- `view=latest` returns the current latest view only;
- the portal stub stores telemetry and related alert state in a local JSON snapshot file in v0.1.

### `GET /api/v1/agent/policy`
Purpose:
- allow the agent to fetch the active policy version and profile identifiers.

Query parameters:
- `node_id`
- `agent_key_fingerprint`

Success response:
```json
{
  "policy_version": "2026-04",
  "heartbeat_interval_seconds": 300,
  "cpu_profile": {
    "id": "cpu-check-v1"
  },
  "disk_profile": {
    "id": "disk-check-v1"
  },
  "hardware_thresholds": {
    "cpu_cores_min": 8,
    "ram_gb_min": 32,
    "storage_gb_min": 750,
    "ssd_required": true
  },
  "reference_environment": {
    "id": "ref-env-2026-04"
  }
}
```

## API Error Handling
The portal should return narrow, machine-readable errors.

Recommended error codes:
- `invalid_schema_version`
- `missing_required_field`
- `invalid_signature`
- `unknown_node_id`
- `agent_not_enrolled`
- `policy_version_mismatch`
- `invalid_profile_id`
- `duplicate_event_id`
- `invalid_overall_passed`
- `stale_timestamp`
- `rate_limited`

Recommended error response:
```json
{
  "status": "error",
  "error_code": "invalid_signature",
  "message": "signature verification failed"
}
```

## API Design Rules
- all endpoints must require TLS;
- all write endpoints must be idempotent on duplicate `event_id` or equivalent request identity;
- the portal must log acceptance and rejection decisions with reason codes;
- the portal must not infer pass/fail from missing fields;
- policy/profile mismatch must be rejected explicitly, not coerced silently.

## Failure Semantics
Program Agent failure affects participation, but it does not rewrite external evidence.

Rules:
- a failed agent heartbeat can block or remove `Qualified` status if the policy requires active agent liveness;
- an invalid or expired hardware check can block registration, voting-key renewal, or Qualified status;
- an external outage remains an outage even if the agent claims the node is healthy;
- portal ingestion failure must be observable separately from node failure;
- operator-visible reason codes must distinguish:
  - agent missing;
  - agent stale;
  - invalid signature;
  - invalid hardware check;
  - enrollment revoked;
  - portal delivery failure.

## Security Requirements
- outbound-only communication by default;
- mutually authenticated identity at least at the message-signature layer;
- TLS for transport;
- replay resistance;
- strict input validation on portal ingestion;
- per-node rate limits;
- audit logging for enrollment, key rotation, revocation, invalid heartbeat rejection, and hardware-check rejection.

The portal must never trust unsigned agent messages.

## Privacy and Data Minimization
SSNP should not turn Program Agent into host surveillance.

Therefore:
- collect only data directly needed for liveness and operational warnings;
- expose public pass/fail hardware status instead of raw hardware values or benchmark scores;
- avoid exposing raw infrastructure details publicly;
- separate operator-only details from public portal data;
- document every telemetry field and why it exists.

## Operational Model
For low-cost operations:
- ship a single-purpose agent package or binary per platform;
- keep configuration file format flat and explicit;
- prefer pull-free operation after enrollment;
- log locally in plain text or structured text;
- make failure modes obvious enough for non-expert operators.

Daily operations should mostly involve:
- service health;
- key rotation when needed;
- version updates;
- voting-key renewal and hardware-check renewal when scheduled;
- resolving warning alerts.

## Governance Boundary
Program Agent may support:
- liveness assurance;
- operational warnings;
- operator-facing diagnostics.

Program Agent must not hold final authority over:
- reward eligibility;
- Same Operator Group classification;
- operator disqualification;
- payment approval.

## Recommended v0.1 Decision
For v0.1, Program Agent should remain mandatory because:
- it gives deterministic liveness signals;
- it closes the gap between external monitoring and local operational warnings;
- it is operationally cheaper than building equivalent evidence from manual attestations.

Future versions may relax this only if an equivalent low-trust replacement exists.

## Open Implementation Work
- define exact timestamp skew tolerance;
- define agent public key format and signature scheme;
- define node ID canonicalization;
- define enrollment challenge format;
- define the local hardware-check method for CPU cores, RAM, disk size, SSD, and IOPS;
- define voting-key-renewal semantics for re-check;
- publish the approved reference environment for `cpu-check-v1`;
- define telemetry schema versioning;
- define re-enrollment grace period and operator notification flow.
