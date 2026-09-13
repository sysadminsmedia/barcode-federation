package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// bulkRecordStore is used by the bulk processor to access records later.
// In a production system this would use a separate staging table.
var (
	bulkRecordStore   = make(map[string][]json.RawMessage)
	bulkRecordStoreMu sync.Mutex
)

func BulkRecordStoreGet(jobID string) ([]json.RawMessage, bool) {
	bulkRecordStoreMu.Lock()
	defer bulkRecordStoreMu.Unlock()
	recs, ok := bulkRecordStore[jobID]
	return recs, ok
}

func BulkRecordStoreDelete(jobID string) {
	bulkRecordStoreMu.Lock()
	defer bulkRecordStoreMu.Unlock()
	delete(bulkRecordStore, jobID)
}

func (s *Server) handleBulkImport(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())

	var records []json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&records); err != nil {
		BadRequest(w, "expected JSON array of record objects")
		return
	}

	if len(records) == 0 {
		BadRequest(w, "empty records array")
		return
	}

	jobID := uuid.New().String()
	job := &model.BulkImportJob{
		ID:      jobID,
		ActorID: actor.ID,
		Status:  "pending",
		Total:   len(records),
	}

	if err := s.Store.CreateBulkJob(job); err != nil {
		InternalError(w, err)
		return
	}

	// Store records for async processing
	bulkRecordStoreMu.Lock()
	bulkRecordStore[jobID] = records
	bulkRecordStoreMu.Unlock()

	JSON(w, http.StatusAccepted, map[string]interface{}{
		"jobId":  jobID,
		"status": "pending",
		"total":  len(records),
	})
}

func (s *Server) handleBulkJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	job, err := s.Store.GetBulkJob(jobID)
	if err != nil {
		NotFound(w, "job not found")
		return
	}

	JSON(w, http.StatusOK, job)
}
