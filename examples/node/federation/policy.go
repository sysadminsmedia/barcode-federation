package federation

import "github.com/fbscommunity/barcode-federation/examples/node/config"

func IsAllowed(cfg *config.FederationConfig, nodeID string) bool {
	switch cfg.Policy {
	case "open":
		return true
	case "allowlist":
		for _, allowed := range cfg.Allowlist {
			if allowed == nodeID {
				return true
			}
		}
		return false
	case "blocklist":
		for _, blocked := range cfg.Blocklist {
			if blocked == nodeID {
				return false
			}
		}
		return true
	default:
		return true
	}
}
