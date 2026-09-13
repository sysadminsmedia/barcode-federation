package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) CreateClaim(claim *model.NamespaceClaim) error {
	rawJSON, err := json.Marshal(claim)
	if err != nil {
		return fmt.Errorf("marshal claim: %w", err)
	}
	_, err = s.DB.Exec(`INSERT INTO namespace_claims (id, claimed_by, namespace_type, scope, issued_at, expires_at, evidence, signature, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		claim.ID, claim.ClaimedBy, claim.NamespaceType,
		string(claim.Scope), claim.IssuedAt, nilIfEmpty(claim.ExpiresAt),
		string(claim.Evidence), claim.Signature, string(rawJSON))
	if err != nil {
		return fmt.Errorf("insert claim: %w", err)
	}
	return nil
}

func (s *Store) GetClaim(id string) (*model.NamespaceClaim, error) {
	var row model.ClaimRow
	err := s.DB.Get(&row, "SELECT * FROM namespace_claims WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("claim not found")
		}
		return nil, fmt.Errorf("get claim: %w", err)
	}
	var claim model.NamespaceClaim
	if err := json.Unmarshal([]byte(row.RawJSON), &claim); err != nil {
		return nil, fmt.Errorf("unmarshal claim: %w", err)
	}
	return &claim, nil
}

func (s *Store) ListClaimsByActor(actorID string) ([]model.NamespaceClaim, error) {
	var rows []model.ClaimRow
	err := s.DB.Select(&rows, "SELECT * FROM namespace_claims WHERE claimed_by = ? ORDER BY issued_at DESC", actorID)
	if err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	claims := make([]model.NamespaceClaim, 0, len(rows))
	for _, row := range rows {
		var claim model.NamespaceClaim
		if err := json.Unmarshal([]byte(row.RawJSON), &claim); err != nil {
			continue
		}
		claims = append(claims, claim)
	}
	return claims, nil
}
