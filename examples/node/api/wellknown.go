package api

import (
	"crypto/ed25519"
	"net/http"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	pubPEM, err := crypto.EncodePublicKeyPEM(s.NodeKey.Public().(ed25519.PublicKey))
	if err != nil {
		InternalError(w, err)
		return
	}

	baseURL := s.NodeBaseURL()
	capabilities := []string{"federation", "bulk-import", "subscriptions", "namespace-claims", "peer-list"}

	meta := model.NodeMeta{
		FBS:              "1.0",
		Type:             "NodeMeta",
		NodeID:           baseURL,
		DisplayName:      s.Config.Node.DisplayName,
		OperatedBy:       s.Config.Node.OperatedBy,
		Version:          "1.0.0",
		Capabilities:     capabilities,
		FederationPolicy: s.Config.Federation.Policy,
		Inbox:            baseURL + "/federation/inbox",
		Outbox:           baseURL + "/federation/outbox",
		Peers:            baseURL + "/.well-known/fbs/peers",
		PublicKey: model.NodeKey{
			ID:           baseURL + "#node-key",
			Algorithm:    "Ed25519",
			PublicKeyPem: pubPEM,
		},
		TrustAnchors:     s.Config.Node.TrustAnchors,
		SchemaRegistries: s.Config.Node.SchemaRegistries,
		Contact:          s.Config.Node.Contact,
	}

	JSON(w, http.StatusOK, meta)
}

func (s *Server) handleActorProfile(w http.ResponseWriter, r *http.Request) {
	actorLocalID := chi.URLParam(r, "actorID")
	actorID := "fbs://" + s.Config.Node.Host + "/" + actorLocalID

	actor, err := s.Store.GetActor(actorID)
	if err != nil {
		NotFound(w, "actor not found")
		return
	}

	JSON(w, http.StatusOK, actor.ToProfile(s.NodeBaseURL()))
}

func (s *Server) handleWebRecord(w http.ResponseWriter, r *http.Request) {
	actorLocalID := r.URL.Query().Get("actor")
	if actorLocalID == "" {
		BadRequest(w, "actor query parameter required")
		return
	}
	actorID := "fbs://" + s.Config.Node.Host + "/" + actorLocalID

	actor, err := s.Store.GetActor(actorID)
	if err != nil {
		NotFound(w, "actor not found")
		return
	}

	JSON(w, http.StatusOK, actor.ToProfile(s.NodeBaseURL()))
}

func (s *Server) handlePeerList(w http.ResponseWriter, r *http.Request) {
	peers, err := s.Store.ListPeers()
	if err != nil {
		InternalError(w, err)
		return
	}

	entries := make([]model.PeerEntry, 0, len(peers))
	for _, p := range peers {
		entries = append(entries, model.PeerEntry{
			NodeID:       p.NodeID,
			Meta:         p.MetaURL,
			AddedAt:      p.AddedAt,
			Relationship: p.Relationship,
		})
	}

	peerList := model.PeerList{
		FBS:         "1.0",
		Type:        "PeerList",
		NodeID:      s.NodeBaseURL(),
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
		TTL:         3600,
		Peers:       entries,
	}

	JSON(w, http.StatusOK, peerList)
}
