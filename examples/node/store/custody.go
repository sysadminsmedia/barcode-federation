package store

import (
	"fmt"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

// CustodyTransferRow is the DB representation of a custody transfer.
type CustodyTransferRow struct {
	ID             int64   `db:"id"`
	RecordID       string  `db:"record_id"`
	FromActor      string  `db:"from_actor"`
	ToActor        string  `db:"to_actor"`
	TransferredAt  string  `db:"transferred_at"`
	Condition      string  `db:"condition"`
	DelegateExpiry *string `db:"delegate_expiry"`
	FromSignature  string  `db:"from_signature"`
	ToSignature    *string `db:"to_signature"`
	AcceptedAt     *string `db:"accepted_at"`
	MessageID      *string `db:"message_id"`
}

// CreateCustodyTransfer records a custody transfer and returns its ID.
func (s *Store) CreateCustodyTransfer(recordID string, notice *model.CustodyTransferNotice) (int64, error) {
	result, err := s.DB.Exec(`INSERT INTO custody_transfers
		(record_id, from_actor, to_actor, transferred_at, condition, delegate_expiry, from_signature)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		recordID, notice.FromActor, notice.ToActor, notice.TransferredAt,
		notice.Condition, nilIfEmpty(notice.ExpiresAt), notice.FromSignature)
	if err != nil {
		return 0, fmt.Errorf("create custody transfer: %w", err)
	}
	return result.LastInsertId()
}

// AcceptCustodyTransfer records the acceptance of a transfer.
func (s *Store) AcceptCustodyTransfer(recordID, toActor, toSignature string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`UPDATE custody_transfers SET to_signature=?, accepted_at=?
		WHERE id = (SELECT id FROM custody_transfers WHERE record_id=? AND to_actor=? AND to_signature IS NULL ORDER BY id DESC LIMIT 1)`,
		toSignature, now, recordID, toActor)
	if err != nil {
		return fmt.Errorf("accept custody transfer: %w", err)
	}
	return nil
}

// GetCurrentCustodian returns the current custodian of a record.
// If no custody transfers exist, returns the submittedBy actor.
func (s *Store) GetCurrentCustodian(recordID, submittedBy string) (string, error) {
	var toActor string
	err := s.DB.Get(&toActor,
		`SELECT to_actor FROM custody_transfers
		 WHERE record_id=? AND condition != 'rejected' AND to_signature IS NOT NULL
		 ORDER BY id DESC LIMIT 1`, recordID)
	if err != nil {
		return submittedBy, nil // no transfers, original submitter is custodian
	}
	return toActor, nil
}

// IsDelegate checks if an actor has active delegated update rights on a record.
func (s *Store) IsDelegate(recordID, actorID string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var count int
	err := s.DB.Get(&count,
		`SELECT COUNT(*) FROM custody_transfers
		 WHERE record_id=? AND to_actor=? AND to_signature IS NOT NULL
		 AND condition != 'rejected'
		 AND (delegate_expiry IS NULL OR delegate_expiry > ?)`,
		recordID, actorID, now)
	if err != nil {
		return false, fmt.Errorf("check delegate: %w", err)
	}
	return count > 0, nil
}

// GetCustodyChain returns all custody transfers for a record in order.
func (s *Store) GetCustodyChain(recordID string) ([]model.CustodyEntry, error) {
	var rows []CustodyTransferRow
	err := s.DB.Select(&rows,
		"SELECT * FROM custody_transfers WHERE record_id=? ORDER BY id ASC", recordID)
	if err != nil {
		return nil, fmt.Errorf("get custody chain: %w", err)
	}

	entries := make([]model.CustodyEntry, len(rows))
	for i, r := range rows {
		entries[i] = model.CustodyEntry{
			From:      r.FromActor,
			To:        r.ToActor,
			At:        r.TransferredAt,
			Condition: r.Condition,
			FromSignature: r.FromSignature,
		}
		if r.ToSignature != nil {
			entries[i].ToSignature = *r.ToSignature
		}
		if r.DelegateExpiry != nil {
			entries[i].DelegateExpiry = *r.DelegateExpiry
		}
	}
	return entries, nil
}

// SaveRelatedRecords stores the related record links for indexing.
func (s *Store) SaveRelatedRecords(recordID string, related []model.RelatedRecord) error {
	for _, r := range related {
		_, err := s.DB.Exec(`INSERT INTO related_records (record_id, related_cbi, relationship, quantity, description)
			VALUES (?, ?, ?, ?, ?)`,
			recordID, r.CBI, r.Relationship, r.Quantity, nilIfEmpty(r.Description))
		if err != nil {
			return fmt.Errorf("save related record: %w", err)
		}
	}
	return nil
}

// FindRecordsRelatedTo finds all records that declare a relationship to the given CBI.
func (s *Store) FindRecordsRelatedTo(cbi string) ([]model.BarcodeRecord, error) {
	var recordIDs []string
	err := s.DB.Select(&recordIDs,
		"SELECT DISTINCT record_id FROM related_records WHERE related_cbi = ?", cbi)
	if err != nil {
		return nil, fmt.Errorf("find related records: %w", err)
	}

	var records []model.BarcodeRecord
	for _, id := range recordIDs {
		rec, err := s.GetRecord(id)
		if err != nil {
			continue
		}
		records = append(records, *rec)
	}
	return records, nil
}
