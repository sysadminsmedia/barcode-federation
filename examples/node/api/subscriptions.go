package api

import (
	"encoding/json"
	"net/http"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/google/uuid"
)

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())

	var req model.SubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if req.CallbackURL == "" {
		BadRequest(w, "callbackUrl is required")
		return
	}
	if len(req.Events) == 0 {
		BadRequest(w, "events array is required")
		return
	}

	eventsJSON, _ := json.Marshal(req.Events)

	sub := &model.Subscription{
		ID:          uuid.New().String(),
		ActorID:     actor.ID,
		Scope:       req.Scope,
		CallbackURL: req.CallbackURL,
		Events:      eventsJSON,
	}

	if err := s.Store.CreateSubscription(sub); err != nil {
		InternalError(w, err)
		return
	}

	JSON(w, http.StatusCreated, sub)
}
