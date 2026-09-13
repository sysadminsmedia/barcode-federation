package model

import "encoding/json"

type Subscription struct {
	ID          string          `json:"id" db:"id"`
	ActorID     string          `json:"actorId" db:"actor_id"`
	Scope       json.RawMessage `json:"scope" db:"scope"`
	CallbackURL string          `json:"callbackUrl" db:"callback_url"`
	Events      json.RawMessage `json:"events" db:"events"`
	CreatedAt   string          `json:"createdAt" db:"created_at"`
}

type SubscriptionRequest struct {
	Scope       json.RawMessage `json:"scope"`
	CallbackURL string          `json:"callbackUrl"`
	Events      []string        `json:"events"`
}
