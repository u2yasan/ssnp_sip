from __future__ import annotations

import json
import urllib.error
import urllib.request


class HTTPClient:
    def __init__(self, base_url: str, timeout_seconds: int) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout_seconds = timeout_seconds

    def post_json(self, path: str, payload: object) -> None:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        req = urllib.request.Request(
            self.base_url + path,
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout_seconds):
                return
        except urllib.error.HTTPError as err:
            raise RuntimeError(f"post {path} failed: {err.code} {err.reason}") from err

    def get_json(self, path: str) -> dict:
        req = urllib.request.Request(self.base_url + path, method="GET")
        try:
            with urllib.request.urlopen(req, timeout=self.timeout_seconds) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as err:
            raise RuntimeError(f"get {path} failed: {err.code} {err.reason}") from err
