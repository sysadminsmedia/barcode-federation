package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) SaveSuccession(decl *model.KeySuccessionDeclaration) error {
	rawJSON, err := json.Marshal(decl)
	if err != nil {
		return fmt.Errorf("marshal succession: %w", err)
	}
	_, err = s.DB.Exec(`INSERT OR REPLACE INTO successions (id, old_fni, successor_fni, declared_at, expires_at, document_json)
		VALUES (?, ?, ?, ?, ?, ?)`,
		decl.ID, decl.OldFNI, decl.SuccessorFNI, decl.DeclaredAt, decl.ExpiresAt, string(rawJSON))
	if err != nil {
		return fmt.Errorf("save succession: %w", err)
	}
	return nil
}

func (s *Store) GetSuccession(id string) (*model.KeySuccessionDeclaration, error) {
	var row model.SuccessionRow
	err := s.DB.Get(&row, "SELECT * FROM successions WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("succession not found")
		}
		return nil, fmt.Errorf("get succession: %w", err)
	}
	var decl model.KeySuccessionDeclaration
	if err := json.Unmarshal([]byte(row.DocumentJSON), &decl); err != nil {
		return nil, fmt.Errorf("unmarshal succession: %w", err)
	}
	return &decl, nil
}

func (s *Store) GetSuccessionByOldFNI(oldFNI string) (*model.KeySuccessionDeclaration, error) {
	var row model.SuccessionRow
	err := s.DB.Get(&row, "SELECT * FROM successions WHERE old_fni = ? ORDER BY declared_at DESC LIMIT 1", oldFNI)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get succession by fni: %w", err)
	}
	var decl model.KeySuccessionDeclaration
	if err := json.Unmarshal([]byte(row.DocumentJSON), &decl); err != nil {
		return nil, err
	}
	return &decl, nil
}

func (s *Store) ListSuccessionsSince(since string) ([]model.KeySuccessionDeclaration, error) {
	var rows []model.SuccessionRow
	err := s.DB.Select(&rows, "SELECT * FROM successions WHERE declared_at >= ? ORDER BY declared_at ASC", since)
	if err != nil {
		return nil, fmt.Errorf("list successions: %w", err)
	}
	decls := make([]model.KeySuccessionDeclaration, 0, len(rows))
	for _, row := range rows {
		var d model.KeySuccessionDeclaration
		if err := json.Unmarshal([]byte(row.DocumentJSON), &d); err != nil {
			continue
		}
		decls = append(decls, d)
	}
	return decls, nil
}

// Pending verification queue

type PendingVerification struct {
	ID                 int64  `db:"id"`
	RetractMessageJSON string `db:"retract_message_json"`
	SuccessionURL      string `db:"succession_url"`
	NextRetryAt        string `db:"next_retry_at"`
	AttemptCount       int    `db:"attempt_count"`
	CreatedAt          string `db:"created_at"`
}

func (s *Store) AddPendingVerification(messageJSON, successionURL string) error {
	_, err := s.DB.Exec(`INSERT INTO pending_verifications (retract_message_json, succession_url, next_retry_at)
		VALUES (?, ?, datetime('now', '+5 minutes'))`, messageJSON, successionURL)
	return err
}

func (s *Store) GetPendingVerifications(limit int) ([]PendingVerification, error) {
	var rows []PendingVerification
	err := s.DB.Select(&rows,
		"SELECT * FROM pending_verifications WHERE next_retry_at <= datetime('now') ORDER BY next_retry_at LIMIT ?", limit)
	return rows, err
}

func (s *Store) UpdatePendingVerification(id int64, nextRetry string, attempts int) error {
	_, err := s.DB.Exec("UPDATE pending_verifications SET next_retry_at=?, attempt_count=? WHERE id=?",
		nextRetry, attempts, id)
	return err
}

func (s *Store) RemovePendingVerification(id int64) error {
	_, err := s.DB.Exec("DELETE FROM pending_verifications WHERE id = ?", id)
	return err
}
