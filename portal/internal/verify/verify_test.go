package verify

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestVerifyHexPublicKeySignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	data := []byte(`{"node_id":"node-abc"}`)
	sig := ed25519.Sign(priv, data)

	if err := VerifyHexPublicKeySignature(hex.EncodeToString(pub), hex.EncodeToString(sig), data); err != nil {
		t.Fatalf("VerifyHexPublicKeySignature() error = %v", err)
	}
	if err := VerifyHexPublicKeySignature(hex.EncodeToString(pub), hex.EncodeToString(sig), []byte(`{"node_id":"other"}`)); err == nil {
		t.Fatal("VerifyHexPublicKeySignature() error = nil, want invalid signature")
	}
}

func TestCanonicalJSONWithoutSignature(t *testing.T) {
	payload := map[string]any{
		"b":         "two",
		"a":         "one",
		"signature": "ignored",
	}

	data, err := CanonicalJSONWithoutSignature(payload)
	if err != nil {
		t.Fatalf("CanonicalJSONWithoutSignature() error = %v", err)
	}
	if string(data) != `{"a":"one","b":"two"}` {
		t.Fatalf("canonical json = %s", string(data))
	}
}
