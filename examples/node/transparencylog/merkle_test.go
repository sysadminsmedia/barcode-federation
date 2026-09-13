package transparencylog

import (
	"fmt"
	"testing"
)

func TestMerkleTree_Empty(t *testing.T) {
	tree := NewMerkleTree()
	if tree.Size() != 0 {
		t.Error("empty tree should have size 0")
	}
	root := tree.RootHash()
	if len(root) != 32 {
		t.Error("empty root should be 32 bytes")
	}
}

func TestMerkleTree_SingleLeaf(t *testing.T) {
	tree := NewMerkleTree()
	idx := tree.AddLeaf([]byte("hello"))
	if idx != 0 {
		t.Errorf("first leaf index should be 0, got %d", idx)
	}
	if tree.Size() != 1 {
		t.Error("size should be 1")
	}

	proof, err := tree.InclusionProof(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof) != 0 {
		t.Error("single-leaf proof should be empty")
	}
}

func TestMerkleTree_InclusionProof(t *testing.T) {
	tree := NewMerkleTree()
	data := [][]byte{
		[]byte("badge-0"),
		[]byte("badge-1"),
		[]byte("badge-2"),
		[]byte("badge-3"),
		[]byte("badge-4"),
	}
	for _, d := range data {
		tree.AddLeaf(d)
	}

	rootHash := tree.RootHash()

	for i, d := range data {
		proof, err := tree.InclusionProof(i)
		if err != nil {
			t.Fatalf("proof for leaf %d: %v", i, err)
		}

		if !VerifyInclusion(d, i, tree.Size(), proof, rootHash) {
			t.Errorf("inclusion verification failed for leaf %d", i)
		}
	}
}

func TestMerkleTree_TamperDetection(t *testing.T) {
	tree := NewMerkleTree()
	tree.AddLeaf([]byte("badge-0"))
	tree.AddLeaf([]byte("badge-1"))
	tree.AddLeaf([]byte("badge-2"))

	rootHash := tree.RootHash()
	proof, _ := tree.InclusionProof(1)

	// Verify correct data passes
	if !VerifyInclusion([]byte("badge-1"), 1, tree.Size(), proof, rootHash) {
		t.Fatal("correct data should verify")
	}

	// Tampered data should fail
	if VerifyInclusion([]byte("tampered"), 1, tree.Size(), proof, rootHash) {
		t.Fatal("tampered data should not verify")
	}
}

func TestMerkleTree_ConsistentRoot(t *testing.T) {
	// Adding the same leaves should produce the same root
	tree1 := NewMerkleTree()
	tree2 := NewMerkleTree()
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf("entry-%d", i))
		tree1.AddLeaf(data)
		tree2.AddLeaf(data)
	}
	root1 := tree1.RootHashHex()
	root2 := tree2.RootHashHex()
	if root1 != root2 {
		t.Errorf("same leaves should produce same root:\n  %s\n  %s", root1, root2)
	}
}

func TestMerkleTree_AppendOnlyRootChanges(t *testing.T) {
	tree := NewMerkleTree()
	tree.AddLeaf([]byte("a"))
	root1 := tree.RootHashHex()

	tree.AddLeaf([]byte("b"))
	root2 := tree.RootHashHex()

	if root1 == root2 {
		t.Error("adding a leaf should change the root hash")
	}
}
