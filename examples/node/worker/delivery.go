package worker

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/federation"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
)

const (
	minRetryDelay = 30 * time.Second
	maxRetryDelay = 24 * time.Hour
	maxRetryAge   = 7 * 24 * time.Hour
)

func StartDeliveryWorker(ctx context.Context, s *store.Store, client *federation.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processDeliveries(s, client)
			s.RemoveExpiredDeliveries(maxRetryAge)
		}
	}
}

func processDeliveries(s *store.Store, client *federation.Client) {
	items, err := s.GetPendingDeliveries(50)
	if err != nil {
		slog.Error("failed to get pending deliveries", "error", err)
		return
	}

	for _, item := range items {
		inboxURL := item.TargetNode + "/federation/inbox"
		err := client.PostToInbox(inboxURL, []byte(item.MessageJSON))
		if err != nil {
			nextDelay := time.Duration(math.Min(
				float64(minRetryDelay)*math.Pow(2, float64(item.AttemptCount)),
				float64(maxRetryDelay),
			))
			nextAttempt := time.Now().Add(nextDelay)
			errStr := err.Error()
			s.UpdateDeliveryAttempt(item.ID, nextAttempt, item.AttemptCount+1, errStr)
			slog.Warn("delivery failed, will retry",
				"target", item.TargetNode,
				"attempt", item.AttemptCount+1,
				"next_retry", nextAttempt.Format(time.RFC3339),
				"error", errStr)
		} else {
			s.RemoveDelivery(item.ID)
		}
	}
}
