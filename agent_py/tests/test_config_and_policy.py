from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

from ssnp_agent.config import Config
from ssnp_agent.policy import PolicyClient


class ConfigAndPolicyTests(unittest.TestCase):
    def test_config_validation_rejects_missing_node_id(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "config.yaml"
            path.write_text(
                """
portal_base_url: "http://127.0.0.1:8080"
agent_key_path: "/tmp/private.pem"
agent_public_key_path: "/tmp/public.pem"
monitored_endpoint: "http://127.0.0.1:3000"
state_path: "/tmp/state.json"
temp_dir: "/tmp"
request_timeout_seconds: 5
heartbeat_jitter_seconds_max: 0
agent_version: "1.0.0"
enrollment_generation: 1
""".strip(),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "node_id is required"):
                Config.load(str(path))

    def test_policy_fetch_validates_shape(self) -> None:
        client = PolicyClient("http://portal.example.test", 5)
        with mock.patch.object(
            client.http,
            "get_json",
            return_value={
                "policy_version": "2026-04",
                "heartbeat_interval_seconds": 10,
                "cpu_profile": {
                    "id": "cpu-check-v1",
                    "duration_seconds": 180,
                    "warmup_seconds": 30,
                    "measured_seconds": 120,
                    "cooldown_seconds": 30,
                    "worker_cap": 8,
                    "workload_mix": {"hashing": 0.5, "integer": 0.3, "matrix": 0.2},
                    "acceptance_floor": {"type": "normalized_score", "minimum": 1.0},
                },
                "disk_profile": {
                    "id": "disk-check-v1",
                    "duration_seconds": 180,
                    "warmup_seconds": 30,
                    "measured_seconds": 120,
                    "cooldown_seconds": 30,
                    "block_size_bytes": 4096,
                    "queue_depth": 32,
                    "concurrency": 4,
                    "read_ratio": 0.7,
                    "write_ratio": 0.3,
                    "acceptance_floor": {"type": "measured_iops", "minimum": 1500},
                },
                "hardware_thresholds": {
                    "cpu_cores_min": 8,
                    "ram_gb_min": 32,
                    "storage_gb_min": 750,
                    "ssd_required": True,
                },
                "probe_thresholds": {
                    "finalized_lag_max_blocks": 2,
                    "chain_lag_max_blocks": 5,
                },
                "reference_environment": {
                    "id": "ref-env-2026-04",
                    "os_image_id": "ubuntu-24.04-lts",
                    "agent_version": "1.0.0",
                    "cpu_profile_id": "cpu-check-v1",
                    "disk_profile_id": "disk-check-v1",
                    "baseline_source_date": "2026-04-06",
                },
            },
        ):
            policy = client.fetch("node-abc", "fp")
        self.assertEqual(policy.policy_version, "2026-04")
        self.assertEqual(policy.cpu_profile.id, "cpu-check-v1")


if __name__ == "__main__":
    unittest.main()
