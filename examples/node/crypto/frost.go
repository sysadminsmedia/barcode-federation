// Package crypto implements FROST (Flexible Round-Optimized Schnorr Threshold
// Signatures) for Ed25519 per RFC 9591 and FBS Appendix B.
//
// This implementation uses a trusted dealer model for key share generation,
// which is appropriate for FBS per-session ephemeral quorum groups where the
// Quorum Coordinator acts as the dealer (Appendix B.1).
//
// The protocol produces a threshold Ed25519-compatible signature that can be
// verified against the group public key.
package crypto

import (
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"fmt"
	"math/big"

	"filippo.io/edwards25519"
)

// edL is the Ed25519 group order L = 2^252 + 27742317777372353535851937790883648493
var edL, _ = new(big.Int).SetString("7237005577332262213973186563042994240857116359379907606001950938285454250989", 10)

// FROSTKeyShare represents a single participant's share of the group secret.
type FROSTKeyShare struct {
	ID          int    // Participant index (1-based)
	SecretShare []byte // 32-byte scalar (little-endian)
	PublicShare []byte // 32-byte compressed Edwards point
	GroupPublic []byte // 32-byte compressed Edwards point (group verification key)
}

// FROSTGroupSetup contains the full group configuration from DKG.
type FROSTGroupSetup struct {
	Threshold       int
	NumParticipants int
	GroupPublic     []byte           // 32-byte group public key
	Shares          []FROSTKeyShare
}

// FROSTNonceCommitment is Round 1 output from a participant.
type FROSTNonceCommitment struct {
	ParticipantID int
	HidingNonce   []byte // 32-byte scalar (secret)
	BindingNonce  []byte // 32-byte scalar (secret)
	HidingCommit  []byte // 32-byte point (public)
	BindingCommit []byte // 32-byte point (public)
}

// FROSTNoncePublic is the public portion shared with the coordinator.
type FROSTNoncePublic struct {
	ParticipantID int
	HidingCommit  []byte // 32-byte point
	BindingCommit []byte // 32-byte point
}

// FROSTPartialSig is Round 2 output from a participant.
type FROSTPartialSig struct {
	ParticipantID int
	Z             []byte // 32-byte scalar
}

// FROSTDealerGenerate performs trusted dealer key generation for a FROST group.
// It generates a random polynomial of degree (threshold-1), evaluates it at
// each participant's index to produce secret shares, and derives the group
// public key and per-participant public verification shares.
func FROSTDealerGenerate(threshold, numParticipants int) (*FROSTGroupSetup, error) {
	if threshold < 2 {
		return nil, errors.New("frost: threshold must be at least 2")
	}
	if numParticipants < threshold {
		return nil, errors.New("frost: numParticipants must be >= threshold")
	}

	// Generate random polynomial coefficients a_0, a_1, ..., a_{t-1}
	coefficients := make([]*big.Int, threshold)
	for i := range coefficients {
		coefficients[i] = randomBigScalar()
	}
	groupSecret := coefficients[0] // a_0

	// Group public key Y = a_0 * G
	groupPub := bigScalarBaseMult(groupSecret)

	// Per-participant shares: s_i = f(i) mod L
	shares := make([]FROSTKeyShare, numParticipants)
	for i := 0; i < numParticipants; i++ {
		id := i + 1
		x := big.NewInt(int64(id))
		shareVal := evaluatePoly(coefficients, x)

		sharePub := bigScalarBaseMult(shareVal)

		shares[i] = FROSTKeyShare{
			ID:          id,
			SecretShare: bigToLE32(shareVal),
			PublicShare: sharePub,
			GroupPublic: groupPub,
		}
	}

	return &FROSTGroupSetup{
		Threshold:       threshold,
		NumParticipants: numParticipants,
		GroupPublic:     groupPub,
		Shares:          shares,
	}, nil
}

// FROSTRound1 generates a participant's nonce pair for a signing session.
// The returned nonce secrets MUST NOT be reused across sessions (Appendix B.3).
func FROSTRound1(participantID int) (*FROSTNonceCommitment, error) {
	dScalar := randomBigScalar()
	eScalar := randomBigScalar()

	dCommit := bigScalarBaseMult(dScalar)
	eCommit := bigScalarBaseMult(eScalar)

	return &FROSTNonceCommitment{
		ParticipantID: participantID,
		HidingNonce:   bigToLE32(dScalar),
		BindingNonce:  bigToLE32(eScalar),
		HidingCommit:  dCommit,
		BindingCommit: eCommit,
	}, nil
}

// NoncePublic extracts the public portion of a nonce commitment.
func (nc *FROSTNonceCommitment) NoncePublic() FROSTNoncePublic {
	return FROSTNoncePublic{
		ParticipantID: nc.ParticipantID,
		HidingCommit:  nc.HidingCommit,
		BindingCommit: nc.BindingCommit,
	}
}

// FROSTRound2 produces a participant's partial signature share.
//
// Parameters:
//   - share: this participant's key share
//   - nonce: this participant's nonce commitment from Round 1
//   - message: the canonical attestation object bytes to sign
//   - allCommitments: all participating signers' public nonce commitments
//   - signerIDs: IDs of all participating signers (must include share.ID)
func FROSTRound2(
	share *FROSTKeyShare,
	nonce *FROSTNonceCommitment,
	message []byte,
	allCommitments []FROSTNoncePublic,
	signerIDs []int,
) (*FROSTPartialSig, error) {
	if nonce.ParticipantID != share.ID {
		return nil, fmt.Errorf("frost: nonce ID %d != share ID %d", nonce.ParticipantID, share.ID)
	}

	// Compute binding factor rho_i = H("FROST-bind" || i || msg || commitments)
	rho := bindingFactor(share.ID, message, allCommitments)

	// Compute group commitment R = sum(D_i + rho_i * E_i)
	R, err := groupCommitmentPoint(allCommitments, message)
	if err != nil {
		return nil, fmt.Errorf("frost: group commitment: %w", err)
	}

	// Compute challenge c = H(R || GroupPublic || msg) (Ed25519 Schnorr)
	c := schnorrChallenge(R.Bytes(), share.GroupPublic, message)

	// Compute Lagrange coefficient lambda_i
	lambda := lagrangeCoeff(share.ID, signerIDs)

	// z_i = d_i + rho_i * e_i + lambda_i * s_i * c  (mod L)
	d := le32ToBig(nonce.HidingNonce)
	e := le32ToBig(nonce.BindingNonce)
	s := le32ToBig(share.SecretShare)

	z := new(big.Int).Mul(rho, e)
	z.Mod(z, edL)
	z.Add(z, d)
	z.Mod(z, edL)

	lambdaSC := new(big.Int).Mul(lambda, s)
	lambdaSC.Mul(lambdaSC, c)
	lambdaSC.Mod(lambdaSC, edL)

	z.Add(z, lambdaSC)
	z.Mod(z, edL)

	return &FROSTPartialSig{
		ParticipantID: share.ID,
		Z:             bigToLE32(z),
	}, nil
}

// FROSTAggregate combines partial signature shares into the final group signature.
// Returns a 64-byte signature (R || z) compatible with Ed25519 verification.
func FROSTAggregate(
	message []byte,
	allCommitments []FROSTNoncePublic,
	partialSigs []FROSTPartialSig,
) ([]byte, error) {
	if len(partialSigs) == 0 {
		return nil, errors.New("frost: no partial signatures")
	}

	// Compute R
	R, err := groupCommitmentPoint(allCommitments, message)
	if err != nil {
		return nil, fmt.Errorf("frost: group commitment: %w", err)
	}

	// Aggregate z = sum(z_i) mod L
	z := new(big.Int)
	for _, ps := range partialSigs {
		zi := le32ToBig(ps.Z)
		z.Add(z, zi)
	}
	z.Mod(z, edL)

	// Encode as 64-byte Ed25519 signature: R (32 bytes) || z (32 bytes LE)
	sig := make([]byte, 64)
	copy(sig[:32], R.Bytes())
	copy(sig[32:], bigToLE32(z))

	return sig, nil
}

// FROSTVerify verifies a FROST threshold signature against the group public key.
// Uses standard Ed25519 verification since FROST produces compatible signatures.
func FROSTVerify(groupPublic []byte, message, signature []byte) bool {
	if len(signature) != 64 || len(groupPublic) != 32 {
		return false
	}

	// Decode R and z from signature
	R, err := new(edwards25519.Point).SetBytes(signature[:32])
	if err != nil {
		return false
	}

	z, err := new(edwards25519.Scalar).SetCanonicalBytes(signature[32:])
	if err != nil {
		return false
	}

	// Decode group public key
	A, err := new(edwards25519.Point).SetBytes(groupPublic)
	if err != nil {
		return false
	}

	// Compute challenge c = H(R || A || msg)
	cBig := schnorrChallenge(R.Bytes(), groupPublic, message)
	cScalar, err := new(edwards25519.Scalar).SetCanonicalBytes(bigToLE32(cBig))
	if err != nil {
		return false
	}

	// Verify: z * G == R + c * A
	// Equivalently: z * G - c * A == R
	lhs := new(edwards25519.Point).VarTimeDoubleScalarBaseMult(
		new(edwards25519.Scalar).Negate(cScalar), A, z,
	)

	return lhs.Equal(R) == 1
}

// --- Internal helpers ---

func randomBigScalar() *big.Int {
	for {
		b := make([]byte, 64)
		rand.Read(b)
		s := new(big.Int).SetBytes(b)
		s.Mod(s, edL)
		if s.Sign() > 0 {
			return s
		}
	}
}

func bigScalarBaseMult(s *big.Int) []byte {
	sc, err := new(edwards25519.Scalar).SetCanonicalBytes(bigToLE32(s))
	if err != nil {
		// Fallback: reduce mod L
		s.Mod(s, edL)
		sc, _ = new(edwards25519.Scalar).SetCanonicalBytes(bigToLE32(s))
	}
	pt := new(edwards25519.Point).ScalarBaseMult(sc)
	return pt.Bytes()
}

func evaluatePoly(coefficients []*big.Int, x *big.Int) *big.Int {
	result := new(big.Int).Set(coefficients[len(coefficients)-1])
	for j := len(coefficients) - 2; j >= 0; j-- {
		result.Mul(result, x)
		result.Add(result, coefficients[j])
		result.Mod(result, edL)
	}
	return result
}

func lagrangeCoeff(i int, signerIDs []int) *big.Int {
	xi := big.NewInt(int64(i))
	num := big.NewInt(1)
	den := big.NewInt(1)

	for _, j := range signerIDs {
		if j == i {
			continue
		}
		xj := big.NewInt(int64(j))
		num.Mul(num, xj)
		num.Mod(num, edL)

		diff := new(big.Int).Sub(xj, xi)
		diff.Mod(diff, edL)
		den.Mul(den, diff)
		den.Mod(den, edL)
	}

	denInv := new(big.Int).ModInverse(den, edL)
	if denInv == nil {
		return big.NewInt(0)
	}
	result := new(big.Int).Mul(num, denInv)
	result.Mod(result, edL)
	return result
}

func bindingFactor(participantID int, message []byte, commitments []FROSTNoncePublic) *big.Int {
	h := sha512.New()
	fmt.Fprintf(h, "FROST-binding-%d-", participantID)
	h.Write(message)
	for _, c := range commitments {
		h.Write(c.HidingCommit)
		h.Write(c.BindingCommit)
	}
	digest := h.Sum(nil)
	rho := new(big.Int).SetBytes(digest)
	rho.Mod(rho, edL)
	return rho
}

func groupCommitmentPoint(commitments []FROSTNoncePublic, message []byte) (*edwards25519.Point, error) {
	result := edwards25519.NewIdentityPoint()

	for _, c := range commitments {
		Di, err := new(edwards25519.Point).SetBytes(c.HidingCommit)
		if err != nil {
			return nil, fmt.Errorf("invalid hiding commit for participant %d: %w", c.ParticipantID, err)
		}

		Ei, err := new(edwards25519.Point).SetBytes(c.BindingCommit)
		if err != nil {
			return nil, fmt.Errorf("invalid binding commit for participant %d: %w", c.ParticipantID, err)
		}

		rho := bindingFactor(c.ParticipantID, message, commitments)
		rhoScalar, err := new(edwards25519.Scalar).SetCanonicalBytes(bigToLE32(rho))
		if err != nil {
			return nil, fmt.Errorf("binding factor scalar: %w", err)
		}

		// rho_i * E_i
		rhoEi := new(edwards25519.Point).ScalarMult(rhoScalar, Ei)

		// D_i + rho_i * E_i
		term := new(edwards25519.Point).Add(Di, rhoEi)

		result.Add(result, term)
	}

	return result, nil
}

func schnorrChallenge(R, publicKey, message []byte) *big.Int {
	h := sha512.New()
	h.Write(R)
	h.Write(publicKey)
	h.Write(message)
	digest := h.Sum(nil)
	c := new(big.Int).SetBytes(digest)
	c.Mod(c, edL)
	return c
}

// bigToLE32 converts a big.Int to a 32-byte little-endian representation.
func bigToLE32(n *big.Int) []byte {
	b := n.Bytes() // big-endian
	le := make([]byte, 32)
	for i, v := range b {
		le[len(b)-1-i] = v
	}
	return le
}

// le32ToBig converts 32 little-endian bytes to a big.Int.
func le32ToBig(b []byte) *big.Int {
	if len(b) < 32 {
		padded := make([]byte, 32)
		copy(padded, b)
		b = padded
	}
	be := make([]byte, 32)
	for i := 0; i < 32; i++ {
		be[31-i] = b[i]
	}
	return new(big.Int).SetBytes(be)
}
