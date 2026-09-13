// Package quorum implements the FBS Level 3 Verifier Quorum Protocol
// per RFC FBS0001 Section 4.6. It coordinates independent FBS nodes to
// produce a FROST threshold signature over a Verification Badge.
package quorum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SessionStatus tracks the state of a quorum verification session.
type SessionStatus string

const (
	StatusPending        SessionStatus = "pending"
	StatusCollecting     SessionStatus = "collecting"     // Phase 2: collecting verifier commitments
	StatusSigning        SessionStatus = "signing"        // Phase 4: FROST signing round
	StatusSubmittingLog  SessionStatus = "submitting_log" // Phase 5: submitting to transparency log
	StatusCompleted      SessionStatus = "completed"
	StatusFailed         SessionStatus = "failed"
)

// ValidEvidenceSources are the acceptable external evidence sources per Section 4.5.6.
var ValidEvidenceSources = map[string]bool{
	"GS1CompanyPrefix":     true,
	"DNSControl":           true,
	"BusinessRegistration": true,
	"TrademarkRecord":      true,
}

// EvidenceSource records which source a verifier used.
type EvidenceSource struct {
	Node       string `json:"node"`
	Source     string `json:"source"`
	VerifiedAt string `json:"verifiedAt"`
}

// VerifierCommitment is a verifier's response to Phase 2.
type VerifierCommitment struct {
	NodeID         string         `json:"nodeId"`
	Accepted       bool           `json:"accepted"`
	RejectReason   string         `json:"rejectReason,omitempty"`
	EvidenceSource EvidenceSource `json:"evidenceSource,omitempty"`
	// FROST Round 1 nonce commitments
	NonceHidingCommit  []byte `json:"nonceHidingCommit,omitempty"`
	NonceBindingCommit []byte `json:"nonceBindingCommit,omitempty"`
}

// VerifierPartialSig is a verifier's Round 2 output.
type VerifierPartialSig struct {
	NodeID        string `json:"nodeId"`
	ParticipantID int    `json:"participantId"`
	Z             []byte `json:"z"`
}

// Session manages the full lifecycle of a Level 3 verification.
type Session struct {
	mu sync.Mutex

	ID              string              `json:"id"`
	Status          SessionStatus       `json:"status"`
	ActorID         string              `json:"actorId"`
	ClaimedPrefixes []string            `json:"claimedPrefixes"`
	EvidenceHash    string              `json:"evidenceHash"`
	Threshold       int                 `json:"threshold"`
	QuorumSize      int                 `json:"quorumSize"`
	VerifierNodes   []string            `json:"verifierNodes"`
	Commitments     []VerifierCommitment `json:"commitments"`
	PartialSigs     []VerifierPartialSig `json:"partialSigs,omitempty"`
	AttestationJSON []byte              `json:"attestationJson,omitempty"`
	BadgeJSON       []byte              `json:"badgeJson,omitempty"`
	Signature       []byte              `json:"signature,omitempty"`
	SCT             json.RawMessage     `json:"sct,omitempty"`
	Error           string              `json:"error,omitempty"`
	CreatedAt       time.Time           `json:"createdAt"`
	UpdatedAt       time.Time           `json:"updatedAt"`

	// Participant ID -> node ID mapping for FROST
	nodeToParticipant map[string]int
}

// NewSession creates a new quorum verification session.
// The coordinator selects verifier nodes and computes the evidence hash commitment.
func NewSession(
	sessionID string,
	actorID string,
	claimedPrefixes []string,
	evidencePackage json.RawMessage,
	verifierNodes []string,
	threshold int,
) (*Session, error) {
	if len(verifierNodes) < 3 {
		return nil, fmt.Errorf("quorum: minimum 3 verifier nodes required, got %d", len(verifierNodes))
	}
	if threshold < 2 {
		return nil, fmt.Errorf("quorum: minimum threshold is 2, got %d", threshold)
	}
	if threshold > len(verifierNodes) {
		return nil, fmt.Errorf("quorum: threshold %d exceeds quorum size %d", threshold, len(verifierNodes))
	}

	// Compute evidence hash commitment (Section 4.6.5)
	evidenceHash := sha256.Sum256(evidencePackage)

	nodeMap := make(map[string]int)
	for i, nodeID := range verifierNodes {
		nodeMap[nodeID] = i + 1 // 1-based participant IDs
	}

	now := time.Now().UTC()
	return &Session{
		ID:                sessionID,
		Status:            StatusPending,
		ActorID:           actorID,
		ClaimedPrefixes:   claimedPrefixes,
		EvidenceHash:      hex.EncodeToString(evidenceHash[:]),
		Threshold:         threshold,
		QuorumSize:        len(verifierNodes),
		VerifierNodes:     verifierNodes,
		Commitments:       make([]VerifierCommitment, 0),
		CreatedAt:         now,
		UpdatedAt:         now,
		nodeToParticipant: nodeMap,
	}, nil
}

// AddCommitment records a verifier's Phase 2 response.
// Returns true if the threshold has been reached.
func (s *Session) AddCommitment(c VerifierCommitment) (thresholdReached bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status != StatusPending && s.Status != StatusCollecting {
		return false, fmt.Errorf("session in wrong state: %s", s.Status)
	}
	s.Status = StatusCollecting

	// Check this node hasn't already committed
	for _, existing := range s.Commitments {
		if existing.NodeID == c.NodeID {
			return false, fmt.Errorf("verifier %s already committed", c.NodeID)
		}
	}

	s.Commitments = append(s.Commitments, c)
	s.UpdatedAt = time.Now().UTC()

	// Count accepted commitments
	accepted := 0
	for _, comm := range s.Commitments {
		if comm.Accepted {
			accepted++
		}
	}

	// Only signal threshold reached the first time
	return accepted == s.Threshold, nil
}

// AcceptedCommitments returns only the commitments from verifiers that accepted.
func (s *Session) AcceptedCommitments() []VerifierCommitment {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []VerifierCommitment
	for _, c := range s.Commitments {
		if c.Accepted {
			result = append(result, c)
		}
	}
	return result
}

// ParticipantID returns the FROST participant ID for a given node.
func (s *Session) ParticipantID(nodeID string) (int, error) {
	id, ok := s.nodeToParticipant[nodeID]
	if !ok {
		return 0, fmt.Errorf("node %s not in quorum", nodeID)
	}
	return id, nil
}

// ValidateEvidenceSources checks the Level 3 evidence source constraint:
// at least one source in the threshold set must be GS1CompanyPrefix or BusinessRegistration.
func (s *Session) ValidateEvidenceSources() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasStrongEvidence := false
	allDNS := true

	for _, c := range s.Commitments {
		if !c.Accepted {
			continue
		}
		src := c.EvidenceSource.Source
		if !ValidEvidenceSources[src] {
			return fmt.Errorf("invalid evidence source: %s", src)
		}
		if src != "DNSControl" {
			allDNS = false
		}
		if src == "GS1CompanyPrefix" || src == "BusinessRegistration" {
			hasStrongEvidence = true
		}
	}

	if allDNS {
		return fmt.Errorf("quorum cannot achieve threshold using only DNSControl evidence")
	}
	if !hasStrongEvidence {
		return fmt.Errorf("at least one verifier must use GS1CompanyPrefix or BusinessRegistration")
	}
	return nil
}

// SetStatus updates the session status.
func (s *Session) SetStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.UpdatedAt = time.Now().UTC()
}

// SetError marks the session as failed with an error message.
func (s *Session) SetError(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = StatusFailed
	s.Error = errMsg
	s.UpdatedAt = time.Now().UTC()
}
