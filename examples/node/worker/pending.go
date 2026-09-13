package worker

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/store"
)

func StartPendingVerificationWorker(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processPendingVerifications(s)
		}
	}
}

func processPendingVerifications(s *store.Store) {
	items, err := s.GetPendingVerifications(20)
	if err != nil {
		slog.Error("failed to get pending verifications", "error", err)
		return
	}

	for _, item := range items {
		// TODO: fetch succession document from item.SuccessionURL and verify it
		// For now, just retry with exponential backoff up to 30 days
		if item.AttemptCount >= 30 {
			slog.Warn("pending verification exceeded max attempts, marking disputed",
				"id", item.ID, "succession_url", item.SuccessionURL)
			s.RemovePendingVerification(item.ID)
			continue
		}

		delay := time.Duration(math.Min(
			float64(5*time.Minute)*math.Pow(2, float64(item.AttemptCount)),
			float64(24*time.Hour),
		))
		nextRetry := time.Now().Add(delay).UTC().Format(time.RFC3339)
		s.UpdatePendingVerification(item.ID, nextRetry, item.AttemptCount+1)
		slog.Info("retrying pending verification", "id", item.ID, "attempt", item.AttemptCount+1)
	}
}
