package api

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var _ = fmt.Errorf // ensure fmt is used

func (s *Server) handleSearchBarcodes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	symbology := r.URL.Query().Get("symbology")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	filters := store.RecordFilters{
		Query:             query,
		Symbology:         symbology,
		Status:            r.URL.Query().Get("status"),
		SubmittedBy:       r.URL.Query().Get("submittedBy"),
		OriginNode:        r.URL.Query().Get("originNode"),
		MinVerification:   r.URL.Query().Get("minLevel"),
		MetadataSchemaURL: r.URL.Query().Get("metadataSchema"),
	}

	records, total, err := s.Store.SearchRecordsFiltered(filters, limit, offset)
	if err != nil {
		InternalError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"total":   total,
		"page":    page,
		"records": records,
	})
}

func (s *Server) handleLookupBarcode(w http.ResponseWriter, r *http.Request) {
	symbology := chi.URLParam(r, "symbology")
	value := chi.URLParam(r, "value")

	records, err := s.Store.GetRecordByCBI(symbology, value)
	if err != nil {
		InternalError(w, err)
		return
	}
	if len(records) == 0 {
		NotFound(w, "no records found for this barcode")
		return
	}

	// Apply optional filters to the result set
	filtered := records
	if status := r.URL.Query().Get("status"); status != "" {
		filtered = filterByStatus(filtered, status)
	}
	if minLevel := r.URL.Query().Get("minLevel"); minLevel != "" {
		if lvl, err := strconv.Atoi(minLevel); err == nil {
			filtered = filterByMinLevel(filtered, lvl)
		}
	}
	if submittedBy := r.URL.Query().Get("submittedBy"); submittedBy != "" {
		filtered = filterBySubmitter(filtered, submittedBy)
	}

	if len(filtered) == 0 {
		NotFound(w, "no records match the given filters")
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"best":         filtered[0],
		"records":      filtered,
		"totalUnfiltered": len(records),
	})
}

func filterByStatus(records []model.BarcodeRecord, status string) []model.BarcodeRecord {
	var out []model.BarcodeRecord
	for _, r := range records {
		if r.Status == status {
			out = append(out, r)
		}
	}
	return out
}

func filterByMinLevel(records []model.BarcodeRecord, minLevel int) []model.BarcodeRecord {
	// This requires checking the record's verification context.
	// For now, we filter based on whether a namespace claim is present (level > 0)
	// or not (level -1). A full implementation would check the badge level.
	var out []model.BarcodeRecord
	for _, r := range records {
		level := -1
		if r.NamespaceClaim != nil {
			level = 0 // has a claim but unknown verification level
		}
		if level >= minLevel {
			out = append(out, r)
		}
	}
	return out
}

func filterBySubmitter(records []model.BarcodeRecord, submittedBy string) []model.BarcodeRecord {
	var out []model.BarcodeRecord
	for _, r := range records {
		if r.SubmittedBy == submittedBy {
			out = append(out, r)
		}
	}
	return out
}

func (s *Server) handleGetRecord(w http.ResponseWriter, r *http.Request) {
	recordID := chi.URLParam(r, "recordID")
	// The record ID is a URL, so it may be URL-encoded.
	// Try both the raw param and reconstructing the full URL.
	rec, err := s.Store.GetRecord(recordID)
	if err != nil {
		// Try as a path suffix
		fullID := s.NodeBaseURL() + "/api/v1/records/" + recordID
		rec, err = s.Store.GetRecord(fullID)
		if err != nil {
			NotFound(w, "record not found")
			return
		}
	}
	JSON(w, http.StatusOK, rec)
}

func (s *Server) handleSubmitRecord(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())

	var sub model.RecordSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if !model.ValidSymbologies[sub.Symbology] {
		BadRequest(w, "unsupported symbology: "+sub.Symbology)
		return
	}
	if sub.Value == "" {
		BadRequest(w, "value is required")
		return
	}
	if sub.MetadataSchema == "" || !strings.HasPrefix(sub.MetadataSchema, "https://") {
		BadRequest(w, "metadataSchema must be an HTTPS URL")
		return
	}
	if len(sub.Metadata) == 0 {
		BadRequest(w, "metadata is required")
		return
	}

	cbi := sub.Symbology + ":" + sub.Value

	// Check for existing active record from this actor
	existing, err := s.Store.GetActiveRecordByCBIAndActor(cbi, actor.ID)
	if err != nil {
		InternalError(w, err)
		return
	}
	if existing != nil {
		BadRequest(w, "active record already exists for this CBI from this actor; use PUT to update")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	recordID := s.NodeBaseURL() + "/api/v1/records/" + uuid.New().String()

	rec := &model.BarcodeRecord{
		FBS:                  "1.0",
		Type:                 "BarcodeRecord",
		ID:                   recordID,
		CBI:                  cbi,
		Symbology:            sub.Symbology,
		Value:                sub.Value,
		Status:               "active",
		SubmittedBy:          actor.ID,
		SubmittedAt:          now,
		UpdatedAt:            now,
		ExpiresAt:            sub.ExpiresAt,
		OriginNode:           s.NodeBaseURL(),
		NamespaceClaim:       sub.NamespaceClaim,
		MetadataSchema:       sub.MetadataSchema,
		MetadataSchemaDigest: sub.MetadataSchemaDigest,
		Metadata:             sub.Metadata,
		GS1CrossRef:          sub.GS1CrossRef,
		LifecycleSchema:      sub.LifecycleSchema,
		RelatedRecords:       sub.RelatedRecords,
		RevisionHistory: []model.RevisionEntry{
			{
				RevisionID:                uuid.New().String(),
				At:                        now,
				By:                        actor.ID,
				ChangeType:                "create",
				PreviousRevisionSignature: nil,
			},
		},
	}

	// Sign the record
	actorKey, err := getActorPrivateKey(actor)
	if err != nil {
		InternalError(w, err)
		return
	}
	sig, err := fbscrypto.SignRecord(rec, actorKey)
	if err != nil {
		InternalError(w, err)
		return
	}
	rec.Signature = sig

	if err := s.Store.CreateRecord(rec); err != nil {
		InternalError(w, err)
		return
	}

	// Index related records for cross-CBI queries (Section 4.2.8)
	s.saveRelatedRecordsIndex(rec.ID, rec.RelatedRecords)

	// Dispatch federation (async)
	if s.Dispatcher != nil {
		go s.Dispatcher.DispatchPublish(rec)
	}

	JSON(w, http.StatusCreated, rec)
}

func (s *Server) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())
	recordID := chi.URLParam(r, "recordID")

	fullID := s.NodeBaseURL() + "/api/v1/records/" + recordID
	rec, err := s.Store.GetRecord(fullID)
	if err != nil {
		rec, err = s.Store.GetRecord(recordID)
		if err != nil {
			NotFound(w, "record not found")
			return
		}
	}

	if rec.SubmittedBy != actor.ID {
		// Check if actor is a delegate via custody transfer (Section 4.2.7.3)
		isDelegate, err := s.Store.IsDelegate(rec.ID, actor.ID)
		if err != nil || !isDelegate {
			Forbidden(w, "record:update:own (not owner or delegate)")
			return
		}
	}
	if rec.Status != "active" {
		BadRequest(w, "cannot update a "+rec.Status+" record")
		return
	}

	var update struct {
		Metadata             json.RawMessage `json:"metadata"`
		MetadataSchema       string          `json:"metadataSchema,omitempty"`
		MetadataSchemaDigest string          `json:"metadataSchemaDigest,omitempty"`
		Summary              string          `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}
	if update.Summary == "" {
		BadRequest(w, "summary is required for updates")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	oldSig := rec.Signature

	if len(update.Metadata) > 0 {
		rec.Metadata = update.Metadata
	}
	if update.MetadataSchema != "" {
		rec.MetadataSchema = update.MetadataSchema
	}
	if update.MetadataSchemaDigest != "" {
		rec.MetadataSchemaDigest = update.MetadataSchemaDigest
	}
	rec.UpdatedAt = now

	rec.RevisionHistory = append(rec.RevisionHistory, model.RevisionEntry{
		RevisionID:                uuid.New().String(),
		At:                        now,
		By:                        actor.ID,
		ChangeType:                "update",
		Summary:                   update.Summary,
		PreviousRevisionSignature: &oldSig,
	})

	actorKey, err := getActorPrivateKey(actor)
	if err != nil {
		InternalError(w, err)
		return
	}
	sig, err := fbscrypto.SignRecord(rec, actorKey)
	if err != nil {
		InternalError(w, err)
		return
	}
	rec.Signature = sig

	if err := s.Store.UpdateRecord(rec); err != nil {
		InternalError(w, err)
		return
	}

	if s.Dispatcher != nil {
		go s.Dispatcher.DispatchPublish(rec)
	}

	JSON(w, http.StatusOK, rec)
}

func getActorPrivateKey(actor *model.Actor) (ed25519.PrivateKey, error) {
	if actor.PrivateKeyPem == nil || *actor.PrivateKeyPem == "" {
		return nil, errNoPrivateKey
	}
	return fbscrypto.LoadPrivateKeyFromPEM(*actor.PrivateKeyPem)
}

var errNoPrivateKey = fmt.Errorf("actor has no private key stored on this node")
