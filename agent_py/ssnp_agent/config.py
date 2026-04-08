from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class Config:
    node_id: str = ""
    portal_base_url: str = ""
    agent_key_path: str = ""
    agent_public_key_path: str = ""
    monitored_endpoint: str = ""
    state_path: str = ""
    temp_dir: str = ""
    request_timeout_seconds: int = 0
    heartbeat_jitter_seconds_max: int = 0
    agent_version: str = ""
    enrollment_generation: int = 0

    @classmethod
    def load(cls, path: str) -> "Config":
        payload = _parse_simple_yaml(Path(path).read_text(encoding="utf-8"))
        cfg = cls(**payload)
        cfg.validate()
        return cfg

    def validate(self) -> None:
        if not self.node_id:
            raise ValueError("config: node_id is required")
        if not self.portal_base_url:
            raise ValueError("config: portal_base_url is required")
        if not self.agent_key_path:
            raise ValueError("config: agent_key_path is required")
        if not self.agent_public_key_path:
            raise ValueError("config: agent_public_key_path is required")
        if not self.monitored_endpoint:
            raise ValueError("config: monitored_endpoint is required")
        if not self.state_path:
            raise ValueError("config: state_path is required")
        if not self.temp_dir:
            raise ValueError("config: temp_dir is required")
        if self.request_timeout_seconds <= 0:
            raise ValueError("config: request_timeout_seconds must be > 0")
        if self.heartbeat_jitter_seconds_max < 0:
            raise ValueError("config: heartbeat_jitter_seconds_max must be >= 0")
        if not self.agent_version:
            raise ValueError("config: agent_version is required")
        if self.enrollment_generation <= 0:
            raise ValueError("config: enrollment_generation must be > 0")


def _parse_simple_yaml(content: str) -> dict[str, object]:
    out: dict[str, object] = {}
    for raw_line in content.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" not in line:
            raise ValueError(f"config: invalid line: {raw_line}")
        key, raw_value = line.split(":", 1)
        out[key.strip()] = _parse_scalar(raw_value.strip())
    return out


def _parse_scalar(value: str) -> object:
    if value.startswith('"') and value.endswith('"'):
        return value[1:-1]
    if value.startswith("'") and value.endswith("'"):
        return value[1:-1]
    lower = value.lower()
    if lower == "true":
        return True
    if lower == "false":
        return False
    if value.isdigit() or (value.startswith("-") and value[1:].isdigit()):
        return int(value)
    return value
