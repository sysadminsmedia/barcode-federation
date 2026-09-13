package crypto

import (
	"crypto/ed25519"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	record := map[string]interface{}{
		"fbs":  "1.0",
		"type": "BarcodeRecord",
		"id":   "https://example.com/records/123",
		"data": "test",
	}

	sig, err := SignRecord(record, priv)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := VerifySignature(record, sig, pub)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Error("signature should verify")
	}

	// Tamper and verify fails
	record["data"] = "tampered"
	valid, err = VerifySignature(record, sig, pub)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Error("tampered record should not verify")
	}
}

func TestCanonicalJSON_SignatureOmitted(t *testing.T) {
	record := map[string]interface{}{
		"b":         "second",
		"a":         "first",
		"signature": "should-be-omitted",
	}

	canonical, err := CanonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"a":"first","b":"second"}`
	if string(canonical) != expected {
		t.Errorf("canonical JSON:\n  got:    %s\n  expect: %s", string(canonical), expected)
	}
}

func TestCanonicalJSON_NestedSorting(t *testing.T) {
	record := map[string]interface{}{
		"z": map[string]interface{}{
			"b": 2,
			"a": 1,
		},
		"a": "first",
	}

	canonical, err := CanonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"a":"first","z":{"a":1,"b":2}}`
	if string(canonical) != expected {
		t.Errorf("got: %s\nexpect: %s", string(canonical), expected)
	}
}

func TestKeyFingerprint(t *testing.T) {
	pub, _, _ := GenerateKeypair()
	fp := KeyFingerprint(pub)
	if len(fp) < 10 {
		t.Error("fingerprint too short")
	}
	if fp[:7] != "sha256:" {
		t.Error("fingerprint should start with sha256:")
	}
}

func TestEncodeDecodePublicKey(t *testing.T) {
	pub, _, _ := GenerateKeypair()

	pem, err := EncodePublicKeyPEM(pub)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodePublicKeyPEM(pem)
	if err != nil {
		t.Fatal(err)
	}

	if !ed25519.PublicKey(pub).Equal(decoded) {
		t.Error("decoded key should equal original")
	}
}
