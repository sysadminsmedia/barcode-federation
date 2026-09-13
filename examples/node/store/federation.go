package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) SaveOutboxMessage(msg *model.FederationMessage) error {
	rawJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	_, err = s.DB.Exec(`INSERT INTO federation_outbox (id, activity_type, message_json, published_at)
		VALUES (?, ?, ?, ?)`, msg.ID, msg.Activity, string(rawJSON), msg.PublishedAt)
	if err != nil {
		return fmt.Errorf("save outbox message: %w", err)
	}
	return nil
}

func (s *Store) GetOutboxPage(page, pageSize int) ([]json.RawMessage, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var total int
	if err := s.DB.Get(&total, "SELECT COUNT(*) FROM federation_outbox"); err != nil {
		return nil, 0, fmt.Errorf("count outbox: %w", err)
	}

	var rows []model.OutboxRow
	err := s.DB.Select(&rows, "SELECT * FROM federation_outbox ORDER BY published_at DESC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get outbox page: %w", err)
	}

	items := make([]json.RawMessage, len(rows))
	for i, row := range rows {
		items[i] = json.RawMessage(row.MessageJSON)
	}
	return items, total, nil
}

func (s *Store) SaveInboxMessage(msg *model.FederationMessage) error {
	rawJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	_, err = s.DB.Exec(`INSERT OR IGNORE INTO federation_inbox (id, sender, activity_type, message_json)
		VALUES (?, ?, ?, ?)`, msg.ID, msg.Sender, msg.Activity, string(rawJSON))
	if err != nil {
		return fmt.Errorf("save inbox message: %w", err)
	}
	return nil
}

func (s *Store) MarkInboxProcessed(id string) error {
	_, err := s.DB.Exec("UPDATE federation_inbox SET processed = 1 WHERE id = ?", id)
	return err
}

func (s *Store) EnqueueDelivery(targetNode, messageID, messageJSON string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`INSERT INTO delivery_queue (target_node, message_id, message_json, next_attempt_at)
		VALUES (?, ?, ?, ?)`, targetNode, messageID, messageJSON, now)
	if err != nil {
		return fmt.Errorf("enqueue delivery: %w", err)
	}
	return nil
}

func (s *Store) GetPendingDeliveries(limit int) ([]model.DeliveryQueueRow, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var rows []model.DeliveryQueueRow
	err := s.DB.Select(&rows,
		"SELECT * FROM delivery_queue WHERE next_attempt_at <= ? ORDER BY next_attempt_at ASC LIMIT ?",
		now, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending deliveries: %w", err)
	}
	return rows, nil
}

func (s *Store) UpdateDeliveryAttempt(id int64, nextAttempt time.Time, attemptCount int, lastError string) error {
	_, err := s.DB.Exec(
		"UPDATE delivery_queue SET next_attempt_at=?, attempt_count=?, last_error=? WHERE id=?",
		nextAttempt.UTC().Format(time.RFC3339), attemptCount, lastError, id)
	return err
}

func (s *Store) RemoveDelivery(id int64) error {
	_, err := s.DB.Exec("DELETE FROM delivery_queue WHERE id = ?", id)
	return err
}

func (s *Store) RemoveExpiredDeliveries(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).UTC().Format(time.RFC3339)
	result, err := s.DB.Exec("DELETE FROM delivery_queue WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}
