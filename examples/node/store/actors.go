package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

const actorColumns = `id, profile, display_name, node, public_key_pem, capabilities, namespace_claims, api_token, private_key_pem`

func (s *Store) GetActor(id string) (*model.Actor, error) {
	var a model.Actor
	err := s.DB.Get(&a, "SELECT "+actorColumns+" FROM actors WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("get actor %s: %w", id, err)
	}
	a.ParseCapabilities()
	return &a, nil
}

func (s *Store) GetActorByToken(token string) (*model.Actor, error) {
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])
	var a model.Actor
	err := s.DB.Get(&a, "SELECT "+actorColumns+" FROM actors WHERE api_token = ?", hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid token")
		}
		return nil, fmt.Errorf("lookup actor by token: %w", err)
	}
	a.ParseCapabilities()
	return &a, nil
}

func (s *Store) CreateActor(a *model.Actor, token string) error {
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])
	_, err := s.DB.Exec(`INSERT INTO actors (id, profile, display_name, node, public_key_pem, capabilities, namespace_claims, api_token, private_key_pem)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Profile, a.DisplayName, a.Node, a.PublicKeyPem,
		a.CapabilitiesRaw, a.NamespaceClaimsRaw, hash, a.PrivateKeyPem)
	if err != nil {
		return fmt.Errorf("create actor: %w", err)
	}
	return nil
}

type ActorForCreate struct {
	ID              string
	Profile         string
	DisplayName     string
	Node            string
	PublicKeyPem    string
	CapabilitiesRaw string
	PrivateKeyPem   string
}

func (s *Store) CreateActorWithKey(a *ActorForCreate, token string) error {
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])
	_, err := s.DB.Exec(`INSERT INTO actors (id, profile, display_name, node, public_key_pem, capabilities, namespace_claims, api_token, private_key_pem)
		VALUES (?, ?, ?, ?, ?, ?, '[]', ?, ?)`,
		a.ID, a.Profile, a.DisplayName, a.Node, a.PublicKeyPem,
		a.CapabilitiesRaw, hash, a.PrivateKeyPem)
	if err != nil {
		return fmt.Errorf("create actor: %w", err)
	}
	return nil
}

func (s *Store) ListActors() ([]model.Actor, error) {
	var actors []model.Actor
	err := s.DB.Select(&actors, "SELECT "+actorColumns+" FROM actors ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list actors: %w", err)
	}
	for i := range actors {
		actors[i].ParseCapabilities()
	}
	return actors, nil
}
