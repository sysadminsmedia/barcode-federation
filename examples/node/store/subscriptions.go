package store

import (
	"encoding/json"
	"fmt"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

func (s *Store) CreateSubscription(sub *model.Subscription) error {
	_, err := s.DB.Exec(`INSERT INTO subscriptions (id, actor_id, scope, callback_url, events)
		VALUES (?, ?, ?, ?, ?)`,
		sub.ID, sub.ActorID, string(sub.Scope), sub.CallbackURL, string(sub.Events))
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
}

func (s *Store) ListSubscriptionsForCBI(cbi string, eventType string) ([]model.Subscription, error) {
	// For simplicity, return all subscriptions and let the caller filter by scope match.
	var subs []model.Subscription
	err := s.DB.Select(&subs, "SELECT * FROM subscriptions")
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}

	var matching []model.Subscription
	for _, sub := range subs {
		var events []string
		json.Unmarshal(sub.Events, &events)
		for _, e := range events {
			if e == eventType {
				matching = append(matching, sub)
				break
			}
		}
	}
	return matching, nil
}
