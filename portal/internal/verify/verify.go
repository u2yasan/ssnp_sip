package verify

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

func FingerprintFromHexPublicKey(hexKey string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", err
	}
	if len(key) != ed25519.PublicKeySize {
		return "", errors.New("invalid ed25519 public key length")
	}
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:16]), nil
}

func VerifyHexPublicKeySignature(hexKey, hexSig string, data []byte) error {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return err
	}
	if len(key) != ed25519.PublicKeySize {
		return errors.New("invalid ed25519 public key length")
	}
	sig, err := hex.DecodeString(hexSig)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(key), data, sig) {
		return errors.New("invalid signature")
	}
	return nil
}

func CanonicalJSONWithoutSignature(m map[string]any) ([]byte, error) {
	copyMap := map[string]any{}
	for k, v := range m {
		if k == "signature" {
			continue
		}
		copyMap[k] = v
	}
	return marshalCanonical(copyMap)
}

func marshalCanonical(v any) ([]byte, error) {
	normalized := normalize(v)
	return json.Marshal(normalized)
}

func normalize(v any) any {
	switch value := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make([]kv, 0, len(keys))
		for _, key := range keys {
			out = append(out, kv{Key: key, Value: normalize(value[key])})
		}
		return orderedMap(out)
	case []any:
		out := make([]any, 0, len(value))
		for _, item := range value {
			out = append(out, normalize(item))
		}
		return out
	default:
		return v
	}
}

type kv struct {
	Key   string
	Value any
}

type orderedMap []kv

func (m orderedMap) MarshalJSON() ([]byte, error) {
	out := []byte{'{'}
	for i, pair := range m {
		if i > 0 {
			out = append(out, ',')
		}
		keyJSON, err := json.Marshal(pair.Key)
		if err != nil {
			return nil, err
		}
		valueJSON, err := json.Marshal(pair.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, keyJSON...)
		out = append(out, ':')
		out = append(out, valueJSON...)
	}
	out = append(out, '}')
	return out, nil
}
