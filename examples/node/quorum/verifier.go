package quorum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// VerificationEvidence is the evidence package sent to verifier nodes.
type VerificationEvidence struct {
	SessionID       string          `json:"sessionId"`
	ActorID         string          `json:"actorId"`
	ClaimedPrefixes []string        `json:"claimedPrefixes"`
	EvidenceHash    string          `json:"evidenceHash"` // SHA-256 commitment
	EvidencePackage json.RawMessage `json:"evidencePackage"`
}

// VerifierNode represents a node's role as an independent verifier
// in the Level 3 quorum protocol.
type VerifierNode struct {
	NodeID string
}

// NewVerifierNode creates a new verifier instance.
func NewVerifierNode(nodeID string) *VerifierNode {
	return &VerifierNode{NodeID: nodeID}
}

// EvaluateEvidence performs independent verification of the evidence package
// against an external evidence source (Section 4.5.6).
//
// In a production implementation, this would make real external API calls
// to GS1 GEPIR, Companies House, etc. This reference implementation
// simulates the verification by checking the evidence structure.
func (v *VerifierNode) EvaluateEvidence(evidence *VerificationEvidence) (*VerifierCommitment, error) {
	// Section 4.6.5: Verify evidence hash commitment
	actualHash := sha256.Sum256(evidence.EvidencePackage)
	actualHashHex := hex.EncodeToString(actualHash[:])
	if actualHashHex != evidence.EvidenceHash {
		return &VerifierCommitment{
			NodeID:       v.NodeID,
			Accepted:     false,
			RejectReason: "evidence hash mismatch: integrity failure",
		}, nil
	}

	// Parse evidence to determine what sources are available
	var pkg map[string]interface{}
	if err := json.Unmarshal(evidence.EvidencePackage, &pkg); err != nil {
		return &VerifierCommitment{
			NodeID:       v.NodeID,
			Accepted:     false,
			RejectReason: "invalid evidence package JSON",
		}, nil
	}

	// Determine which evidence source this verifier will use
	source := v.selectEvidenceSource(pkg)
	if source == "" {
		return &VerifierCommitment{
			NodeID:       v.NodeID,
			Accepted:     false,
			RejectReason: "no verifiable evidence source found",
		}, nil
	}

	// Simulate external verification
	verified, reason := v.verifyAgainstSource(source, pkg, evidence.ClaimedPrefixes)
	if !verified {
		return &VerifierCommitment{
			NodeID:       v.NodeID,
			Accepted:     false,
			RejectReason: reason,
		}, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	return &VerifierCommitment{
		NodeID:   v.NodeID,
		Accepted: true,
		EvidenceSource: EvidenceSource{
			Node:       v.NodeID,
			Source:     source,
			VerifiedAt: now,
		},
	}, nil
}

// selectEvidenceSource determines which external source to verify against
// based on what's available in the evidence package.
func (v *VerifierNode) selectEvidenceSource(pkg map[string]interface{}) string {
	// Priority: GS1CompanyPrefix > BusinessRegistration > DNSControl > TrademarkRecord
	if _, ok := pkg["gs1PrefixLicense"]; ok {
		return "GS1CompanyPrefix"
	}
	if _, ok := pkg["businessRegistration"]; ok {
		return "BusinessRegistration"
	}
	if _, ok := pkg["verifiedDomain"]; ok {
		return "DNSControl"
	}
	if _, ok := pkg["trademarkRecord"]; ok {
		return "TrademarkRecord"
	}
	return ""
}

// verifyAgainstSource simulates external verification.
// In production, this would make real API calls.
func (v *VerifierNode) verifyAgainstSource(source string, pkg map[string]interface{}, claimedPrefixes []string) (bool, string) {
	switch source {
	case "GS1CompanyPrefix":
		license, ok := pkg["gs1PrefixLicense"]
		if !ok || license == "" {
			return false, "missing gs1PrefixLicense in evidence"
		}
		// Production: query GS1 GEPIR to verify prefix ownership
		// Reference: accept if evidence is present and non-empty
		return true, ""

	case "BusinessRegistration":
		reg, ok := pkg["businessRegistration"]
		if !ok || reg == "" {
			return false, "missing businessRegistration in evidence"
		}
		// Production: query Companies House / BRIS / SEC EDGAR
		return true, ""

	case "DNSControl":
		domain, ok := pkg["verifiedDomain"]
		if !ok || domain == "" {
			return false, "missing verifiedDomain in evidence"
		}
		// Production: perform DNS TXT challenge verification
		return true, ""

	case "TrademarkRecord":
		record, ok := pkg["trademarkRecord"]
		if !ok || record == "" {
			return false, "missing trademarkRecord in evidence"
		}
		// Production: query USPTO / EUIPO
		return true, ""

	default:
		return false, fmt.Sprintf("unknown evidence source: %s", source)
	}
}

// CheckEligibility verifies a node meets quorum eligibility per Section 4.6.2.
func CheckEligibility(nodeID string, subjectActorHomeNode string, nodeAge time.Duration) error {
	if nodeAge < 90*24*time.Hour {
		return fmt.Errorf("node must be operating for at least 90 days (has %s)", nodeAge)
	}
	// Check: node is not the home node of the subject actor
	if nodeID == subjectActorHomeNode {
		return fmt.Errorf("verifier cannot be the subject actor's home node")
	}
	return nil
}
