from __future__ import annotations

import json
import random
import signal
import socket
import ssl
import sys
import time
import urllib.parse
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta

from cryptography.hazmat.primitives.asymmetric import ed25519

from .checks import (
    local_check_execution_failed,
    run_cpu,
    run_disk,
    run_hardware,
)
from .config import Config
from .crypto import fingerprint, load_private_key, load_public_key, public_key_bytes, sign_hex
from .http_client import HTTPClient
from .policy import PolicyClient
from .state import State
from .symbol import SymbolClient

WARNING_PORTAL_UNREACHABLE = "portal_unreachable"
WARNING_LOCAL_CHECK_EXEC_FAILED = "local_check_execution_failed"
WARNING_VOTING_KEY_EXPIRY_RISK = "voting_key_expiry_risk"
WARNING_CERTIFICATE_EXPIRY_RISK = "certificate_expiry_risk"
PORTAL_FAILURE_THRESHOLD = 3
VOTING_KEY_RISK_WINDOW = timedelta(days=14)
CERTIFICATE_RISK_WINDOW = timedelta(days=14)
MAX_HEARTBEAT_ATTEMPTS = 3


@dataclass(slots=True)
class Agent:
    cfg: Config
    http: HTTPClient
    policy_client: PolicyClient
    symbol_client: SymbolClient
    private_key: ed25519.Ed25519PrivateKey
    public_key: ed25519.Ed25519PublicKey
    agent_fingerprint: str

    @classmethod
    def from_config(cls, cfg: Config) -> "Agent":
        private_key = load_private_key(cfg.agent_key_path)
        public_key = load_public_key(cfg.agent_public_key_path)
        return cls(
            cfg=cfg,
            http=HTTPClient(cfg.portal_base_url, cfg.request_timeout_seconds),
            policy_client=PolicyClient(cfg.portal_base_url, cfg.request_timeout_seconds),
            symbol_client=SymbolClient(cfg.monitored_endpoint, cfg.request_timeout_seconds),
            private_key=private_key,
            public_key=public_key,
            agent_fingerprint=fingerprint(public_key),
        )

    def enroll(self, challenge_id: str) -> None:
        payload = {
            "node_id": self.cfg.node_id,
            "enrollment_challenge_id": challenge_id,
            "agent_public_key": public_key_bytes(self.public_key).hex(),
            "agent_version": self.cfg.agent_version,
        }
        payload["signature"] = self._sign_map(payload)
        self.http.post_json("/api/v1/agent/enroll", payload)

    def run_checks(self, event_type: str, event_id: str) -> None:
        if event_type not in {"registration", "voting_key_renewal", "recheck"}:
            raise ValueError(f"invalid event type: {event_type}")
        policy = self.policy_client.fetch(self.cfg.node_id, self.agent_fingerprint)
        hardware = run_hardware(self.cfg.temp_dir, policy.hardware_thresholds)
        cpu_result = run_cpu(policy.cpu_profile)
        disk_result = run_disk(self.cfg.temp_dir, policy.disk_profile)
        if local_check_execution_failed(hardware, cpu_result, disk_result):
            try:
                self._mark_warning(policy.policy_version, WARNING_LOCAL_CHECK_EXEC_FAILED)
            except Exception:
                pass
        overall = all(
            [
                hardware.cpu_check_passed,
                hardware.ram_check_passed,
                hardware.storage_size_check_passed,
                hardware.ssd_check_passed,
                cpu_result.passed,
                disk_result.passed,
            ]
        )
        payload = {
            "schema_version": "1",
            "node_id": self.cfg.node_id,
            "agent_key_fingerprint": self.agent_fingerprint,
            "event_type": event_type,
            "event_id": event_id,
            "policy_version": policy.policy_version,
            "cpu_profile_id": policy.cpu_profile.id,
            "disk_profile_id": policy.disk_profile.id,
            "checked_at": _time_now_rfc3339(),
            "cpu_check_passed": hardware.cpu_check_passed,
            "disk_check_passed": disk_result.passed,
            "ram_check_passed": hardware.ram_check_passed,
            "storage_size_check_passed": hardware.storage_size_check_passed,
            "ssd_check_passed": hardware.ssd_check_passed,
            "cpu_load_test_passed": cpu_result.passed,
            "overall_passed": overall,
            "agent_version": self.cfg.agent_version,
            "normalized_cpu_score": cpu_result.normalized_score,
            "measured_iops": disk_result.measured_iops,
            "measured_latency_p95": disk_result.measured_latency_p95,
            "visible_cpu_threads": hardware.visible_cpu_threads,
            "visible_memory_bytes": hardware.visible_memory_bytes,
            "visible_storage_bytes": hardware.visible_storage_bytes,
        }
        payload["signature"] = self._sign_map(payload)
        self.http.post_json("/api/v1/agent/checks", payload)
        json.dump(
            {
                "event_id": event_id,
                "policy_version": policy.policy_version,
                "cpu_check_passed": hardware.cpu_check_passed,
                "disk_check_passed": disk_result.passed,
                "overall_passed": overall,
                "submitted": True,
            },
            sys.stdout,
        )
        sys.stdout.write("\n")

    def submit_telemetry(self, warning_flags: list[str]) -> None:
        if not warning_flags:
            raise ValueError("missing warning flags")
        policy = self.policy_client.fetch(self.cfg.node_id, self.agent_fingerprint)
        self._submit_telemetry_with_policy(policy.policy_version, warning_flags)

    def run(self) -> None:
        policy = self.policy_client.fetch(self.cfg.node_id, self.agent_fingerprint)
        state = State.load(self.cfg.state_path)
        state.agent_key_fingerprint = self.agent_fingerprint
        state.last_policy_version = policy.policy_version
        state.save(self.cfg.state_path)

        if self.cfg.heartbeat_jitter_seconds_max > 0:
            time.sleep(random.randint(0, self.cfg.heartbeat_jitter_seconds_max))

        stop = False

        def handle_signal(_signum: int, _frame: object) -> None:
            nonlocal stop
            stop = True

        signal.signal(signal.SIGTERM, handle_signal)
        signal.signal(signal.SIGINT, handle_signal)

        while not stop:
            try:
                self._run_recurring_checks(policy.policy_version)
            except Exception:
                pass
            try:
                self._send_heartbeat_with_retry()
                self._handle_portal_recovery(policy.policy_version)
            except Exception:
                self._record_portal_failure()
            slept = 0.0
            while not stop and slept < policy.heartbeat_interval_seconds:
                time.sleep(0.2)
                slept += 0.2

    def _run_recurring_checks(self, policy_version: str) -> None:
        self._maybe_emit_voting_key_expiry_risk(policy_version)
        self._maybe_emit_certificate_expiry_risk(policy_version)

    def _send_heartbeat(self) -> None:
        state = State.load(self.cfg.state_path)
        if (
            state.agent_key_fingerprint
            and state.agent_key_fingerprint != self.agent_fingerprint
        ):
            raise RuntimeError("state fingerprint mismatch")
        payload = {
            "node_id": self.cfg.node_id,
            "agent_key_fingerprint": self.agent_fingerprint,
            "heartbeat_timestamp": _time_now_rfc3339(),
            "sequence_number": state.sequence_number + 1,
            "agent_version": self.cfg.agent_version,
            "enrollment_generation": self.cfg.enrollment_generation,
            "local_observation_flags": self._collect_local_observation_flags(),
        }
        payload["signature"] = self._sign_heartbeat(payload)
        self.http.post_json("/api/v1/agent/heartbeat", payload)
        state.sequence_number += 1
        state.agent_key_fingerprint = self.agent_fingerprint
        state.save(self.cfg.state_path)

    def _send_heartbeat_with_retry(self) -> None:
        last_error: Exception | None = None
        backoff = 1
        for attempt in range(1, MAX_HEARTBEAT_ATTEMPTS + 1):
            try:
                self._send_heartbeat()
                return
            except Exception as err:  # noqa: BLE001
                last_error = err
                if attempt == MAX_HEARTBEAT_ATTEMPTS:
                    break
                time.sleep(backoff)
                backoff = min(backoff * 2, 4)
        if last_error is not None:
            raise last_error

    def _submit_telemetry_with_policy(
        self, policy_version: str, warning_flags: list[str]
    ) -> None:
        payload = {
            "schema_version": "1",
            "node_id": self.cfg.node_id,
            "agent_key_fingerprint": self.agent_fingerprint,
            "telemetry_timestamp": _time_now_rfc3339(),
            "policy_version": policy_version,
            "warning_flags": warning_flags,
        }
        payload["signature"] = self._sign_map(payload)
        self.http.post_json("/api/v1/agent/telemetry", payload)
        json.dump(
            {
                "policy_version": policy_version,
                "warning_flags": warning_flags,
                "submitted": True,
            },
            sys.stdout,
        )
        sys.stdout.write("\n")

    def _record_portal_failure(self) -> None:
        state = State.load(self.cfg.state_path)
        state.consecutive_portal_failures += 1
        if (
            state.consecutive_portal_failures >= PORTAL_FAILURE_THRESHOLD
            and not state.active_warnings.get(WARNING_PORTAL_UNREACHABLE, False)
        ):
            state.pending_warnings[WARNING_PORTAL_UNREACHABLE] = True
        state.save(self.cfg.state_path)

    def _handle_portal_recovery(self, policy_version: str) -> None:
        state = State.load(self.cfg.state_path)
        state.consecutive_portal_failures = 0
        if state.pending_warnings.get(WARNING_PORTAL_UNREACHABLE, False) and not state.active_warnings.get(
            WARNING_PORTAL_UNREACHABLE, False
        ):
            self._submit_telemetry_with_policy(
                policy_version, [WARNING_PORTAL_UNREACHABLE]
            )
            state.active_warnings[WARNING_PORTAL_UNREACHABLE] = True
            state.pending_warnings.pop(WARNING_PORTAL_UNREACHABLE, None)
        state.save(self.cfg.state_path)

    def _mark_warning(self, policy_version: str, warning_flag: str) -> None:
        state = State.load(self.cfg.state_path)
        if state.active_warnings.get(warning_flag) or state.pending_warnings.get(warning_flag):
            return
        try:
            self._submit_telemetry_with_policy(policy_version, [warning_flag])
        except Exception:
            state.pending_warnings[warning_flag] = True
            state.save(self.cfg.state_path)
            raise
        state.active_warnings[warning_flag] = True
        state.save(self.cfg.state_path)

    def _maybe_emit_voting_key_expiry_risk(self, policy_version: str) -> None:
        try:
            status = self.symbol_client.has_voting_key_expiry_risk(VOTING_KEY_RISK_WINDOW)
        except Exception:
            return
        if status.near_expiry:
            self._mark_warning(policy_version, WARNING_VOTING_KEY_EXPIRY_RISK)

    def _maybe_emit_certificate_expiry_risk(self, policy_version: str) -> None:
        endpoint = self.cfg.monitored_endpoint.strip()
        if not endpoint:
            return
        parsed = urllib.parse.urlparse(endpoint)
        if parsed.scheme != "https":
            return
        not_after = _fetch_leaf_certificate_not_after(parsed)
        if not not_after:
            return
        if not_after - datetime.now(UTC) < CERTIFICATE_RISK_WINDOW:
            self._mark_warning(policy_version, WARNING_CERTIFICATE_EXPIRY_RISK)

    def _collect_local_observation_flags(self) -> list[str]:
        flags: list[str] = []
        try:
            status = self.symbol_client.has_voting_key_expiry_risk(VOTING_KEY_RISK_WINDOW)
        except Exception:
            flags.append("local_api_unreachable")
        else:
            if status.near_expiry:
                flags.append(WARNING_VOTING_KEY_EXPIRY_RISK)
        endpoint = self.cfg.monitored_endpoint.strip()
        if endpoint:
            parsed = urllib.parse.urlparse(endpoint)
            if parsed.scheme == "https":
                not_after = _fetch_leaf_certificate_not_after(parsed)
                if not_after and not_after - datetime.now(UTC) < CERTIFICATE_RISK_WINDOW:
                    flags.append(WARNING_CERTIFICATE_EXPIRY_RISK)
        state = State.load(self.cfg.state_path)
        if state.pending_warnings.get(WARNING_PORTAL_UNREACHABLE) or state.active_warnings.get(
            WARNING_PORTAL_UNREACHABLE
        ):
            flags.append(WARNING_PORTAL_UNREACHABLE)
        return flags

    def _sign_map(self, payload: dict) -> str:
        copy = {key: value for key, value in payload.items() if key != "signature"}
        return sign_hex(
            self.private_key,
            json.dumps(copy, separators=(",", ":"), sort_keys=True).encode("utf-8"),
        )

    def _sign_heartbeat(self, payload: dict) -> str:
        ordered = {
            "node_id": payload["node_id"],
            "agent_key_fingerprint": payload["agent_key_fingerprint"],
            "heartbeat_timestamp": payload["heartbeat_timestamp"],
            "sequence_number": payload["sequence_number"],
            "agent_version": payload["agent_version"],
            "enrollment_generation": payload["enrollment_generation"],
            "local_observation_flags": payload["local_observation_flags"],
        }
        return sign_hex(
            self.private_key,
            json.dumps(ordered, separators=(",", ":")).encode("utf-8"),
        )


def _fetch_leaf_certificate_not_after(parsed: urllib.parse.ParseResult) -> datetime | None:
    host = parsed.hostname
    if not host:
        return None
    port = parsed.port or 443
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    try:
        with socket.create_connection((host, port), timeout=5) as sock:
            with context.wrap_socket(sock, server_hostname=host) as tls_sock:
                cert = tls_sock.getpeercert()
    except OSError:
        return None
    not_after = cert.get("notAfter")
    if not not_after:
        return None
    return datetime.strptime(not_after, "%b %d %H:%M:%S %Y %Z").replace(tzinfo=UTC)


def _time_now_rfc3339() -> str:
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
