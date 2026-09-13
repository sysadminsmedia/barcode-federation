// Field-level encryption for FBS metadata fields per RFC FBS0001 Section 4.2.6.
//
// Uses X25519 key agreement (RFC 7748) + XSalsa20-Poly1305 authenticated encryption
// (NaCl crypto_box construction). Each field is encrypted independently per recipient
// using a fresh ephemeral X25519 keypair.
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

const (
	FieldEncryptionAlgorithm = "X25519-XSalsa20-Poly1305"
	nonceSize                = 24
)

// EncryptedFieldEnvelope is the JSON structure that replaces a plaintext
// metadata field value when encrypted. Defined in Section 4.2.6.2.
type EncryptedFieldEnvelope struct {
	Encrypted  bool                    `json:"encrypted"`
	Algorithm  string                  `json:"algorithm"`
	Recipients []EncryptedFieldRecipient `json:"recipients"`
}

// EncryptedFieldRecipient holds the ciphertext for one recipient.
type EncryptedFieldRecipient struct {
	KeyID              string `json:"keyId"`
	EphemeralPublicKey string `json:"ephemeralPublicKey"`
	Nonce              string `json:"nonce"`
	Ciphertext         string `json:"ciphertext"`
}

// X25519KeyPair holds an X25519 key pair for encryption/decryption.
type X25519KeyPair struct {
	PublicKey  [32]byte
	PrivateKey [32]byte
}

// GenerateX25519KeyPair generates a new X25519 key pair.
func GenerateX25519KeyPair() (*X25519KeyPair, error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate X25519 keypair: %w", err)
	}
	return &X25519KeyPair{PublicKey: *pub, PrivateKey: *priv}, nil
}

// PublicKeyBase64 returns the base64url-encoded public key.
func (kp *X25519KeyPair) PublicKeyBase64() string {
	return base64.RawURLEncoding.EncodeToString(kp.PublicKey[:])
}

// Recipient describes a target for field encryption.
type Recipient struct {
	KeyID     string   // e.g. "fbs://node/actor#encryption-key"
	PublicKey [32]byte // X25519 public key
}

// EncryptField encrypts a JSON field value for one or more recipients.
// Returns the EncryptedFieldEnvelope ready to be placed in the metadata.
func EncryptField(plaintext json.RawMessage, recipients []Recipient) (*EncryptedFieldEnvelope, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient required")
	}

	envelope := &EncryptedFieldEnvelope{
		Encrypted:  true,
		Algorithm:  FieldEncryptionAlgorithm,
		Recipients: make([]EncryptedFieldRecipient, 0, len(recipients)),
	}

	for _, r := range recipients {
		// Generate fresh ephemeral X25519 keypair per recipient (Section 4.2.6.3 step 2b)
		ephPub, ephPriv, err := box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}

		// Generate random nonce (Section 4.2.6.3 step 2d)
		var nonce [nonceSize]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, fmt.Errorf("generate nonce: %w", err)
		}

		// Encrypt using NaCl box (X25519 + XSalsa20-Poly1305)
		recipientPub := r.PublicKey
		ciphertext := box.Seal(nil, []byte(plaintext), &nonce, &recipientPub, ephPriv)

		envelope.Recipients = append(envelope.Recipients, EncryptedFieldRecipient{
			KeyID:              r.KeyID,
			EphemeralPublicKey: base64.RawURLEncoding.EncodeToString(ephPub[:]),
			Nonce:              base64.RawURLEncoding.EncodeToString(nonce[:]),
			Ciphertext:         base64.RawURLEncoding.EncodeToString(ciphertext),
		})

		// Ephemeral private key is not stored — it falls out of scope here
		_ = ephPriv
	}

	return envelope, nil
}

// DecryptField decrypts a field from an EncryptedFieldEnvelope using the
// recipient's X25519 private key. Returns the original JSON field value.
func DecryptField(envelope *EncryptedFieldEnvelope, keyID string, privateKey [32]byte) (json.RawMessage, error) {
	if !envelope.Encrypted {
		return nil, fmt.Errorf("field is not encrypted")
	}
	if envelope.Algorithm != FieldEncryptionAlgorithm {
		return nil, fmt.Errorf("unsupported algorithm: %s", envelope.Algorithm)
	}

	for _, r := range envelope.Recipients {
		if r.KeyID != keyID {
			continue
		}

		ephPubBytes, err := base64.RawURLEncoding.DecodeString(r.EphemeralPublicKey)
		if err != nil {
			return nil, fmt.Errorf("decode ephemeral public key: %w", err)
		}
		if len(ephPubBytes) != 32 {
			return nil, fmt.Errorf("ephemeral public key must be 32 bytes")
		}

		nonceBytes, err := base64.RawURLEncoding.DecodeString(r.Nonce)
		if err != nil {
			return nil, fmt.Errorf("decode nonce: %w", err)
		}
		if len(nonceBytes) != nonceSize {
			return nil, fmt.Errorf("nonce must be %d bytes", nonceSize)
		}

		ciphertextBytes, err := base64.RawURLEncoding.DecodeString(r.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decode ciphertext: %w", err)
		}

		var ephPub [32]byte
		copy(ephPub[:], ephPubBytes)
		var nonce [nonceSize]byte
		copy(nonce[:], nonceBytes)

		plaintext, ok := box.Open(nil, ciphertextBytes, &nonce, &ephPub, &privateKey)
		if !ok {
			return nil, fmt.Errorf("decryption failed: authentication error")
		}

		return json.RawMessage(plaintext), nil
	}

	return nil, fmt.Errorf("no recipient entry found for keyId: %s", keyID)
}

// IsEncryptedField checks if a JSON value is an EncryptedFieldEnvelope.
func IsEncryptedField(data json.RawMessage) bool {
	var probe struct {
		Encrypted bool `json:"encrypted"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Encrypted
}

// ParseEncryptedField parses a JSON value as an EncryptedFieldEnvelope.
func ParseEncryptedField(data json.RawMessage) (*EncryptedFieldEnvelope, error) {
	var envelope EncryptedFieldEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse encrypted field: %w", err)
	}
	if !envelope.Encrypted {
		return nil, fmt.Errorf("not an encrypted field")
	}
	return &envelope, nil
}

// DeriveX25519PublicKey derives the X25519 public key from a private key.
func DeriveX25519PublicKey(privateKey [32]byte) [32]byte {
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &privateKey)
	return pub
}
