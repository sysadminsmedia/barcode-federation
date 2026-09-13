package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) CreateRecord(rec *model.BarcodeRecord) error {
	rawJSON, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	var nsClaim, gs1CrossRef *string
	if rec.NamespaceClaim != nil {
		b, _ := json.Marshal(rec.NamespaceClaim)
		str := string(b)
		nsClaim = &str
	}
	if rec.GS1CrossRef != nil {
		b, _ := json.Marshal(rec.GS1CrossRef)
		str := string(b)
		gs1CrossRef = &str
	}

	tx, err := s.DB.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO records (id, cbi, symbology, value, status, submitted_by, submitted_at, updated_at, expires_at, origin_node, namespace_claim, metadata_schema, metadata_schema_digest, metadata, gs1_cross_ref, lifecycle_schema, signature, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.CBI, rec.Symbology, rec.Value, rec.Status,
		rec.SubmittedBy, rec.SubmittedAt, rec.UpdatedAt, nilIfEmpty(rec.ExpiresAt),
		rec.OriginNode, nsClaim, rec.MetadataSchema, nilIfEmpty(rec.MetadataSchemaDigest),
		string(rec.Metadata), gs1CrossRef, nilIfEmpty(rec.LifecycleSchema),
		rec.Signature, string(rawJSON))
	if err != nil {
		return fmt.Errorf("insert record: %w", err)
	}

	for _, rev := range rec.RevisionHistory {
		_, err = tx.Exec(`INSERT INTO revision_history (record_id, revision_id, at, by_actor, change_type, summary, previous_revision_signature)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			rec.ID, rev.RevisionID, rev.At, rev.By, rev.ChangeType, nilIfEmpty(rev.Summary), rev.PreviousRevisionSignature)
		if err != nil {
			return fmt.Errorf("insert revision: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Store) GetRecord(id string) (*model.BarcodeRecord, error) {
	var row model.RecordRow
	err := s.DB.Get(&row, "SELECT * FROM records WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("record not found")
		}
		return nil, fmt.Errorf("get record: %w", err)
	}
	return s.rowToRecord(&row)
}

func (s *Store) GetRecordByCBI(symbology, value string) ([]model.BarcodeRecord, error) {
	cbi := symbology + ":" + value
	var rows []model.RecordRow
	err := s.DB.Select(&rows, `SELECT * FROM records WHERE cbi = ? ORDER BY
		CASE status WHEN 'active' THEN 0 WHEN 'expired' THEN 1 WHEN 'retracted' THEN 2 END,
		verification_level DESC, updated_at DESC`, cbi)
	if err != nil {
		return nil, fmt.Errorf("get records by cbi: %w", err)
	}
	return s.rowsToRecords(rows)
}

// RecordFilters holds optional query parameters for searching records.
type RecordFilters struct {
	Query             string // partial barcode value or CBI match
	Symbology         string // exact symbology match
	Status            string // active, retracted, or expired (default: active only)
	SubmittedBy       string // exact actor ID
	OriginNode        string // exact origin node URL
	MinVerification   string // minimum verification level (0-3)
	MetadataSchemaURL string // exact metadata schema URL
}

func (s *Store) SearchRecordsFiltered(f RecordFilters, limit, offset int) ([]model.BarcodeRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	where := "1=1"
	args := []interface{}{}

	if f.Status != "" {
		where += " AND status = ?"
		args = append(args, f.Status)
	} else {
		where += " AND status = 'active'"
	}

	if f.Query != "" {
		where += " AND (value LIKE ? OR cbi LIKE ?)"
		q := "%" + f.Query + "%"
		args = append(args, q, q)
	}
	if f.Symbology != "" {
		where += " AND symbology = ?"
		args = append(args, f.Symbology)
	}
	if f.SubmittedBy != "" {
		where += " AND submitted_by = ?"
		args = append(args, f.SubmittedBy)
	}
	if f.OriginNode != "" {
		where += " AND origin_node = ?"
		args = append(args, f.OriginNode)
	}
	if f.MinVerification != "" {
		where += " AND verification_level >= ?"
		args = append(args, f.MinVerification)
	}
	if f.MetadataSchemaURL != "" {
		where += " AND metadata_schema = ?"
		args = append(args, f.MetadataSchemaURL)
	}

	var total int
	if err := s.DB.Get(&total, "SELECT COUNT(*) FROM records WHERE "+where, args...); err != nil {
		return nil, 0, fmt.Errorf("count records: %w", err)
	}

	selectQuery := "SELECT * FROM records WHERE " + where +
		" ORDER BY verification_level DESC, updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var rows []model.RecordRow
	if err := s.DB.Select(&rows, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("search records: %w", err)
	}

	records, err := s.rowsToRecords(rows)
	return records, total, err
}

func (s *Store) SearchRecords(query string, symbology string, limit, offset int) ([]model.BarcodeRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	where := "status = 'active'"
	args := []interface{}{}

	if query != "" {
		where += " AND (value LIKE ? OR cbi LIKE ?)"
		q := "%" + query + "%"
		args = append(args, q, q)
	}
	if symbology != "" {
		where += " AND symbology = ?"
		args = append(args, symbology)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM records WHERE " + where
	if err := s.DB.Get(&total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count records: %w", err)
	}

	selectQuery := "SELECT * FROM records WHERE " + where +
		" ORDER BY verification_level DESC, updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var rows []model.RecordRow
	if err := s.DB.Select(&rows, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("search records: %w", err)
	}

	records, err := s.rowsToRecords(rows)
	return records, total, err
}

func (s *Store) GetActiveRecordByCBIAndActor(cbi, actorID string) (*model.BarcodeRecord, error) {
	var row model.RecordRow
	err := s.DB.Get(&row, "SELECT * FROM records WHERE cbi = ? AND submitted_by = ? AND status = 'active'", cbi, actorID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active record: %w", err)
	}
	return s.rowToRecord(&row)
}

func (s *Store) UpdateRecord(rec *model.BarcodeRecord) error {
	rawJSON, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	var nsClaim *string
	if rec.NamespaceClaim != nil {
		b, _ := json.Marshal(rec.NamespaceClaim)
		str := string(b)
		nsClaim = &str
	}

	tx, err := s.DB.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE records SET status=?, updated_at=?, metadata=?, metadata_schema=?, metadata_schema_digest=?, namespace_claim=?, signature=?, raw_json=? WHERE id=?`,
		rec.Status, rec.UpdatedAt, string(rec.Metadata), rec.MetadataSchema, nilIfEmpty(rec.MetadataSchemaDigest), nsClaim, rec.Signature, string(rawJSON), rec.ID)
	if err != nil {
		return fmt.Errorf("update record: %w", err)
	}

	if len(rec.RevisionHistory) > 0 {
		latest := rec.RevisionHistory[len(rec.RevisionHistory)-1]
		_, err = tx.Exec(`INSERT INTO revision_history (record_id, revision_id, at, by_actor, change_type, summary, previous_revision_signature)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			rec.ID, latest.RevisionID, latest.At, latest.By, latest.ChangeType, nilIfEmpty(latest.Summary), latest.PreviousRevisionSignature)
		if err != nil {
			return fmt.Errorf("insert revision: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Store) RetractRecord(id string, rev model.RevisionEntry) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE records SET status='retracted', updated_at=? WHERE id=?", rev.At, id)
	if err != nil {
		return fmt.Errorf("retract record: %w", err)
	}

	_, err = tx.Exec(`INSERT INTO revision_history (record_id, revision_id, at, by_actor, change_type, summary, previous_revision_signature)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, rev.RevisionID, rev.At, rev.By, "retract", nil, rev.PreviousRevisionSignature)
	if err != nil {
		return fmt.Errorf("insert retract revision: %w", err)
	}

	return tx.Commit()
}

func (s *Store) ExpireRecords() (int, error) {
	result, err := s.DB.Exec("UPDATE records SET status='expired' WHERE status='active' AND expires_at IS NOT NULL AND expires_at <= datetime('now')")
	if err != nil {
		return 0, fmt.Errorf("expire records: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (s *Store) rowToRecord(row *model.RecordRow) (*model.BarcodeRecord, error) {
	var rec model.BarcodeRecord
	if err := json.Unmarshal([]byte(row.RawJSON), &rec); err != nil {
		return nil, fmt.Errorf("unmarshal record: %w", err)
	}
	// Ensure status is current (may have been updated)
	rec.Status = row.Status
	rec.UpdatedAt = row.UpdatedAt

	// Load revision history from DB
	var revisions []struct {
		RevisionID                string  `db:"revision_id"`
		At                        string  `db:"at"`
		By                        string  `db:"by_actor"`
		ChangeType                string  `db:"change_type"`
		Summary                   *string `db:"summary"`
		PreviousRevisionSignature *string `db:"previous_revision_signature"`
	}
	if err := s.DB.Select(&revisions, "SELECT revision_id, at, by_actor, change_type, summary, previous_revision_signature FROM revision_history WHERE record_id = ? ORDER BY id ASC", row.ID); err == nil {
		rec.RevisionHistory = make([]model.RevisionEntry, len(revisions))
		for i, r := range revisions {
			rec.RevisionHistory[i] = model.RevisionEntry{
				RevisionID:                r.RevisionID,
				At:                        r.At,
				By:                        r.By,
				ChangeType:                r.ChangeType,
				PreviousRevisionSignature: r.PreviousRevisionSignature,
			}
			if r.Summary != nil {
				rec.RevisionHistory[i].Summary = *r.Summary
			}
		}
	}
	return &rec, nil
}

func (s *Store) rowsToRecords(rows []model.RecordRow) ([]model.BarcodeRecord, error) {
	records := make([]model.BarcodeRecord, 0, len(rows))
	for _, row := range rows {
		rec, err := s.rowToRecord(&row)
		if err != nil {
			continue
		}
		records = append(records, *rec)
	}
	return records, nil
}

func nilIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
