package transparencylog

import (
	"encoding/json"
	"testing"

	"github.com/fbscommunity/barcode-federation/examples/node/crypto"
)

func TestTransparencyLog_AddAndRetrieve(t *testing.T) {
	_, logKey, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	log, err := NewTransparencyLog("https://log.example.com/logs/primary", logKey)
	if err != nil {
		t.Fatal(err)
	}

	badge := map[string]interface{}{
		"fbs":     "1.0",
		"type":    "VerificationBadge",
		"badgeId": "https://example.com/badges/test-1",
		"level":   3,
		"subject": "fbs://example.com/actors/acme",
	}
	badgeJSON, _ := json.Marshal(badge)

	// Add badge
	sct, err := log.AddBadge(badgeJSON, "fbs://trust.example.com/coordinator")
	if err != nil {
		t.Fatal(err)
	}
	if sct.LogID != "https://log.example.com/logs/primary" {
		t.Error("SCT logId mismatch")
	}
	if sct.BadgeHash == "" {
		t.Error("SCT should have badgeHash")
	}
	if sct.Signature == "" {
		t.Error("SCT should be signed")
	}

	// Retrieve entries
	entries, err := log.GetEntries(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].BadgeHash != sct.BadgeHash {
		t.Error("entry badge hash should match SCT badge hash")
	}

	// Get inclusion proof
	proof, leafIndex, err := log.GetProofByHash(sct.BadgeHash)
	if err != nil {
		t.Fatal(err)
	}
	if leafIndex != 0 {
		t.Errorf("first entry should be at index 0, got %d", leafIndex)
	}

	// Verify inclusion
	rootHash := log.tree.RootHash()
	if !VerifyInclusion(badgeJSON, leafIndex, log.tree.Size(), proof, rootHash) {
		t.Fatal("inclusion proof should verify")
	}

	// Duplicate rejection
	_, err = log.AddBadge(badgeJSON, "fbs://other.example.com")
	if err == nil {
		t.Error("duplicate badge should be rejected")
	}

	// Check metadata
	meta := log.Metadata()
	if meta.TreeSize != 1 {
		t.Errorf("tree size should be 1, got %d", meta.TreeSize)
	}
	if meta.LogID != "https://log.example.com/logs/primary" {
		t.Error("metadata logId mismatch")
	}
}

func TestTransparencyLog_SCTVerification(t *testing.T) {
	pub, logKey, _ := crypto.GenerateKeypair()

	log, _ := NewTransparencyLog("https://log.test/primary", logKey)
	badgeJSON := []byte(`{"type":"VerificationBadge","level":1}`)

	sct, err := log.AddBadge(badgeJSON, "submitter")
	if err != nil {
		t.Fatal(err)
	}

	if !VerifySCT(sct, pub) {
		t.Fatal("SCT should verify with correct public key")
	}

	// Wrong key should fail
	otherPub, _, _ := crypto.GenerateKeypair()
	if VerifySCT(sct, otherPub) {
		t.Fatal("SCT should not verify with wrong key")
	}
}

func TestTransparencyLog_SignedTreeHead(t *testing.T) {
	_, logKey, _ := crypto.GenerateKeypair()
	log, _ := NewTransparencyLog("https://log.test/primary", logKey)

	log.AddBadge([]byte(`{"badge":1}`), "sub")
	log.AddBadge([]byte(`{"badge":2}`), "sub")
	log.AddBadge([]byte(`{"badge":3}`), "sub")

	sth := log.GetSignedTreeHead()
	if sth.TreeSize != 3 {
		t.Errorf("STH tree size should be 3, got %d", sth.TreeSize)
	}
	if sth.RootHash == "" {
		t.Error("STH should have root hash")
	}
	if sth.Signature == "" {
		t.Error("STH should be signed")
	}
}

func TestTransparencyLog_MultipleBadges(t *testing.T) {
	_, logKey, _ := crypto.GenerateKeypair()
	log, _ := NewTransparencyLog("https://log.test/primary", logKey)

	for i := 0; i < 20; i++ {
		badge, _ := json.Marshal(map[string]interface{}{
			"badgeId": i,
			"level":   i % 4,
		})
		_, err := log.AddBadge(badge, "coordinator")
		if err != nil {
			t.Fatalf("add badge %d: %v", i, err)
		}
	}

	if log.tree.Size() != 20 {
		t.Errorf("tree size should be 20, got %d", log.tree.Size())
	}

	// Verify inclusion for each badge
	for i := 0; i < 20; i++ {
		badge, _ := json.Marshal(map[string]interface{}{
			"badgeId": i,
			"level":   i % 4,
		})
		entries, _ := log.GetEntries(i, i+1)
		proof, leafIdx, err := log.GetProofByHash(entries[0].BadgeHash)
		if err != nil {
			t.Fatalf("proof for badge %d: %v", i, err)
		}
		if leafIdx != i {
			t.Errorf("badge %d at wrong index %d", i, leafIdx)
		}
		rootHash := log.tree.RootHash()
		if !VerifyInclusion(badge, leafIdx, log.tree.Size(), proof, rootHash) {
			t.Fatalf("inclusion failed for badge %d", i)
		}
	}
}
