package federation

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
	"github.com/google/uuid"
)

type Dispatcher struct {
	Store      *store.Store
	NodeKey    []byte // ed25519.PrivateKey is []byte
	NodeBaseURL string
}

func NewDispatcher(s *store.Store, nodeKey []byte, nodeBaseURL string) *Dispatcher {
	return &Dispatcher{Store: s, NodeKey: nodeKey, NodeBaseURL: nodeBaseURL}
}

func (d *Dispatcher) DispatchPublish(rec interface{}) {
	d.dispatch("Publish", rec)
}

func (d *Dispatcher) DispatchRetract(recordID, cbi string) {
	ref := model.BarcodeRecordRef{
		Type: "BarcodeRecordRef",
		ID:   recordID,
		CBI:  cbi,
	}
	d.dispatch("Retract", ref)
}

func (d *Dispatcher) DispatchClaimPublish(claim interface{}) {
	d.dispatch("ClaimPublish", claim)
}

func (d *Dispatcher) DispatchActivity(activityType string, object interface{}) {
	d.dispatch(activityType, object)
}

func (d *Dispatcher) dispatch(activity string, object interface{}) {
	peers, err := d.Store.ListFederatedPeers()
	if err != nil {
		slog.Error("failed to list peers for dispatch", "error", err)
		return
	}
	if len(peers) == 0 {
		return
	}

	objectJSON, err := json.Marshal(object)
	if err != nil {
		slog.Error("failed to marshal object", "error", err)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for _, peer := range peers {
		msgID := d.NodeBaseURL + "/federation/messages/" + uuid.New().String()
		msg := model.FederationMessage{
			FBS:         "1.0",
			Type:        "FederationMessage",
			ID:          msgID,
			Sender:      d.NodeBaseURL,
			Recipient:   peer.NodeID,
			Activity:    activity,
			Object:      objectJSON,
			PublishedAt: now,
		}

		sig, err := crypto.SignRecord(&msg, d.NodeKey)
		if err != nil {
			slog.Error("failed to sign federation message", "error", err)
			continue
		}
		msg.Signature = sig

		if err := d.Store.SaveOutboxMessage(&msg); err != nil {
			slog.Error("failed to save outbox message", "error", err)
			continue
		}

		msgJSON, _ := json.Marshal(msg)
		inboxURL := peer.NodeID + "/federation/inbox"
		if err := d.Store.EnqueueDelivery(peer.NodeID, msg.ID, string(msgJSON)); err != nil {
			slog.Error("failed to enqueue delivery", "error", err, "target", inboxURL)
		}
	}
}
