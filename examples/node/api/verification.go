package api

import (
	"encoding/json"
	"net/http"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/quorum"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleStartVerification(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())
	if actor == nil {
		Unauthorized(w)
		return
	}

	var req model.VerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if len(req.ClaimedPrefixes) == 0 {
		BadRequest(w, "claimedPrefixes is required")
		return
	}

	// If coordinator is available, start a real quorum session
	if s.QuorumCoordinator != nil {
		evidenceJSON, err := json.Marshal(req.EvidencePackage)
		if err != nil {
			BadRequest(w, "invalid evidence package")
			return
		}

		// Get quorum-eligible peer nodes
		peers, err := s.Store.ListFederatedPeers()
		if err != nil {
			InternalError(w, err)
			return
		}

		var verifierNodes []string
		for _, p := range peers {
			// Skip our own node (can't self-attest)
			if p.NodeID == s.NodeBaseURL() {
				continue
			}
			verifierNodes = append(verifierNodes, p.NodeID)
		}

		// Need at least 3 verifier nodes per Section 4.6.1
		if len(verifierNodes) < 3 {
			// Fall back to simulating verifiers for the reference implementation
			verifierNodes = []string{
				s.NodeBaseURL() + "/verifier-sim-1",
				s.NodeBaseURL() + "/verifier-sim-2",
				s.NodeBaseURL() + "/verifier-sim-3",
				s.NodeBaseURL() + "/verifier-sim-4",
				s.NodeBaseURL() + "/verifier-sim-5",
			}
		}

		session, err := s.QuorumCoordinator.StartSession(
			actor.ID,
			"", // subject key fingerprint filled later
			req.ClaimedPrefixes,
			evidenceJSON,
			verifierNodes,
		)
		if err != nil {
			InternalError(w, err)
			return
		}

		// Simulate verifier evaluation for the reference implementation
		go s.simulateVerifierEvaluation(session, evidenceJSON)

		JSON(w, http.StatusAccepted, map[string]interface{}{
			"sessionId": session.ID,
			"status":    session.Status,
			"threshold": session.Threshold,
			"quorumSize": session.QuorumSize,
			"verifiers": session.VerifierNodes,
		})
		return
	}

	// Fallback: no coordinator configured
	BadRequest(w, "quorum verification not available: coordinator not configured")
}

// simulateVerifierEvaluation runs the verifier evaluation for all nodes.
// In a real federated network, this would send evidence to remote verifier nodes
// via federation messages and collect their commitments asynchronously.
func (s *Server) simulateVerifierEvaluation(session *quorum.Session, evidenceJSON []byte) {
	evidence := &quorum.VerificationEvidence{
		SessionID:       session.ID,
		ActorID:         session.ActorID,
		ClaimedPrefixes: session.ClaimedPrefixes,
		EvidenceHash:    session.EvidenceHash,
		EvidencePackage: evidenceJSON,
	}

	for _, nodeID := range session.VerifierNodes {
		verifier := quorum.NewVerifierNode(nodeID)
		commitment, err := verifier.EvaluateEvidence(evidence)
		if err != nil {
			continue
		}
		s.QuorumCoordinator.ProcessCommitment(session.ID, *commitment)
	}
}

func (s *Server) handleVerificationStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	if s.QuorumCoordinator != nil {
		session, ok := s.QuorumCoordinator.GetSession(sessionID)
		if ok {
			resp := map[string]interface{}{
				"sessionId":  session.ID,
				"status":     session.Status,
				"actorId":    session.ActorID,
				"threshold":  session.Threshold,
				"quorumSize": session.QuorumSize,
				"createdAt":  session.CreatedAt,
				"updatedAt":  session.UpdatedAt,
			}

			commitments := session.AcceptedCommitments()
			resp["acceptedVerifiers"] = len(commitments)

			if session.Status == quorum.StatusCompleted && len(session.BadgeJSON) > 0 {
				var badge json.RawMessage = session.BadgeJSON
				resp["badge"] = badge
			}
			if session.Error != "" {
				resp["error"] = session.Error
			}

			JSON(w, http.StatusOK, resp)
			return
		}
	}

	// Fallback: check DB for legacy sessions
	var session model.VerificationSession
	err := s.Store.DB.Get(&session, "SELECT * FROM verification_sessions WHERE id = ?", sessionID)
	if err != nil {
		NotFound(w, "verification session not found")
		return
	}

	JSON(w, http.StatusOK, session)
}
