package crypto

import (
	"testing"
)

func TestFROSTEndToEnd(t *testing.T) {
	// Test the full FROST protocol: dealer keygen, round 1, round 2, aggregate, verify
	threshold := 3
	numParticipants := 5

	// Dealer generates key shares
	setup, err := FROSTDealerGenerate(threshold, numParticipants)
	if err != nil {
		t.Fatalf("DealerGenerate: %v", err)
	}

	if len(setup.GroupPublic) != 32 {
		t.Fatalf("group public key should be 32 bytes, got %d", len(setup.GroupPublic))
	}
	if len(setup.Shares) != numParticipants {
		t.Fatalf("expected %d shares, got %d", numParticipants, len(setup.Shares))
	}

	message := []byte(`{"fbs":"1.0","type":"VerificationBadge","level":3}`)

	// Select threshold signers (first 3 of 5)
	signerIDs := []int{1, 2, 3}
	signerShares := setup.Shares[:threshold]

	// Round 1: Each signer generates nonces
	nonces := make([]*FROSTNonceCommitment, threshold)
	publics := make([]FROSTNoncePublic, threshold)
	for i, share := range signerShares {
		nc, err := FROSTRound1(share.ID)
		if err != nil {
			t.Fatalf("Round1 for participant %d: %v", share.ID, err)
		}
		nonces[i] = nc
		publics[i] = nc.NoncePublic()
	}

	// Round 2: Each signer produces a partial signature
	partialSigs := make([]FROSTPartialSig, threshold)
	for i, share := range signerShares {
		ps, err := FROSTRound2(&share, nonces[i], message, publics, signerIDs)
		if err != nil {
			t.Fatalf("Round2 for participant %d: %v", share.ID, err)
		}
		partialSigs[i] = *ps
	}

	// Aggregate partial signatures
	signature, err := FROSTAggregate(message, publics, partialSigs)
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}

	if len(signature) != 64 {
		t.Fatalf("signature should be 64 bytes, got %d", len(signature))
	}

	// Verify the aggregate signature against the group public key
	if !FROSTVerify(setup.GroupPublic, message, signature) {
		t.Fatal("FROST signature verification failed")
	}

	// Verify that a tampered message fails
	if FROSTVerify(setup.GroupPublic, []byte("tampered"), signature) {
		t.Fatal("FROST signature should not verify against tampered message")
	}

	t.Logf("FROST %d-of-%d signing succeeded: signature is 64 bytes, verified against group key", threshold, numParticipants)
}

func TestFROSTDifferentSignerSubsets(t *testing.T) {
	// Verify that any threshold-sized subset of participants can sign
	setup, err := FROSTDealerGenerate(2, 4)
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("test-message-for-subset")

	// Try different pairs of signers
	subsets := [][]int{
		{1, 2},
		{1, 3},
		{1, 4},
		{2, 3},
		{2, 4},
		{3, 4},
	}

	for _, signerIDs := range subsets {
		nonces := make([]*FROSTNonceCommitment, len(signerIDs))
		publics := make([]FROSTNoncePublic, len(signerIDs))
		for i, id := range signerIDs {
			nc, err := FROSTRound1(id)
			if err != nil {
				t.Fatalf("Round1 (subset %v, id %d): %v", signerIDs, id, err)
			}
			nonces[i] = nc
			publics[i] = nc.NoncePublic()
		}

		partials := make([]FROSTPartialSig, len(signerIDs))
		for i, id := range signerIDs {
			share := setup.Shares[id-1]
			ps, err := FROSTRound2(&share, nonces[i], message, publics, signerIDs)
			if err != nil {
				t.Fatalf("Round2 (subset %v, id %d): %v", signerIDs, id, err)
			}
			partials[i] = *ps
		}

		sig, err := FROSTAggregate(message, publics, partials)
		if err != nil {
			t.Fatalf("Aggregate (subset %v): %v", signerIDs, err)
		}

		if !FROSTVerify(setup.GroupPublic, message, sig) {
			t.Fatalf("Verify failed for signer subset %v", signerIDs)
		}
	}
	t.Log("All 6 signer subsets of 2-of-4 verified successfully")
}

func TestFROSTMinimumParameters(t *testing.T) {
	// FBS requires minimum quorum size 3, threshold 2
	_, err := FROSTDealerGenerate(1, 3)
	if err == nil {
		t.Error("should reject threshold < 2")
	}

	_, err = FROSTDealerGenerate(4, 3)
	if err == nil {
		t.Error("should reject threshold > numParticipants")
	}

	setup, err := FROSTDealerGenerate(2, 3)
	if err != nil {
		t.Fatalf("2-of-3 should succeed: %v", err)
	}
	if setup.Threshold != 2 || setup.NumParticipants != 3 {
		t.Error("unexpected parameters")
	}
}
