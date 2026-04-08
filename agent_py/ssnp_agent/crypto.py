from __future__ import annotations

import hashlib
from pathlib import Path

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ed25519


def load_private_key(path: str) -> ed25519.Ed25519PrivateKey:
    key = serialization.load_pem_private_key(Path(path).read_bytes(), password=None)
    if not isinstance(key, ed25519.Ed25519PrivateKey):
        raise ValueError("private key: not ed25519")
    return key


def load_public_key(path: str) -> ed25519.Ed25519PublicKey:
    key = serialization.load_pem_public_key(Path(path).read_bytes())
    if not isinstance(key, ed25519.Ed25519PublicKey):
        raise ValueError("public key: not ed25519")
    return key


def public_key_bytes(key: ed25519.Ed25519PublicKey) -> bytes:
    return key.public_bytes(
        encoding=serialization.Encoding.Raw,
        format=serialization.PublicFormat.Raw,
    )


def fingerprint(key: ed25519.Ed25519PublicKey) -> str:
    return hashlib.sha256(public_key_bytes(key)).digest()[:16].hex()


def sign_hex(private_key: ed25519.Ed25519PrivateKey, data: bytes) -> str:
    return private_key.sign(data).hex()


def generate_and_write_key_pair(directory: str) -> tuple[str, str]:
    if not directory:
        raise ValueError("key output dir is required")
    out_dir = Path(directory)
    out_dir.mkdir(parents=True, exist_ok=True)

    private_key = ed25519.Ed25519PrivateKey.generate()
    public_key = private_key.public_key()

    private_key_path = out_dir / "agent_private_key.pem"
    public_key_path = out_dir / "agent_public_key.pem"

    private_key_path.write_bytes(
        private_key.private_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PrivateFormat.PKCS8,
            encryption_algorithm=serialization.NoEncryption(),
        )
    )
    public_key_path.write_bytes(
        public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
    )
    private_key_path.chmod(0o600)
    public_key_path.chmod(0o600)
    return str(private_key_path), str(public_key_path)
