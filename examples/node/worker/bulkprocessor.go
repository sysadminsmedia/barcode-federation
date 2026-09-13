package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/api"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
	"github.com/google/uuid"
)

func StartBulkProcessor(ctx context.Context, s *store.Store, nodeBaseURL string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processBulkJobs(s, nodeBaseURL)
		}
	}
}

func processBulkJobs(s *store.Store, nodeBaseURL string) {
	jobs, err := s.GetPendingBulkJobs()
	if err != nil {
		slog.Error("failed to get pending bulk jobs", "error", err)
		return
	}

	for _, job := range jobs {
		records, ok := api.BulkRecordStoreGet(job.ID)
		if !ok {
			s.UpdateBulkJob(job.ID, 0, 0, "failed")
			continue
		}

		s.UpdateBulkJob(job.ID, 0, 0, "processing")
		processed, failed := 0, 0

		actor, err := s.GetActor(job.ActorID)
		if err != nil {
			slog.Error("bulk job actor not found", "error", err)
			s.UpdateBulkJob(job.ID, 0, job.Total, "failed")
			continue
		}

		for _, rawRec := range records {
			var sub model.RecordSubmission
			if err := json.Unmarshal(rawRec, &sub); err != nil {
				failed++
				continue
			}

			if !model.ValidSymbologies[sub.Symbology] || sub.Value == "" || sub.MetadataSchema == "" {
				failed++
				continue
			}

			now := time.Now().UTC().Format(time.RFC3339)
			cbi := sub.Symbology + ":" + sub.Value
			rec := &model.BarcodeRecord{
				FBS:            "1.0",
				Type:           "BarcodeRecord",
				ID:             nodeBaseURL + "/api/v1/records/" + uuid.New().String(),
				CBI:            cbi,
				Symbology:      sub.Symbology,
				Value:          sub.Value,
				Status:         "active",
				SubmittedBy:    actor.ID,
				SubmittedAt:    now,
				UpdatedAt:      now,
				OriginNode:     nodeBaseURL,
				MetadataSchema: sub.MetadataSchema,
				Metadata:       sub.Metadata,
				RevisionHistory: []model.RevisionEntry{
					{RevisionID: uuid.New().String(), At: now, By: actor.ID, ChangeType: "create"},
				},
				Signature: "bulk-unsigned", // TODO: sign with actor key
			}

			if err := s.CreateRecord(rec); err != nil {
				failed++
			} else {
				processed++
			}
		}

		status := "completed"
		if failed > 0 && processed == 0 {
			status = "failed"
		}
		s.UpdateBulkJob(job.ID, processed, failed, status)
		api.BulkRecordStoreDelete(job.ID)
		slog.Info("bulk job completed", "id", job.ID, "processed", processed, "failed", failed)
	}
}
