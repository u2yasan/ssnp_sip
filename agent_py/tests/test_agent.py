from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from cryptography.hazmat.primitives.asymmetric import ed25519

from ssnp_agent.agent import Agent
from ssnp_agent.config import Config
from ssnp_agent.crypto import fingerprint, generate_and_write_key_pair, load_public_key


class AgentTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        private_key_path, public_key_path = generate_and_write_key_pair(self.temp_dir.name)
        self.cfg = Config(
            node_id="node-abc",
            portal_base_url="http://portal.example.test",
            agent_key_path=private_key_path,
            agent_public_key_path=public_key_path,
            monitored_endpoint="http://symbol.example.test",
            state_path=str(Path(self.temp_dir.name) / "state.json"),
            temp_dir=self.temp_dir.name,
            request_timeout_seconds=5,
            heartbeat_jitter_seconds_max=0,
            agent_version="1.0.0",
            enrollment_generation=1,
        )

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_enroll_payload_uses_portal_contract(self) -> None:
        agent = Agent.from_config(self.cfg)
        with mock.patch.object(agent.http, "post_json") as post_json:
            agent.enroll("challenge-001")
        payload = post_json.call_args.args[1]
        self.assertEqual(post_json.call_args.args[0], "/api/v1/agent/enroll")
        self.assertEqual(payload["node_id"], "node-abc")
        self.assertEqual(payload["enrollment_challenge_id"], "challenge-001")
        self.assertEqual(payload["agent_version"], "1.0.0")
        self.assertTrue(payload["signature"])

    def test_heartbeat_signature_signs_ordered_payload(self) -> None:
        agent = Agent.from_config(self.cfg)
        payload = {
            "node_id": "node-abc",
            "agent_key_fingerprint": agent.agent_fingerprint,
            "heartbeat_timestamp": "2026-04-08T00:00:00Z",
            "sequence_number": 1,
            "agent_version": "1.0.0",
            "enrollment_generation": 1,
            "local_observation_flags": ["portal_unreachable"],
        }
        signature = agent._sign_heartbeat(payload)
        public_key = load_public_key(self.cfg.agent_public_key_path)
        public_key.verify(
            bytes.fromhex(signature),
            json.dumps(payload, separators=(",", ":")).encode("utf-8"),
        )

    def test_fingerprint_matches_expected_scheme(self) -> None:
        public_key = load_public_key(self.cfg.agent_public_key_path)
        self.assertEqual(len(fingerprint(public_key)), 32)


if __name__ == "__main__":
    unittest.main()
