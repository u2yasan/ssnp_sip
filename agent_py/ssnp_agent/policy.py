from __future__ import annotations

import math
import urllib.parse
from dataclasses import dataclass

from .http_client import HTTPClient


@dataclass(slots=True)
class AcceptanceFloor:
    type: str
    minimum: float


@dataclass(slots=True)
class CPUWorkloadMix:
    hashing: float
    integer: float
    matrix: float


@dataclass(slots=True)
class CPUProfile:
    id: str
    duration_seconds: int
    warmup_seconds: int
    measured_seconds: int
    cooldown_seconds: int
    worker_cap: int
    workload_mix: CPUWorkloadMix
    acceptance_floor: AcceptanceFloor


@dataclass(slots=True)
class DiskProfile:
    id: str
    duration_seconds: int
    warmup_seconds: int
    measured_seconds: int
    cooldown_seconds: int
    block_size_bytes: int
    queue_depth: int
    concurrency: int
    read_ratio: float
    write_ratio: float
    acceptance_floor: AcceptanceFloor


@dataclass(slots=True)
class HardwareThresholds:
    cpu_cores_min: int
    ram_gb_min: int
    storage_gb_min: int
    ssd_required: bool


@dataclass(slots=True)
class ProbeThresholds:
    finalized_lag_max_blocks: int
    chain_lag_max_blocks: int


@dataclass(slots=True)
class ReferenceEnvironment:
    id: str
    os_image_id: str
    agent_version: str
    cpu_profile_id: str
    disk_profile_id: str
    baseline_source_date: str


@dataclass(slots=True)
class PolicyResponse:
    policy_version: str
    heartbeat_interval_seconds: int
    cpu_profile: CPUProfile
    disk_profile: DiskProfile
    hardware_thresholds: HardwareThresholds
    probe_thresholds: ProbeThresholds
    reference_environment: ReferenceEnvironment


class PolicyClient:
    def __init__(self, base_url: str, timeout_seconds: int) -> None:
        self.http = HTTPClient(base_url, timeout_seconds)

    def fetch(self, node_id: str, fingerprint: str) -> PolicyResponse:
        query = urllib.parse.urlencode(
            {"node_id": node_id, "agent_key_fingerprint": fingerprint}
        )
        payload = self.http.get_json(f"/api/v1/agent/policy?{query}")
        policy = PolicyResponse(
            policy_version=payload["policy_version"],
            heartbeat_interval_seconds=payload["heartbeat_interval_seconds"],
            cpu_profile=CPUProfile(
                **{
                    **payload["cpu_profile"],
                    "workload_mix": CPUWorkloadMix(**payload["cpu_profile"]["workload_mix"]),
                    "acceptance_floor": AcceptanceFloor(
                        **payload["cpu_profile"]["acceptance_floor"]
                    ),
                }
            ),
            disk_profile=DiskProfile(
                **{
                    **payload["disk_profile"],
                    "acceptance_floor": AcceptanceFloor(
                        **payload["disk_profile"]["acceptance_floor"]
                    ),
                }
            ),
            hardware_thresholds=HardwareThresholds(**payload["hardware_thresholds"]),
            probe_thresholds=ProbeThresholds(**payload["probe_thresholds"]),
            reference_environment=ReferenceEnvironment(**payload["reference_environment"]),
        )
        validate(policy)
        return policy


def validate(doc: PolicyResponse) -> None:
    if not doc.policy_version:
        raise ValueError("missing policy_version")
    if doc.heartbeat_interval_seconds <= 0:
        raise ValueError("heartbeat_interval_seconds must be positive")
    if not doc.cpu_profile.id or not doc.disk_profile.id:
        raise ValueError("missing profile id")
    if (
        doc.cpu_profile.duration_seconds <= 0
        or doc.cpu_profile.warmup_seconds <= 0
        or doc.cpu_profile.measured_seconds <= 0
        or doc.cpu_profile.cooldown_seconds <= 0
    ):
        raise ValueError("cpu_profile durations must be positive")
    if doc.cpu_profile.worker_cap <= 0:
        raise ValueError("cpu_profile.worker_cap must be positive")
    if not _approximately_equal(
        doc.cpu_profile.workload_mix.hashing
        + doc.cpu_profile.workload_mix.integer
        + doc.cpu_profile.workload_mix.matrix,
        1.0,
    ):
        raise ValueError("cpu_profile.workload_mix must sum to 1.0")
    if (
        doc.disk_profile.duration_seconds <= 0
        or doc.disk_profile.warmup_seconds <= 0
        or doc.disk_profile.measured_seconds <= 0
        or doc.disk_profile.cooldown_seconds <= 0
    ):
        raise ValueError("disk_profile durations must be positive")
    if (
        doc.disk_profile.block_size_bytes <= 0
        or doc.disk_profile.queue_depth <= 0
        or doc.disk_profile.concurrency <= 0
    ):
        raise ValueError("disk_profile performance parameters must be positive")
    if not _approximately_equal(
        doc.disk_profile.read_ratio + doc.disk_profile.write_ratio, 1.0
    ):
        raise ValueError("disk_profile read/write ratios must sum to 1.0")
    if (
        doc.hardware_thresholds.cpu_cores_min <= 0
        or doc.hardware_thresholds.ram_gb_min <= 0
        or doc.hardware_thresholds.storage_gb_min <= 0
    ):
        raise ValueError("hardware thresholds must be positive")
    if doc.probe_thresholds.finalized_lag_max_blocks <= 0:
        raise ValueError("probe_thresholds.finalized_lag_max_blocks must be positive")
    if doc.probe_thresholds.chain_lag_max_blocks <= 0:
        raise ValueError("probe_thresholds.chain_lag_max_blocks must be positive")
    ref = doc.reference_environment
    if (
        not ref.id
        or not ref.os_image_id
        or not ref.agent_version
        or not ref.cpu_profile_id
        or not ref.disk_profile_id
        or not ref.baseline_source_date
    ):
        raise ValueError("missing reference_environment fields")
    if ref.cpu_profile_id != doc.cpu_profile.id or ref.disk_profile_id != doc.disk_profile.id:
        raise ValueError("reference_environment profile ids must match active profiles")


def _approximately_equal(left: float, right: float) -> bool:
    return math.fabs(left - right) <= 1e-9
