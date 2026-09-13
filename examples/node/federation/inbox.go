package federation

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/fbscommunity/barcode-federation/examples/node/api"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
)

type InboxHandler struct {
	Store       *store.Store
	NodeBaseURL string
	PolicyCheck func(senderNodeID string) bool
}

func (h *InboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.BadRequest(w, "failed to read body")
		return
	}

	var msg model.FederationMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		api.BadRequest(w, "invalid federation message JSON")
		return
	}

	if msg.FBS != "1.0" || msg.Type != "FederationMessage" {
		api.BadRequest(w, "invalid fbs version or type")
		return
	}

	// Check federation policy
	if h.PolicyCheck != nil && !h.PolicyCheck(msg.Sender) {
		api.Error(w, http.StatusForbidden, "forbidden", "sender not permitted by federation policy")
		return
	}

	// TODO: Verify HTTP signature on inbound request against sender's published key.
	// For this reference implementation, we accept messages and log a warning.
	slog.Warn("HTTP signature verification not fully implemented for inbound federation", "sender", msg.Sender)

	if err := h.Store.SaveInboxMessage(&msg); err != nil {
		slog.Error("failed to save inbox message", "error", err)
		api.InternalError(w, err)
		return
	}

	// Process by activity type
	switch msg.Activity {
	case "Publish":
		h.handlePublish(&msg)
	case "Retract":
		h.handleRetract(&msg)
	case "ClaimPublish":
		h.handleClaimPublish(&msg)
	case "ClaimRetract":
		slog.Info("received ClaimRetract", "id", msg.ID)
	case "KeySuccessionDeclaration":
		h.handleSuccession(&msg)
	case "KeyRotation":
		slog.Info("received KeyRotation", "id", msg.ID)
	case "NodeCompromiseNotice":
		slog.Warn("received NodeCompromiseNotice", "sender", msg.Sender, "id", msg.ID)
	case "TransparencyAlert":
		slog.Warn("received TransparencyAlert", "sender", msg.Sender, "id", msg.ID)
	case "CustodyTransfer":
		h.handleCustodyTransfer(&msg)
	case "CustodyAccept":
		h.handleCustodyAccept(&msg)
	case "Ping":
		slog.Info("received Ping from", "sender", msg.Sender)
		// TODO: send Pong back
	case "Pong":
		slog.Info("received Pong from", "sender", msg.Sender)
	default:
		slog.Warn("unknown activity type", "activity", msg.Activity)
	}

	h.Store.MarkInboxProcessed(msg.ID)
	api.JSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *InboxHandler) handlePublish(msg *model.FederationMessage) {
	var rec model.BarcodeRecord
	if err := json.Unmarshal(msg.Object, &rec); err != nil {
		slog.Error("failed to unmarshal published record", "error", err)
		return
	}
	// TODO: verify record signature against actor's public key
	if err := h.Store.CreateRecord(&rec); err != nil {
		slog.Error("failed to store federated record", "error", err, "id", rec.ID)
	} else {
		slog.Info("stored federated record", "id", rec.ID, "cbi", rec.CBI)
	}
}

func (h *InboxHandler) handleRetract(msg *model.FederationMessage) {
	var ref model.BarcodeRecordRef
	if err := json.Unmarshal(msg.Object, &ref); err != nil {
		slog.Error("failed to unmarshal retract ref", "error", err)
		return
	}
	rev := model.RevisionEntry{
		RevisionID: ref.ID,
		At:         msg.PublishedAt,
		By:         msg.Sender,
		ChangeType: "retract",
	}
	if err := h.Store.RetractRecord(ref.ID, rev); err != nil {
		slog.Error("failed to retract record", "error", err, "id", ref.ID)
	} else {
		slog.Info("retracted federated record", "id", ref.ID)
	}
}

func (h *InboxHandler) handleClaimPublish(msg *model.FederationMessage) {
	var claim model.NamespaceClaim
	if err := json.Unmarshal(msg.Object, &claim); err != nil {
		slog.Error("failed to unmarshal claim", "error", err)
		return
	}
	if err := h.Store.CreateClaim(&claim); err != nil {
		slog.Error("failed to store federated claim", "error", err, "id", claim.ID)
	} else {
		slog.Info("stored federated claim", "id", claim.ID)
	}
}

func (h *InboxHandler) handleSuccession(msg *model.FederationMessage) {
	var decl model.KeySuccessionDeclaration
	if err := json.Unmarshal(msg.Object, &decl); err != nil {
		slog.Error("failed to unmarshal succession", "error", err)
		return
	}
	if err := h.Store.SaveSuccession(&decl); err != nil {
		slog.Error("failed to store succession", "error", err, "id", decl.ID)
	} else {
		slog.Info("stored key succession declaration", "id", decl.ID, "oldFni", decl.OldFNI)
	}
}

func (h *InboxHandler) handleCustodyTransfer(msg *model.FederationMessage) {
	var notice model.CustodyTransferNotice
	if err := json.Unmarshal(msg.Object, &notice); err != nil {
		slog.Error("failed to unmarshal custody transfer", "error", err)
		return
	}
	if _, err := h.Store.CreateCustodyTransfer(notice.RecordID, &notice); err != nil {
		slog.Error("failed to store custody transfer", "error", err, "record", notice.RecordID)
	} else {
		slog.Info("stored custody transfer",
			"record", notice.RecordID,
			"from", notice.FromActor,
			"to", notice.ToActor,
			"condition", notice.Condition,
		)
	}
}

func (h *InboxHandler) handleCustodyAccept(msg *model.FederationMessage) {
	var acceptance model.CustodyAcceptance
	if err := json.Unmarshal(msg.Object, &acceptance); err != nil {
		slog.Error("failed to unmarshal custody acceptance", "error", err)
		return
	}
	if err := h.Store.AcceptCustodyTransfer(acceptance.RecordID, acceptance.AcceptedBy, acceptance.ToSignature); err != nil {
		slog.Error("failed to record custody acceptance", "error", err, "record", acceptance.RecordID)
	} else {
		slog.Info("custody transfer accepted",
			"record", acceptance.RecordID,
			"by", acceptance.AcceptedBy,
		)
	}
}
