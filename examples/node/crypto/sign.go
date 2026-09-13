package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
)

// CanonicalJSON produces the canonical JSON form for signing:
// keys sorted lexicographically at every level, no insignificant whitespace,
// with the "signature" field omitted.
func CanonicalJSON(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal for canonicalization: %w", err)
	}

	cleaned := removeSignatureField(raw)
	return marshalSorted(cleaned)
}

func removeSignatureField(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v2 := range val {
			if k == "signature" {
				continue
			}
			result[k] = removeSignatureField(v2)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v2 := range val {
			result[i] = removeSignatureField(v2)
		}
		return result
	default:
		return v
	}
}

func marshalSorted(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		buf := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			keyBytes, _ := json.Marshal(k)
			buf = append(buf, keyBytes...)
			buf = append(buf, ':')
			valBytes, err := marshalSorted(val[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, valBytes...)
		}
		buf = append(buf, '}')
		return buf, nil

	case []interface{}:
		buf := []byte{'['}
		for i, item := range val {
			if i > 0 {
				buf = append(buf, ',')
			}
			itemBytes, err := marshalSorted(item)
			if err != nil {
				return nil, err
			}
			buf = append(buf, itemBytes...)
		}
		buf = append(buf, ']')
		return buf, nil

	default:
		return json.Marshal(v)
	}
}

// SignRecord signs a record (or any signable object) and returns the base64url signature.
func SignRecord(v interface{}, key ed25519.PrivateKey) (string, error) {
	canonical, err := CanonicalJSON(v)
	if err != nil {
		return "", fmt.Errorf("canonical json: %w", err)
	}
	sig := ed25519.Sign(key, canonical)
	return base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifySignature verifies a base64url signature against the canonical form of v.
func VerifySignature(v interface{}, signature string, pub ed25519.PublicKey) (bool, error) {
	canonical, err := CanonicalJSON(v)
	if err != nil {
		return false, fmt.Errorf("canonical json: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}
	return ed25519.Verify(pub, canonical, sig), nil
}
