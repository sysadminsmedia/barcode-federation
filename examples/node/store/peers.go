package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) AddPeer(nodeID, metaURL, relationship string) error {
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO peers (node_id, meta_url, relationship) VALUES (?, ?, ?)`,
		nodeID, metaURL, relationship)
	if err != nil {
		return fmt.Errorf("add peer: %w", err)
	}
	return nil
}

func (s *Store) GetPeer(nodeID string) (*model.Peer, error) {
	var p model.Peer
	err := s.DB.Get(&p, "SELECT * FROM peers WHERE node_id = ?", nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("peer not found: %s", nodeID)
		}
		return nil, fmt.Errorf("get peer: %w", err)
	}
	return &p, nil
}

func (s *Store) ListPeers() ([]model.Peer, error) {
	var peers []model.Peer
	err := s.DB.Select(&peers, "SELECT * FROM peers ORDER BY added_at")
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	return peers, nil
}

func (s *Store) ListFederatedPeers() ([]model.Peer, error) {
	var peers []model.Peer
	err := s.DB.Select(&peers, "SELECT * FROM peers WHERE relationship = 'federated' ORDER BY added_at")
	if err != nil {
		return nil, fmt.Errorf("list federated peers: %w", err)
	}
	return peers, nil
}

func (s *Store) UpdatePeerKey(nodeID, publicKeyPem, federationPolicy string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec("UPDATE peers SET public_key_pem=?, federation_policy=?, last_seen=? WHERE node_id=?",
		publicKeyPem, federationPolicy, now, nodeID)
	return err
}

func (s *Store) TouchPeer(nodeID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec("UPDATE peers SET last_seen=? WHERE node_id=?", now, nodeID)
	return err
}
