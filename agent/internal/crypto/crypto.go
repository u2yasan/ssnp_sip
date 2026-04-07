package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("private key: invalid pem")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := keyAny.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key: not ed25519")
	}
	return key, nil
}

func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("public key: invalid pem")
	}
	keyAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := keyAny.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key: not ed25519")
	}
	return key, nil
}

func Fingerprint(key ed25519.PublicKey) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:16])
}

func Sign(privateKey ed25519.PrivateKey, data []byte) string {
	sig := ed25519.Sign(privateKey, data)
	return hex.EncodeToString(sig)
}

func GenerateAndWriteKeyPair(dir string) (string, string, error) {
	if dir == "" {
		return "", "", errors.New("key output dir is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", "", err
	}

	privateKeyPath := filepath.Join(dir, "agent_private_key.pem")
	publicKeyPath := filepath.Join(dir, "agent_public_key.pem")
	if err := writePEMFile(privateKeyPath, "PRIVATE KEY", privateDER); err != nil {
		return "", "", err
	}
	if err := writePEMFile(publicKeyPath, "PUBLIC KEY", publicDER); err != nil {
		return "", "", err
	}
	return privateKeyPath, publicKeyPath, nil
}

func writePEMFile(path, blockType string, der []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
