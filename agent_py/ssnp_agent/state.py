from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path


@dataclass(slots=True)
class State:
    sequence_number: int = 0
    last_policy_version: str = ""
    agent_key_fingerprint: str = ""
    consecutive_portal_failures: int = 0
    active_warnings: dict[str, bool] = field(default_factory=dict)
    pending_warnings: dict[str, bool] = field(default_factory=dict)

    @classmethod
    def load(cls, path: str) -> "State":
        state_path = Path(path)
        if not state_path.exists():
            return cls()
        payload = json.loads(state_path.read_text(encoding="utf-8"))
        return cls(**payload)

    def save(self, path: str) -> None:
        state_path = Path(path)
        state_path.parent.mkdir(parents=True, exist_ok=True)
        state_path.write_text(
            json.dumps(
                {
                    "sequence_number": self.sequence_number,
                    "last_policy_version": self.last_policy_version,
                    "agent_key_fingerprint": self.agent_key_fingerprint,
                    "consecutive_portal_failures": self.consecutive_portal_failures,
                    "active_warnings": self.active_warnings,
                    "pending_warnings": self.pending_warnings,
                },
                indent=2,
            ),
            encoding="utf-8",
        )
        state_path.chmod(0o600)
