package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
)

// NotifySubscribers sends webhook notifications for a record event.
func NotifySubscribers(s *store.Store, rec *model.BarcodeRecord, eventType string) {
	subs, err := s.ListSubscriptionsForCBI(rec.CBI, eventType)
	if err != nil {
		slog.Error("failed to list subscriptions", "error", err)
		return
	}

	for _, sub := range subs {
		go deliverWebhook(sub.CallbackURL, rec, eventType)
	}
}

func deliverWebhook(url string, rec *model.BarcodeRecord, eventType string) {
	payload := map[string]interface{}{
		"fbs":       "1.0",
		"type":      "WebhookNotification",
		"event":     eventType,
		"record":    rec,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal webhook payload", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to create webhook request", "error", err, "url", url)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("webhook delivery failed", "url", url, "error", err)
		return
	}
	resp.Body.Close()
	slog.Info("webhook delivered", "url", url, "status", resp.StatusCode)
}
