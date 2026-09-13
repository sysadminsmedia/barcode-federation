package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/store"
)

func StartExpiryWorker(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.ExpireRecords()
			if err != nil {
				slog.Error("expiry worker error", "error", err)
			} else if n > 0 {
				slog.Info("expired records", "count", n)
			}
		}
	}
}
