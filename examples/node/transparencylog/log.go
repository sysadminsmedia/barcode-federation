package transparencylog

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
)

// MaxMergeDelay is the maximum time allowed to incorporate a badge into the tree.
// FBS requires this to be no more than 24 hours.
const MaxMergeDelay = 24 * time.Hour

// LogEntry is a single entry in the transparency log.
type LogEntry struct {
	SequenceNumber int             `json:"sequenceNumber"`
	Timestamp      string          `json:"timestamp"`
	BadgeJSON      json.RawMessage `json:"badge"`
	BadgeHash      string          `json:"badgeHash"`
	SubmitterID    string          `json:"submitterId"`
	LeafHash       string          `json:"leafHash"`
}

// SignedTreeHead is the log's current state, signed by the log operator.
type SignedTreeHead struct {
	TreeSize       int    `json:"treeSize"`
	Timestamp      string `json:"timestamp"`
	RootHash       string `json:"rootHash"`
	Signature      string `json:"signature"`
}

// SCT (Signed Certificate Timestamp) is the log's promise that a badge
// will be incorporated into the tree within the MMD.
type SCT struct {
	LogID         string `json:"logId"`
	Timestamp     string `json:"timestamp"`
	BadgeHash     string `json:"badgeHash"`
	Signature     string `json:"signature"`
	MergeDeadline string `json:"mergeDeadline"`
}

// LogMetadata describes a transparency log's configuration.
type LogMetadata struct {
	FBS              string `json:"fbs"`
	Type             string `json:"type"`
	LogID            string `json:"logId"`
	PublicKeyPem     string `json:"publicKeyPem"`
	MaxMergeDelay    int    `json:"maxMergeDelay"`
	TreeSize         int    `json:"treeSize"`
	CurrentRootHash  string `json:"currentRootHash"`
}

// TransparencyLog is the full log operator implementation.
type TransparencyLog struct {
	mu          sync.RWMutex
	logID       string
	logKey      ed25519.PrivateKey
	logPubPEM   string
	tree        *MerkleTree
	entries     []LogEntry
	badgeIndex  map[string]int // badgeHash -> sequence number
}

// NewTransparencyLog creates a new transparency log instance.
func NewTransparencyLog(logID string, logKey ed25519.PrivateKey) (*TransparencyLog, error) {
	pubPEM, err := fbscrypto.EncodePublicKeyPEM(logKey.Public().(ed25519.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("encode log public key: %w", err)
	}

	return &TransparencyLog{
		logID:      logID,
		logKey:     logKey,
		logPubPEM:  pubPEM,
		tree:       NewMerkleTree(),
		entries:    make([]LogEntry, 0),
		badgeIndex: make(map[string]int),
	}, nil
}

// Metadata returns the log's public metadata for the /fbs-log/v1/metadata endpoint.
func (l *TransparencyLog) Metadata() LogMetadata {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return LogMetadata{
		FBS:             "1.0",
		Type:            "TransparencyLogMeta",
		LogID:           l.logID,
		PublicKeyPem:    l.logPubPEM,
		MaxMergeDelay:   int(MaxMergeDelay.Seconds()),
		TreeSize:        l.tree.Size(),
		CurrentRootHash: l.tree.RootHashHex(),
	}
}

// AddBadge adds a badge to the log and returns an SCT.
// This is the /fbs-log/v1/add-badge endpoint implementation.
func (l *TransparencyLog) AddBadge(badgeJSON []byte, submitterID string) (*SCT, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Compute badge hash
	badgeHash := sha256.Sum256(badgeJSON)
	badgeHashHex := hex.EncodeToString(badgeHash[:])

	// Check for duplicates
	if _, exists := l.badgeIndex[badgeHashHex]; exists {
		return nil, fmt.Errorf("badge already submitted to log")
	}

	now := time.Now().UTC()

	// Add leaf to Merkle tree
	leafIndex := l.tree.AddLeaf(badgeJSON)

	// Create log entry
	entry := LogEntry{
		SequenceNumber: leafIndex,
		Timestamp:      now.Format(time.RFC3339),
		BadgeJSON:      badgeJSON,
		BadgeHash:      badgeHashHex,
		SubmitterID:    submitterID,
		LeafHash:       hex.EncodeToString(l.tree.Leaves[leafIndex]),
	}
	l.entries = append(l.entries, entry)
	l.badgeIndex[badgeHashHex] = leafIndex

	// Issue SCT (signed promise to merge within MMD)
	mergeDeadline := now.Add(MaxMergeDelay)
	sct := &SCT{
		LogID:         l.logID,
		Timestamp:     now.Format(time.RFC3339),
		BadgeHash:     badgeHashHex,
		MergeDeadline: mergeDeadline.Format(time.RFC3339),
	}

	// Sign the SCT
	sctData := fmt.Sprintf("%s|%s|%s|%s", sct.LogID, sct.Timestamp, sct.BadgeHash, sct.MergeDeadline)
	sig := ed25519.Sign(l.logKey, []byte(sctData))
	sct.Signature = base64.RawURLEncoding.EncodeToString(sig)

	return sct, nil
}

// GetEntries returns log entries in the given sequence range.
// This is the /fbs-log/v1/get-entries endpoint implementation.
func (l *TransparencyLog) GetEntries(start, end int) ([]LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if start < 0 {
		start = 0
	}
	if end > len(l.entries) {
		end = len(l.entries)
	}
	if start >= end {
		return []LogEntry{}, nil
	}

	result := make([]LogEntry, end-start)
	copy(result, l.entries[start:end])
	return result, nil
}

// GetProofByHash returns an inclusion proof for a badge with the given hash.
// This is the /fbs-log/v1/get-proof-by-hash endpoint implementation.
func (l *TransparencyLog) GetProofByHash(badgeHashHex string) ([]ProofNode, int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	leafIndex, exists := l.badgeIndex[badgeHashHex]
	if !exists {
		return nil, -1, fmt.Errorf("badge not found in log")
	}

	proof, err := l.tree.InclusionProof(leafIndex)
	if err != nil {
		return nil, -1, err
	}
	return proof, leafIndex, nil
}

// GetSignedTreeHead returns the current signed tree head.
// This is the /fbs-log/v1/get-sth endpoint implementation.
func (l *TransparencyLog) GetSignedTreeHead() SignedTreeHead {
	l.mu.RLock()
	defer l.mu.RUnlock()

	rootHash := l.tree.RootHashHex()
	now := time.Now().UTC().Format(time.RFC3339)

	sthData := fmt.Sprintf("%d|%s|%s", l.tree.Size(), now, rootHash)
	sig := ed25519.Sign(l.logKey, []byte(sthData))

	return SignedTreeHead{
		TreeSize:  l.tree.Size(),
		Timestamp: now,
		RootHash:  rootHash,
		Signature: base64.RawURLEncoding.EncodeToString(sig),
	}
}

// VerifySCT verifies an SCT signature against the log's public key.
func VerifySCT(sct *SCT, logPublicKey ed25519.PublicKey) bool {
	sctData := fmt.Sprintf("%s|%s|%s|%s", sct.LogID, sct.Timestamp, sct.BadgeHash, sct.MergeDeadline)
	sig, err := base64.RawURLEncoding.DecodeString(sct.Signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(logPublicKey, []byte(sctData), sig)
}

// HasBadge checks if a badge with the given hash exists in the log.
func (l *TransparencyLog) HasBadge(badgeHashHex string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, exists := l.badgeIndex[badgeHashHex]
	return exists
}
