package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
	"strings"
)

var fniRegex = regexp.MustCompile(`^fni1[a-z2-7]{32}$`)

// DeriveFNI computes the FBS Namespace Identifier from a raw Ed25519 public key.
// Algorithm per RFC FBS0001 Section 4.2.5.2:
//   1. SHA-256 hash of the 32-byte raw public key
//   2. Truncate to first 20 bytes
//   3. Base32 encode (RFC 4648 §6), lowercase, no padding
//   4. Prepend "fni1"
func DeriveFNI(pub ed25519.PublicKey) (string, error) {
	if len(pub) != ed25519.PublicKeySize {
		return "", fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}

	hashBytes := sha256.Sum256(pub)
	truncated := hashBytes[:20]
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(truncated)
	encoded = strings.ToLower(encoded)

	if len(encoded) != 32 {
		return "", fmt.Errorf("base32 encoding produced %d chars, expected 32", len(encoded))
	}

	return "fni1" + encoded, nil
}

// ValidateFNISyntax checks if a string is a syntactically valid FNI.
func ValidateFNISyntax(fni string) bool {
	return fniRegex.MatchString(fni)
}

// VerifyFNIDerivation confirms an FNI was correctly derived from a public key.
func VerifyFNIDerivation(pub ed25519.PublicKey, claimedFNI string) (bool, error) {
	derived, err := DeriveFNI(pub)
	if err != nil {
		return false, err
	}
	return TimingSafeEqual(derived, claimedFNI), nil
}

// TimingSafeEqual performs a constant-time comparison of two strings.
func TimingSafeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
