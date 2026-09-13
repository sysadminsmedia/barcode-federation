package crypto

import (
	"encoding/hex"
	"testing"
)

func TestDeriveFNI_NormativeTestVector(t *testing.T) {
	// RFC FBS0001 Section 4.2.5.2.5 normative test vector
	pubKeyHex := "4f6f787a4211deadbeefcafebabe000102030405060708090a0b0c0d0e0f1011"
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	fni, err := DeriveFNI(pubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	expected := "fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7"
	if fni != expected {
		t.Errorf("FNI mismatch:\n  got:    %s\n  expect: %s", fni, expected)
	}
}

func TestDeriveFNI_Length(t *testing.T) {
	pubKeyHex := "4f6f787a4211deadbeefcafebabe000102030405060708090a0b0c0d0e0f1011"
	pubKeyBytes, _ := hex.DecodeString(pubKeyHex)

	fni, _ := DeriveFNI(pubKeyBytes)
	if len(fni) != 36 {
		t.Errorf("FNI length should be 36, got %d", len(fni))
	}
}

func TestValidateFNISyntax(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7", true},
		{"fni1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"fni1AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", false},  // uppercase not allowed
		{"fni2aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // wrong version
		{"fni1aaa", false},                              // too short
		{"", false},
	}

	for _, tc := range tests {
		result := ValidateFNISyntax(tc.input)
		if result != tc.valid {
			t.Errorf("ValidateFNISyntax(%q) = %v, want %v", tc.input, result, tc.valid)
		}
	}
}

func TestTimingSafeEqual(t *testing.T) {
	if !TimingSafeEqual("abc", "abc") {
		t.Error("equal strings should match")
	}
	if TimingSafeEqual("abc", "abd") {
		t.Error("different strings should not match")
	}
	if TimingSafeEqual("ab", "abc") {
		t.Error("different lengths should not match")
	}
}
