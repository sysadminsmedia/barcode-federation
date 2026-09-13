package api

import (
	"encoding/json"
	"net/http"
	"time"

	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleCustodyTransfer(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())
	if actor == nil {
		Unauthorized(w)
		return
	}

	var req struct {
		RecordID             string   `json:"recordId"`
		ToActor              string   `json:"toActor"`
		Condition            string   `json:"condition"`
		DelegateCapabilities []string `json:"delegateCapabilities"`
		ExpiresAt            string   `json:"expiresAt,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if req.RecordID == "" || req.ToActor == "" {
		BadRequest(w, "recordId and toActor are required")
		return
	}
	if req.Condition == "" {
		req.Condition = "accepted"
	}

	// Verify the record exists
	rec, err := s.Store.GetRecord(req.RecordID)
	if err != nil {
		NotFound(w, "record not found")
		return
	}

	// Verify actor is the current custodian
	currentCustodian, err := s.Store.GetCurrentCustodian(rec.ID, rec.SubmittedBy)
	if err != nil {
		InternalError(w, err)
		return
	}
	if currentCustodian != actor.ID {
		Forbidden(w, "record:delegate (not current custodian)")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	actorKey, err := getActorPrivateKey(actor)
	if err != nil {
		InternalError(w, err)
		return
	}

	notice := &model.CustodyTransferNotice{
		Type:                 "CustodyTransferNotice",
		RecordID:             rec.ID,
		RecordCBI:            rec.CBI,
		FromActor:            actor.ID,
		ToActor:              req.ToActor,
		TransferredAt:        now,
		Condition:            req.Condition,
		DelegateCapabilities: req.DelegateCapabilities,
		ExpiresAt:            req.ExpiresAt,
	}

	// Sign the notice
	sig, err := fbscrypto.SignRecord(notice, actorKey)
	if err != nil {
		InternalError(w, err)
		return
	}
	notice.FromSignature = sig

	// Store the custody transfer
	_, err = s.Store.CreateCustodyTransfer(rec.ID, notice)
	if err != nil {
		InternalError(w, err)
		return
	}

	// Dispatch via federation
	if s.Dispatcher != nil {
		go s.Dispatcher.DispatchActivity("CustodyTransfer", notice)
	}

	JSON(w, http.StatusCreated, notice)
}

func (s *Server) handleCustodyAccept(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())
	if actor == nil {
		Unauthorized(w)
		return
	}

	var req struct {
		RecordID         string `json:"recordId"`
		TransferNoticeID string `json:"transferNoticeId"`
		Condition        string `json:"condition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	// Sign acceptance
	actorKey, err := getActorPrivateKey(actor)
	if err != nil {
		InternalError(w, err)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	acceptance := &model.CustodyAcceptance{
		Type:             "CustodyAcceptance",
		TransferNoticeID: req.TransferNoticeID,
		RecordID:         req.RecordID,
		AcceptedBy:       actor.ID,
		AcceptedAt:       now,
		Condition:        req.Condition,
	}

	sig, err := fbscrypto.SignRecord(acceptance, actorKey)
	if err != nil {
		InternalError(w, err)
		return
	}
	acceptance.ToSignature = sig

	// Record the acceptance
	if err := s.Store.AcceptCustodyTransfer(req.RecordID, actor.ID, sig); err != nil {
		InternalError(w, err)
		return
	}

	// Dispatch via federation
	if s.Dispatcher != nil {
		go s.Dispatcher.DispatchActivity("CustodyAccept", acceptance)
	}

	JSON(w, http.StatusOK, acceptance)
}

func (s *Server) handleGetCustodyChain(w http.ResponseWriter, r *http.Request) {
	recordID := chi.URLParam(r, "recordID")
	fullID := s.NodeBaseURL() + "/api/v1/records/" + recordID

	chain, err := s.Store.GetCustodyChain(fullID)
	if err != nil {
		chain, err = s.Store.GetCustodyChain(recordID)
		if err != nil {
			NotFound(w, "no custody chain found")
			return
		}
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"recordId":     recordID,
		"custodyChain": chain,
		"chainLength":  len(chain),
	})
}

func (s *Server) handleGetRelatedRecords(w http.ResponseWriter, r *http.Request) {
	symbology := chi.URLParam(r, "symbology")
	value := chi.URLParam(r, "value")
	cbi := symbology + ":" + value

	records, err := s.Store.FindRecordsRelatedTo(cbi)
	if err != nil {
		InternalError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"cbi":     cbi,
		"records": records,
		"total":   len(records),
	})
}

// Helper used by handleSubmitRecord to also save related records for indexing
func (s *Server) saveRelatedRecordsIndex(recordID string, related []model.RelatedRecord) {
	if len(related) > 0 {
		s.Store.SaveRelatedRecords(recordID, related)
	}
}

// Needed for dispatcher - add DispatchActivity to the interface
var _ = uuid.New // suppress unused import
