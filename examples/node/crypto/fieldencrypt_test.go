package crypto

import (
	"encoding/json"
	"testing"
)

func TestFieldEncryptDecrypt_SingleRecipient(t *testing.T) {
	// Generate recipient keypair
	recipient, err := GenerateX25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}

	plaintext := json.RawMessage(`"123 Main St, Springfield, IL 62704"`)
	keyID := "fbs://usps-node.example.com/actors/carrier#encryption-key"

	// Encrypt
	envelope, err := EncryptField(plaintext, []Recipient{
		{KeyID: keyID, PublicKey: recipient.PublicKey},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !envelope.Encrypted {
		t.Error("encrypted should be true")
	}
	if envelope.Algorithm != FieldEncryptionAlgorithm {
		t.Errorf("unexpected algorithm: %s", envelope.Algorithm)
	}
	if len(envelope.Recipients) != 1 {
		t.Fatalf("expected 1 recipient, got %d", len(envelope.Recipients))
	}

	// Decrypt
	decrypted, err := DecryptField(envelope, keyID, recipient.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted mismatch:\n  got:    %s\n  expect: %s", decrypted, plaintext)
	}
}

func TestFieldEncryptDecrypt_MultipleRecipients(t *testing.T) {
	// Simulate FedEx encrypting a delivery address for both USPS and themselves
	fedex, _ := GenerateX25519KeyPair()
	usps, _ := GenerateX25519KeyPair()

	address := json.RawMessage(`{"street":"456 Oak Ave","city":"Portland","state":"OR","zip":"97201"}`)

	envelope, err := EncryptField(address, []Recipient{
		{KeyID: "fbs://fedex.example/actors/fedex#enc-key", PublicKey: fedex.PublicKey},
		{KeyID: "fbs://usps.example/actors/usps#enc-key", PublicKey: usps.PublicKey},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(envelope.Recipients) != 2 {
		t.Fatalf("expected 2 recipients, got %d", len(envelope.Recipients))
	}

	// FedEx can decrypt
	decFedex, err := DecryptField(envelope, "fbs://fedex.example/actors/fedex#enc-key", fedex.PrivateKey)
	if err != nil {
		t.Fatalf("FedEx decrypt: %v", err)
	}
	if string(decFedex) != string(address) {
		t.Error("FedEx decrypted wrong value")
	}

	// USPS can decrypt
	decUSPS, err := DecryptField(envelope, "fbs://usps.example/actors/usps#enc-key", usps.PrivateKey)
	if err != nil {
		t.Fatalf("USPS decrypt: %v", err)
	}
	if string(decUSPS) != string(address) {
		t.Error("USPS decrypted wrong value")
	}

	t.Log("Both carriers successfully decrypted the delivery address")
}

func TestFieldEncryptDecrypt_WrongKeyFails(t *testing.T) {
	recipient, _ := GenerateX25519KeyPair()
	attacker, _ := GenerateX25519KeyPair()

	plaintext := json.RawMessage(`"sensitive data"`)
	keyID := "fbs://node/actor#key"

	envelope, _ := EncryptField(plaintext, []Recipient{
		{KeyID: keyID, PublicKey: recipient.PublicKey},
	})

	// Attacker tries to decrypt with wrong key
	_, err := DecryptField(envelope, keyID, attacker.PrivateKey)
	if err == nil {
		t.Error("decryption with wrong key should fail")
	}
}

func TestFieldEncryptDecrypt_UnknownKeyIDFails(t *testing.T) {
	recipient, _ := GenerateX25519KeyPair()

	envelope, _ := EncryptField(json.RawMessage(`"data"`), []Recipient{
		{KeyID: "fbs://node/actor#key", PublicKey: recipient.PublicKey},
	})

	_, err := DecryptField(envelope, "fbs://other/actor#key", recipient.PrivateKey)
	if err == nil {
		t.Error("decryption with unknown keyId should fail")
	}
}

func TestIsEncryptedField(t *testing.T) {
	encrypted := json.RawMessage(`{"encrypted":true,"algorithm":"X25519-XSalsa20-Poly1305","recipients":[]}`)
	plain := json.RawMessage(`"just a string"`)
	object := json.RawMessage(`{"name":"test","encrypted":false}`)

	if !IsEncryptedField(encrypted) {
		t.Error("should detect encrypted field")
	}
	if IsEncryptedField(plain) {
		t.Error("plain string should not be detected as encrypted")
	}
	if IsEncryptedField(object) {
		t.Error("object with encrypted:false should not be detected")
	}
}

func TestFieldEncrypt_JSONRoundTrip(t *testing.T) {
	// Verify the envelope survives JSON marshal/unmarshal (federation transit)
	recipient, _ := GenerateX25519KeyPair()
	keyID := "fbs://node/actor#key"

	original := json.RawMessage(`{"street":"123 Main","city":"Test"}`)
	envelope, _ := EncryptField(original, []Recipient{
		{KeyID: keyID, PublicKey: recipient.PublicKey},
	})

	// Marshal to JSON (simulating federation)
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal (simulating receiving node)
	var restored EncryptedFieldEnvelope
	if err := json.Unmarshal(envelopeJSON, &restored); err != nil {
		t.Fatal(err)
	}

	// Decrypt from the restored envelope
	decrypted, err := DecryptField(&restored, keyID, recipient.PrivateKey)
	if err != nil {
		t.Fatalf("decrypt after roundtrip: %v", err)
	}

	if string(decrypted) != string(original) {
		t.Errorf("roundtrip mismatch:\n  got:    %s\n  expect: %s", decrypted, original)
	}
	t.Log("Encrypted field survives JSON federation roundtrip")
}

func TestFieldEncrypt_LogisticsHandoff(t *testing.T) {
	// Full logistics scenario: FedEx creates shipment, encrypts address for
	// USPS and themselves. Later re-encrypts for UPS when rerouted.

	fedex, _ := GenerateX25519KeyPair()
	usps, _ := GenerateX25519KeyPair()
	ups, _ := GenerateX25519KeyPair()

	address := json.RawMessage(`{"name":"Jane Doe","street":"789 Pine","city":"Seattle","state":"WA","zip":"98101"}`)

	// Step 1: FedEx encrypts for USPS and self
	envelope1, _ := EncryptField(address, []Recipient{
		{KeyID: "fedex#key", PublicKey: fedex.PublicKey},
		{KeyID: "usps#key", PublicKey: usps.PublicKey},
	})

	// Step 2: Shipment rerouted — FedEx decrypts and re-encrypts for UPS
	decrypted, err := DecryptField(envelope1, "fedex#key", fedex.PrivateKey)
	if err != nil {
		t.Fatalf("FedEx self-decrypt: %v", err)
	}

	envelope2, _ := EncryptField(decrypted, []Recipient{
		{KeyID: "fedex#key", PublicKey: fedex.PublicKey},
		{KeyID: "ups#key", PublicKey: ups.PublicKey},
	})

	// Step 3: UPS can decrypt from the new envelope
	decUPS, err := DecryptField(envelope2, "ups#key", ups.PrivateKey)
	if err != nil {
		t.Fatalf("UPS decrypt: %v", err)
	}
	if string(decUPS) != string(address) {
		t.Error("UPS got wrong address")
	}

	// USPS cannot decrypt the new envelope (they were removed)
	_, err = DecryptField(envelope2, "usps#key", usps.PrivateKey)
	if err == nil {
		t.Error("USPS should NOT be able to decrypt after reroute")
	}

	t.Log("Logistics reroute: FedEx re-encrypted for UPS, USPS access revoked")
}
