from __future__ import annotations

import urllib.parse
from dataclasses import dataclass
from datetime import timedelta

from .http_client import HTTPClient


@dataclass(slots=True)
class VotingKeyStatus:
    near_expiry: bool


class SymbolClient:
    def __init__(self, base_url: str, timeout_seconds: int) -> None:
        self.http = HTTPClient(base_url, timeout_seconds)

    def has_voting_key_expiry_risk(self, risk_window: timedelta) -> VotingKeyStatus:
        public_key = self._fetch_node_public_key()
        height = self._fetch_chain_height()
        grouping, block_target_seconds = self._fetch_epoch_properties()
        current_epoch = current_voting_epoch(height, grouping)
        end_epoch = self._fetch_earliest_active_voting_end_epoch(public_key, current_epoch)
        if end_epoch is None:
            return VotingKeyStatus(near_expiry=False)
        remaining_epochs = max(end_epoch - current_epoch, 0)
        epoch_duration = grouping * block_target_seconds
        if epoch_duration <= 0:
            raise ValueError("invalid epoch duration")
        return VotingKeyStatus(
            near_expiry=timedelta(seconds=remaining_epochs * epoch_duration) < risk_window
        )

    def _fetch_node_public_key(self) -> str:
        resp = self.http.get_json("/node/info")
        public_key = (resp.get("publicKey") or resp.get("node", {}).get("publicKey") or "").strip()
        if not public_key:
            raise ValueError("node info missing publicKey")
        return public_key

    def _fetch_chain_height(self) -> int:
        resp = self.http.get_json("/chain/info")
        return _parse_int_string(resp["height"])

    def _fetch_epoch_properties(self) -> tuple[int, int]:
        resp = self.http.get_json("/network/properties")
        props = resp.get("chain", {})
        if not props.get("votingSetGrouping"):
            props = resp.get("network", {}).get("chain", {})
        grouping = _parse_int_string(props["votingSetGrouping"])
        block_target_seconds = _parse_duration_seconds(props["blockGenerationTargetTime"])
        if grouping <= 0 or block_target_seconds <= 0:
            raise ValueError("invalid network properties")
        return grouping, block_target_seconds

    def _fetch_earliest_active_voting_end_epoch(
        self, public_key: str, current_epoch: int
    ) -> int | None:
        resp = self.http.get_json(f"/accounts/{urllib.parse.quote(public_key)}")
        public_keys = (
            resp.get("account", {})
            .get("supplementalPublicKeys", {})
            .get("voting", {})
            .get("publicKeys", [])
        )
        earliest: int | None = None
        for item in public_keys:
            start_epoch = _parse_int_like(item["startEpoch"])
            end_epoch = _parse_int_like(item["endEpoch"])
            if current_epoch < start_epoch or current_epoch > end_epoch:
                continue
            if earliest is None or end_epoch < earliest:
                earliest = end_epoch
        return earliest


def current_voting_epoch(height: int, grouping: int) -> int:
    if height <= 0 or grouping <= 0:
        return 0
    return ((height - 1) // grouping) + 1


def _parse_int_string(value: str) -> int:
    value = value.strip()
    if not value:
        raise ValueError("empty value")
    return int(value)


def _parse_int_like(value: object) -> int:
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    if isinstance(value, str):
        return _parse_int_string(value)
    raise ValueError(f"unsupported integer type {type(value)!r}")


def _parse_duration_seconds(value: str) -> int:
    value = value.strip()
    if not value.endswith("s"):
        raise ValueError("invalid duration")
    return int(value[:-1])
