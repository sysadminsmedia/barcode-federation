// Package transparencylog implements the FBS Verification Transparency Log
// per RFC FBS0001 Section 4.7, modeled on Certificate Transparency (RFC 9162).
//
// The log is a Merkle tree over an ordered sequence of badge entries.
// It provides append-only, tamper-evident storage for all Verification Badges.
package transparencylog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// RFC 6962 / 9162 domain separation prefixes for Merkle tree hashing.
const (
	leafPrefix = 0x00
	nodePrefix = 0x01
)

// MerkleTree is an append-only Merkle hash tree.
type MerkleTree struct {
	Leaves [][]byte // SHA-256 leaf hashes
}

// NewMerkleTree creates an empty Merkle tree.
func NewMerkleTree() *MerkleTree {
	return &MerkleTree{}
}

// AddLeaf appends a new entry to the tree and returns its leaf index.
func (t *MerkleTree) AddLeaf(data []byte) int {
	h := leafHash(data)
	t.Leaves = append(t.Leaves, h)
	return len(t.Leaves) - 1
}

// Size returns the number of leaves in the tree.
func (t *MerkleTree) Size() int {
	return len(t.Leaves)
}

// RootHash computes the Merkle tree root hash over all current leaves.
// Returns empty hash for an empty tree.
func (t *MerkleTree) RootHash() []byte {
	if len(t.Leaves) == 0 {
		return make([]byte, 32)
	}
	return computeRoot(t.Leaves, 0, len(t.Leaves))
}

// RootHashHex returns the root hash as a hex string.
func (t *MerkleTree) RootHashHex() string {
	return hex.EncodeToString(t.RootHash())
}

// InclusionProof generates a Merkle inclusion proof for the leaf at the given index.
// The proof is a sequence of sibling hashes needed to recompute the root.
func (t *MerkleTree) InclusionProof(leafIndex int) ([]ProofNode, error) {
	if leafIndex < 0 || leafIndex >= len(t.Leaves) {
		return nil, fmt.Errorf("leaf index %d out of range [0, %d)", leafIndex, len(t.Leaves))
	}
	if len(t.Leaves) == 1 {
		return []ProofNode{}, nil // single leaf, root == leaf
	}
	return buildProof(t.Leaves, leafIndex, 0, len(t.Leaves)), nil
}

// VerifyInclusion verifies that a leaf hash is included in the tree with the given root.
func VerifyInclusion(leafData []byte, leafIndex, treeSize int, proof []ProofNode, rootHash []byte) bool {
	h := leafHash(leafData)
	return verifyPath(h, leafIndex, treeSize, proof, rootHash)
}

// ProofNode is a single node in a Merkle inclusion proof.
type ProofNode struct {
	Hash  []byte `json:"hash"`
	Left  bool   `json:"left"` // true if this hash is the left sibling
}

// --- Internal Merkle tree operations ---

func leafHash(data []byte) []byte {
	h := sha256.New()
	h.Write([]byte{leafPrefix})
	h.Write(data)
	return h.Sum(nil)
}

func nodeHash(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte{nodePrefix})
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// computeRoot recursively computes the Merkle root over leaves[start:end].
func computeRoot(leaves [][]byte, start, end int) []byte {
	n := end - start
	if n == 1 {
		return leaves[start]
	}
	// Split at the largest power of 2 less than n
	k := largestPowerOf2LessThan(n)
	left := computeRoot(leaves, start, start+k)
	right := computeRoot(leaves, start+k, end)
	return nodeHash(left, right)
}

// buildProof generates the inclusion proof for leaves[target] within leaves[start:end].
func buildProof(leaves [][]byte, target, start, end int) []ProofNode {
	n := end - start
	if n == 1 {
		return nil
	}
	k := largestPowerOf2LessThan(n)
	if target-start < k {
		// Target is in the left subtree
		proof := buildProof(leaves, target, start, start+k)
		rightHash := computeRoot(leaves, start+k, end)
		return append(proof, ProofNode{Hash: rightHash, Left: false})
	}
	// Target is in the right subtree
	proof := buildProof(leaves, target, start+k, end)
	leftHash := computeRoot(leaves, start, start+k)
	return append(proof, ProofNode{Hash: leftHash, Left: true})
}

// verifyPath recomputes the root from a leaf hash and proof, then compares.
func verifyPath(leafH []byte, leafIndex, treeSize int, proof []ProofNode, rootHash []byte) bool {
	h := leafH
	idx := leafIndex
	size := treeSize

	for _, node := range proof {
		if node.Left {
			h = nodeHash(node.Hash, h)
		} else {
			h = nodeHash(h, node.Hash)
		}
		k := largestPowerOf2LessThan(size)
		if idx < k {
			size = k
		} else {
			idx -= k
			size -= k
		}
	}

	if len(h) != len(rootHash) {
		return false
	}
	for i := range h {
		if h[i] != rootHash[i] {
			return false
		}
	}
	return true
}

func largestPowerOf2LessThan(n int) int {
	if n <= 1 {
		return 1
	}
	k := 1
	for k*2 < n {
		k *= 2
	}
	return k
}
