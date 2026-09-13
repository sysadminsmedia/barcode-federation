package quorum

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/transparencylog"
	"github.com/google/uuid"
)

// Badge represents the full FBS Verification Badge per Section 4.5.3.
type Badge struct {
	FBS                    string          `json:"fbs"`
	Type                   string          `json:"type"`
	BadgeID                string          `json:"badgeId"`
	Level                  int             `json:"level"`
	Subject                string          `json:"subject"`
	SubjectKeyFingerprint  string          `json:"subjectKeyFingerprint"`
	IssuedBy               string          `json:"issuedBy"`
	IssuedAt               string          `json:"issuedAt"`
	ExpiresAt              string          `json:"expiresAt"`
	Claims                 BadgeClaims     `json:"claims"`
	TransparencyLog        *TransparencyRef `json:"transparencyLog,omitempty"`
	Signature              string          `json:"signature"`
}

// BadgeClaims contains the verification claims within a badge.
type BadgeClaims struct {
	Level              int              `json:"level"`
	VerifiedDomain     string           `json:"verifiedDomain,omitempty"`
	VerifiedPrefixes   []string         `json:"verifiedPrefixes"`
	VerificationMethod string           `json:"verificationMethod"`
	Quorum             *QuorumDetails   `json:"quorum,omitempty"`
}

// QuorumDetails captures the quorum composition for Level 3 badges.
type QuorumDetails struct {
	Threshold       int              `json:"threshold"`
	Size            int              `json:"size"`
	Participants    []string         `json:"participants"`
	EvidenceSources []EvidenceSource `json:"evidenceSources"`
}

// TransparencyRef holds the SCT and log reference in a badge.
type TransparencyRef struct {
	LogID         string `json:"logId"`
	SCT           string `json:"sct"`
	MergeDeadline string `json:"mergeDeadline"`
}

// Coordinator orchestrates the full Level 3 quorum verification ceremony.
type Coordinator struct {
	NodeBaseURL     string
	NodeKey         ed25519.PrivateKey
	Log             *transparencylog.TransparencyLog
	Sessions        map[string]*Session
}

// NewCoordinator creates a quorum coordinator.
func NewCoordinator(nodeBaseURL string, nodeKey ed25519.PrivateKey, log *transparencylog.TransparencyLog) *Coordinator {
	return &Coordinator{
		NodeBaseURL: nodeBaseURL,
		NodeKey:     nodeKey,
		Log:         log,
		Sessions:    make(map[string]*Session),
	}
}

// StartSession initiates a new Level 3 verification session (Phase 1).
func (c *Coordinator) StartSession(
	actorID string,
	subjectKeyFingerprint string,
	claimedPrefixes []string,
	evidencePackage json.RawMessage,
	verifierNodes []string,
) (*Session, error) {
	sessionID := uuid.New().String()

	// FBS requires threshold >= ceil(n/2)+1 (strict majority)
	n := len(verifierNodes)
	threshold := n/2 + 1
	if threshold < 2 {
		threshold = 2
	}

	session, err := NewSession(sessionID, actorID, claimedPrefixes, evidencePackage, verifierNodes, threshold)
	if err != nil {
		return nil, err
	}

	c.Sessions[sessionID] = session
	slog.Info("quorum session started",
		"session", sessionID,
		"actor", actorID,
		"verifiers", len(verifierNodes),
		"threshold", threshold,
	)
	return session, nil
}

// GetSession retrieves a session by ID.
func (c *Coordinator) GetSession(sessionID string) (*Session, bool) {
	s, ok := c.Sessions[sessionID]
	return s, ok
}

// ProcessCommitment handles a verifier's commitment (Phase 2 response).
func (c *Coordinator) ProcessCommitment(sessionID string, commitment VerifierCommitment) error {
	session, ok := c.Sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	thresholdReached, err := session.AddCommitment(commitment)
	if err != nil {
		return err
	}

	if thresholdReached {
		slog.Info("quorum threshold reached", "session", sessionID)
		go c.runSigningCeremony(session)
	}

	return nil
}

// runSigningCeremony executes Phases 3-5 of the quorum protocol.
func (c *Coordinator) runSigningCeremony(session *Session) {
	// Phase 3: Construct attestation object
	accepted := session.AcceptedCommitments()

	if err := session.ValidateEvidenceSources(); err != nil {
		session.SetError("evidence validation: " + err.Error())
		return
	}

	// Collect evidence sources and participant info
	evidenceSources := make([]EvidenceSource, 0, len(accepted))
	participants := make([]string, 0, len(accepted))
	signerIDs := make([]int, 0, len(accepted))

	for _, comm := range accepted {
		evidenceSources = append(evidenceSources, comm.EvidenceSource)
		participants = append(participants, comm.NodeID)
		pid, _ := session.ParticipantID(comm.NodeID)
		signerIDs = append(signerIDs, pid)
	}

	// Build the badge (minus signature and SCT)
	now := time.Now().UTC()
	badge := Badge{
		FBS:     "1.0",
		Type:    "VerificationBadge",
		BadgeID: c.NodeBaseURL + "/badges/" + uuid.New().String(),
		Level:   3,
		Subject: session.ActorID,
		SubjectKeyFingerprint: "", // Filled by caller
		IssuedBy:  c.NodeBaseURL,
		IssuedAt:  now.Format(time.RFC3339),
		ExpiresAt: now.Add(2 * 365 * 24 * time.Hour).Format(time.RFC3339), // 2 years
		Claims: BadgeClaims{
			Level:              3,
			VerifiedPrefixes:   session.ClaimedPrefixes,
			VerificationMethod: "QuorumThreshold",
			Quorum: &QuorumDetails{
				Threshold:       session.Threshold,
				Size:            session.QuorumSize,
				Participants:    participants,
				EvidenceSources: evidenceSources,
			},
		},
	}

	// Phase 4: FROST signing
	session.SetStatus(StatusSigning)

	// Generate FROST key shares for this session (trusted dealer model per Appendix B.1)
	setup, err := fbscrypto.FROSTDealerGenerate(session.Threshold, len(accepted))
	if err != nil {
		session.SetError("FROST key generation: " + err.Error())
		return
	}

	// Serialize attestation object (badge without signature)
	attestationJSON, err := fbscrypto.CanonicalJSON(badge)
	if err != nil {
		session.SetError("canonical JSON: " + err.Error())
		return
	}
	session.AttestationJSON = attestationJSON

	// Execute FROST Round 1 for all participants
	nonces := make([]*fbscrypto.FROSTNonceCommitment, len(accepted))
	publics := make([]fbscrypto.FROSTNoncePublic, len(accepted))
	for i := range accepted {
		nc, err := fbscrypto.FROSTRound1(signerIDs[i])
		if err != nil {
			session.SetError("FROST round 1: " + err.Error())
			return
		}
		nonces[i] = nc
		publics[i] = nc.NoncePublic()
	}

	// Execute FROST Round 2 for all participants
	partialSigs := make([]fbscrypto.FROSTPartialSig, len(accepted))
	for i := range accepted {
		ps, err := fbscrypto.FROSTRound2(&setup.Shares[i], nonces[i], attestationJSON, publics, signerIDs)
		if err != nil {
			session.SetError("FROST round 2: " + err.Error())
			return
		}
		partialSigs[i] = *ps
	}

	// Aggregate signature
	signature, err := fbscrypto.FROSTAggregate(attestationJSON, publics, partialSigs)
	if err != nil {
		session.SetError("FROST aggregate: " + err.Error())
		return
	}

	// Verify the aggregate signature
	if !fbscrypto.FROSTVerify(setup.GroupPublic, attestationJSON, signature) {
		session.SetError("FROST signature verification failed after aggregation")
		return
	}

	badge.Signature = base64.RawURLEncoding.EncodeToString(signature)
	session.Signature = signature

	// Phase 5: Submit to transparency log
	session.SetStatus(StatusSubmittingLog)

	badgeJSON, err := json.Marshal(badge)
	if err != nil {
		session.SetError("marshal badge: " + err.Error())
		return
	}

	if c.Log != nil {
		sct, err := c.Log.AddBadge(badgeJSON, c.NodeBaseURL)
		if err != nil {
			slog.Warn("failed to submit badge to transparency log", "error", err)
			// Continue without SCT for non-fatal (log may be external)
		} else {
			badge.TransparencyLog = &TransparencyRef{
				LogID:         sct.LogID,
				SCT:           sct.Signature,
				MergeDeadline: sct.MergeDeadline,
			}
			// Re-marshal with SCT
			badgeJSON, _ = json.Marshal(badge)
		}
	}

	session.BadgeJSON = badgeJSON
	session.SetStatus(StatusCompleted)

	slog.Info("quorum verification completed",
		"session", session.ID,
		"badge", badge.BadgeID,
		"level", 3,
		"signers", len(accepted),
		"threshold", session.Threshold,
	)
}

// VerifyBadge validates a Level 3 badge per Section 4.5.7 steps 5-6.
func VerifyBadge(badge *Badge, tlog *transparencylog.TransparencyLog) error {
	if badge.Level != 3 {
		return fmt.Errorf("not a Level 3 badge")
	}

	if badge.Claims.Quorum == nil {
		return fmt.Errorf("Level 3 badge missing quorum details")
	}

	q := badge.Claims.Quorum

	// Check threshold was met
	if len(q.EvidenceSources) < q.Threshold {
		return fmt.Errorf("evidence sources (%d) < threshold (%d)", len(q.EvidenceSources), q.Threshold)
	}

	// Check evidence source constraint: not all DNSControl
	hasStrong := false
	for _, es := range q.EvidenceSources {
		if es.Source == "GS1CompanyPrefix" || es.Source == "BusinessRegistration" {
			hasStrong = true
			break
		}
	}
	if !hasStrong {
		return fmt.Errorf("no GS1CompanyPrefix or BusinessRegistration evidence in quorum")
	}

	// Check transparency log (Section 4.5.7 step 6)
	if badge.TransparencyLog == nil {
		return fmt.Errorf("Level 3 badge missing transparency log reference")
	}

	if tlog != nil {
		badgeJSON, err := json.Marshal(badge)
		if err != nil {
			return fmt.Errorf("marshal badge: %w", err)
		}
		badgeHash := sha256.Sum256(badgeJSON)
		badgeHashHex := hex.EncodeToString(badgeHash[:])
		if !tlog.HasBadge(badgeHashHex) {
			return fmt.Errorf("badge not found in transparency log")
		}
	}

	return nil
}
