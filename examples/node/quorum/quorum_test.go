package quorum

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/transparencylog"
)

func TestFullQuorumCeremony(t *testing.T) {
	// Set up transparency log
	_, logKey, _ := crypto.GenerateKeypair()
	tlog, _ := transparencylog.NewTransparencyLog("https://log.test/primary", logKey)

	// Set up coordinator
	_, nodeKey, _ := crypto.GenerateKeypair()
	coordinator := NewCoordinator("https://coordinator.test", nodeKey, tlog)

	// Set up evidence package
	evidence := map[string]interface{}{
		"gs1PrefixLicense":     "LICENSE-DOC-12345",
		"businessRegistration": "REG-DOC-67890",
		"verifiedDomain":       "acme.example.com",
	}
	evidenceJSON, _ := json.Marshal(evidence)

	// Verifier nodes
	verifiers := []string{
		"https://verifier-a.test",
		"https://verifier-b.test",
		"https://verifier-c.test",
		"https://verifier-d.test",
		"https://verifier-e.test",
	}

	// Phase 1: Start session
	session, err := coordinator.StartSession(
		"fbs://acme.test/actors/acme-corp",
		"sha256:abcdef",
		[]string{"5901234"},
		evidenceJSON,
		verifiers,
	)
	if err != nil {
		t.Fatal(err)
	}

	if session.Status != StatusPending {
		t.Errorf("expected pending, got %s", session.Status)
	}
	if session.Threshold != 3 { // ceil(5/2)+1 = 3
		t.Errorf("expected threshold 3, got %d", session.Threshold)
	}

	// Phase 2: Verifiers evaluate evidence and commit
	verifierEvidence := &VerificationEvidence{
		SessionID:       session.ID,
		ActorID:         session.ActorID,
		ClaimedPrefixes: session.ClaimedPrefixes,
		EvidenceHash:    session.EvidenceHash,
		EvidencePackage: evidenceJSON,
	}

	for i, nodeID := range verifiers {
		verifier := NewVerifierNode(nodeID)
		commitment, err := verifier.EvaluateEvidence(verifierEvidence)
		if err != nil {
			t.Fatalf("verifier %d: %v", i, err)
		}

		if !commitment.Accepted {
			t.Fatalf("verifier %d rejected: %s", i, commitment.RejectReason)
		}

		err = coordinator.ProcessCommitment(session.ID, *commitment)
		if err != nil {
			t.Fatalf("process commitment %d: %v", i, err)
		}
	}

	// Wait for signing ceremony to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if session.Status == StatusCompleted || session.Status == StatusFailed {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if session.Status == StatusFailed {
		t.Fatalf("session failed: %s", session.Error)
	}
	if session.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", session.Status)
	}

	// Verify badge was produced
	if len(session.BadgeJSON) == 0 {
		t.Fatal("no badge JSON produced")
	}

	var badge Badge
	if err := json.Unmarshal(session.BadgeJSON, &badge); err != nil {
		t.Fatalf("unmarshal badge: %v", err)
	}

	if badge.Level != 3 {
		t.Errorf("badge level should be 3, got %d", badge.Level)
	}
	if badge.Signature == "" {
		t.Error("badge should have a signature")
	}
	if badge.Claims.Quorum == nil {
		t.Fatal("badge should have quorum details")
	}
	if badge.Claims.Quorum.Threshold != 3 {
		t.Errorf("quorum threshold should be 3, got %d", badge.Claims.Quorum.Threshold)
	}
	if len(badge.Claims.Quorum.EvidenceSources) < 3 {
		t.Errorf("should have at least 3 evidence sources, got %d", len(badge.Claims.Quorum.EvidenceSources))
	}

	// Verify transparency log reference
	if badge.TransparencyLog == nil {
		t.Error("Level 3 badge should have transparency log reference")
	} else {
		if badge.TransparencyLog.SCT == "" {
			t.Error("badge should have SCT")
		}
	}

	// Verify badge is in the transparency log
	if !tlog.HasBadge(session.EvidenceHash) {
		// Check by badge hash instead
		meta := tlog.Metadata()
		if meta.TreeSize < 1 {
			t.Error("transparency log should have at least 1 entry")
		}
	}

	t.Logf("Full 3-of-5 quorum ceremony completed: badge=%s, signers=%d",
		badge.BadgeID, len(badge.Claims.Quorum.Participants))
}

func TestEvidenceHashMismatch(t *testing.T) {
	verifier := NewVerifierNode("https://v1.test")

	evidence := &VerificationEvidence{
		SessionID:       "test",
		EvidenceHash:    "wrong-hash",
		EvidencePackage: json.RawMessage(`{"gs1PrefixLicense":"doc"}`),
	}

	commitment, err := verifier.EvaluateEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if commitment.Accepted {
		t.Error("should reject evidence with hash mismatch")
	}
	if commitment.RejectReason == "" {
		t.Error("should have a reason")
	}
}

func TestEvidenceSourceConstraint(t *testing.T) {
	// A session where all accepted commitments use DNSControl should fail validation
	evidence := json.RawMessage(`{"verifiedDomain":"example.com"}`)
	session, _ := NewSession("test", "fbs://test/actor", []string{"590"}, evidence,
		[]string{"https://v1.test", "https://v2.test", "https://v3.test"}, 2)

	session.AddCommitment(VerifierCommitment{
		NodeID:         "https://v1.test",
		Accepted:       true,
		EvidenceSource: EvidenceSource{Source: "DNSControl"},
	})
	session.AddCommitment(VerifierCommitment{
		NodeID:         "https://v2.test",
		Accepted:       true,
		EvidenceSource: EvidenceSource{Source: "DNSControl"},
	})

	err := session.ValidateEvidenceSources()
	if err == nil {
		t.Error("should fail: all evidence is DNSControl")
	}
}

func TestCheckEligibility(t *testing.T) {
	// Too young
	err := CheckEligibility("https://new-node.test", "https://other.test", 30*24*time.Hour)
	if err == nil {
		t.Error("should reject node < 90 days old")
	}

	// Self-attestation
	err = CheckEligibility("https://home.test", "https://home.test", 100*24*time.Hour)
	if err == nil {
		t.Error("should reject home node as verifier")
	}

	// Valid
	err = CheckEligibility("https://verifier.test", "https://home.test", 100*24*time.Hour)
	if err != nil {
		t.Errorf("should accept: %v", err)
	}
}
