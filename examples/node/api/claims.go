package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleSubmitClaim(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())

	var sub model.ClaimSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if sub.NamespaceType != "gs1" && sub.NamespaceType != "self" {
		BadRequest(w, "namespaceType must be 'gs1' or 'self'")
		return
	}

	// Validate self-sovereign evidence: verify FNI derivation from public key
	if sub.NamespaceType == "self" {
		if err := validateSSNEvidence(sub.Scope, sub.Evidence); err != nil {
			BadRequest(w, "SSN evidence validation failed: "+err.Error())
			return
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	localID := extractLocalID(actor.ID)
	claimID := s.NodeBaseURL() + "/api/v1/actors/" + localID + "/claims/" + uuid.New().String()

	claim := &model.NamespaceClaim{
		FBS:           "1.0",
		Type:          "NamespaceClaim",
		ID:            claimID,
		ClaimedBy:     actor.ID,
		NamespaceType: sub.NamespaceType,
		Scope:         sub.Scope,
		IssuedAt:      now,
		ExpiresAt:     sub.ExpiresAt,
		Evidence:      sub.Evidence,
	}

	actorKey, err := getActorPrivateKey(actor)
	if err != nil {
		InternalError(w, err)
		return
	}
	sig, err := fbscrypto.SignRecord(claim, actorKey)
	if err != nil {
		InternalError(w, err)
		return
	}
	claim.Signature = sig

	if err := s.Store.CreateClaim(claim); err != nil {
		InternalError(w, err)
		return
	}

	if s.Dispatcher != nil {
		go s.Dispatcher.DispatchClaimPublish(claim)
	}

	JSON(w, http.StatusCreated, claim)
}

func (s *Server) handleListClaims(w http.ResponseWriter, r *http.Request) {
	actorLocalID := chi.URLParam(r, "actorID")
	actorID := "fbs://" + s.Config.Node.Host + "/" + actorLocalID

	claims, err := s.Store.ListClaimsByActor(actorID)
	if err != nil {
		InternalError(w, err)
		return
	}

	JSON(w, http.StatusOK, claims)
}

// validateSSNEvidence verifies self-sovereign namespace claim evidence per Section 4.2.3.3:
// 1. Derive FNI from evidence.namespacePublicKey
// 2. Confirm derived FNI equals scope.fni and evidence.derivedFni
func validateSSNEvidence(scopeRaw, evidenceRaw json.RawMessage) error {
	var scope model.SelfScope
	if err := json.Unmarshal(scopeRaw, &scope); err != nil {
		return fmt.Errorf("invalid self scope: %w", err)
	}
	if !fbscrypto.ValidateFNISyntax(scope.FNI) {
		return fmt.Errorf("scope.fni is not a valid FNI: %s", scope.FNI)
	}

	var evidence model.SelfSovereignEvidence
	if err := json.Unmarshal(evidenceRaw, &evidence); err != nil {
		return fmt.Errorf("invalid self-sovereign evidence: %w", err)
	}
	if evidence.Type != "SelfSovereign" {
		return fmt.Errorf("evidence.type must be 'SelfSovereign', got '%s'", evidence.Type)
	}

	// Extract the public key from evidence
	var keyObj struct {
		Algorithm    string `json:"algorithm"`
		PublicKeyPem string `json:"publicKeyPem"`
	}
	if err := json.Unmarshal(evidence.NamespacePublicKey, &keyObj); err != nil {
		return fmt.Errorf("invalid namespacePublicKey: %w", err)
	}
	if keyObj.Algorithm != "Ed25519" {
		return fmt.Errorf("namespacePublicKey.algorithm must be 'Ed25519'")
	}

	pub, err := fbscrypto.DecodePublicKeyPEM(keyObj.PublicKeyPem)
	if err != nil {
		return fmt.Errorf("decode namespace public key: %w", err)
	}

	// Derive FNI from the public key and verify it matches
	derivedFNI, err := fbscrypto.DeriveFNI(pub)
	if err != nil {
		return fmt.Errorf("derive FNI: %w", err)
	}

	if !fbscrypto.TimingSafeEqual(derivedFNI, scope.FNI) {
		return fmt.Errorf("derived FNI (%s) does not match scope.fni (%s)", derivedFNI, scope.FNI)
	}
	if evidence.DerivedFNI != "" && !fbscrypto.TimingSafeEqual(derivedFNI, evidence.DerivedFNI) {
		return fmt.Errorf("derived FNI (%s) does not match evidence.derivedFni (%s)", derivedFNI, evidence.DerivedFNI)
	}

	return nil
}

func extractLocalID(aid string) string {
	if !strings.HasPrefix(aid, "fbs://") {
		return aid
	}
	rest := aid[6:]
	idx := strings.Index(rest, "/")
	if idx < 0 {
		return rest
	}
	return rest[idx+1:]
}
