package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) CreateBulkJob(job *model.BulkImportJob) error {
	_, err := s.DB.Exec(`INSERT INTO bulk_jobs (id, actor_id, status, total)
		VALUES (?, ?, ?, ?)`, job.ID, job.ActorID, job.Status, job.Total)
	if err != nil {
		return fmt.Errorf("create bulk job: %w", err)
	}
	return nil
}

func (s *Store) GetBulkJob(id string) (*model.BulkImportJob, error) {
	var job model.BulkImportJob
	err := s.DB.Get(&job, "SELECT * FROM bulk_jobs WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("get bulk job: %w", err)
	}
	return &job, nil
}

func (s *Store) UpdateBulkJob(id string, processed, failed int, status string) error {
	var completedAt *string
	if status == "completed" || status == "failed" {
		t := time.Now().UTC().Format(time.RFC3339)
		completedAt = &t
	}
	_, err := s.DB.Exec("UPDATE bulk_jobs SET processed=?, failed=?, status=?, completed_at=? WHERE id=?",
		processed, failed, status, completedAt, id)
	return err
}

func (s *Store) GetPendingBulkJobs() ([]model.BulkImportJob, error) {
	var jobs []model.BulkImportJob
	err := s.DB.Select(&jobs, "SELECT * FROM bulk_jobs WHERE status = 'pending' ORDER BY created_at ASC LIMIT 10")
	if err != nil {
		return nil, fmt.Errorf("get pending jobs: %w", err)
	}
	return jobs, nil
}
