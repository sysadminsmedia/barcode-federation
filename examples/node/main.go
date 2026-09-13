package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/api"
	"github.com/fbscommunity/barcode-federation/examples/node/config"
	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/federation"
	"github.com/fbscommunity/barcode-federation/examples/node/quorum"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
	"github.com/fbscommunity/barcode-federation/examples/node/transparencylog"
	"github.com/fbscommunity/barcode-federation/examples/node/worker"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	initMode := flag.Bool("init", false, "initialize a new node (generate keys and default config)")
	createActor := flag.String("create-actor", "", "create an actor with the given local ID (e.g. 'acme-corp')")
	actorProfile := flag.String("profile", "PublicUser", "profile for the new actor")
	actorDisplay := flag.String("display-name", "", "display name for the new actor")
	actorCaps := flag.String("capabilities", "", "JSON array of capabilities (overrides profile defaults)")
	flag.Parse()

	if *initMode {
		runInit(*configPath)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	nodeKey, err := fbscrypto.LoadOrGenerateKey(cfg.Node.KeyFile)
	if err != nil {
		slog.Error("failed to load node key", "error", err)
		os.Exit(1)
	}
	pubPEM, _ := fbscrypto.EncodePublicKeyPEM(nodeKey.Public().(ed25519.PublicKey))
	slog.Info("node key loaded",
		"fingerprint", fbscrypto.KeyFingerprint(nodeKey.Public().(ed25519.PublicKey)),
	)

	db, err := store.New(cfg.Database.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Handle actor creation subcommand
	if *createActor != "" {
		createActorCmd(db, cfg, nodeKey, *createActor, *actorProfile, *actorDisplay, *actorCaps, pubPEM)
		return
	}

	// Create API server
	srv := api.NewServer(cfg, db, nodeKey)
	nodeBaseURL := srv.NodeBaseURL()

	// Set up federation
	keyID := nodeBaseURL + "#node-key"
	fedClient := federation.NewClient(nodeKey, keyID)
	dispatcher := federation.NewDispatcher(db, nodeKey, nodeBaseURL)
	srv.Dispatcher = dispatcher

	// Set up federation handlers
	srv.FederationInboxHandler = &federation.InboxHandler{
		Store:       db,
		NodeBaseURL: nodeBaseURL,
		PolicyCheck: func(senderNodeID string) bool {
			return federation.IsAllowed(&cfg.Federation, senderNodeID)
		},
	}
	srv.FederationOutboxHandler = &federation.OutboxHandler{
		Store:       db,
		NodeBaseURL: nodeBaseURL,
	}

	// Set up Verification Transparency Log
	logID := nodeBaseURL + "/fbs-log"
	// Use a dedicated log keypair (derived from node key for simplicity in reference impl)
	_, logKey, err := fbscrypto.GenerateKeypair()
	if err != nil {
		slog.Error("failed to generate log keypair", "error", err)
		os.Exit(1)
	}
	tlog, err := transparencylog.NewTransparencyLog(logID, logKey)
	if err != nil {
		slog.Error("failed to create transparency log", "error", err)
		os.Exit(1)
	}
	srv.TransparencyLog = tlog
	slog.Info("transparency log initialized", "logId", logID)

	// Set up Quorum Coordinator
	coordinator := quorum.NewCoordinator(nodeBaseURL, nodeKey, tlog)
	srv.QuorumCoordinator = coordinator
	slog.Info("quorum coordinator initialized")

	// Add initial peers from config
	for _, peerURL := range cfg.Federation.Peers {
		metaURL := peerURL + "/.well-known/fbs/meta"
		if err := db.AddPeer(peerURL, metaURL, "federated"); err != nil {
			slog.Warn("failed to add peer", "peer", peerURL, "error", err)
		}
	}

	// Start background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.StartDeliveryWorker(ctx, db, fedClient)
	go worker.StartExpiryWorker(ctx, db)
	go worker.StartBulkProcessor(ctx, db, nodeBaseURL)
	go worker.StartPendingVerificationWorker(ctx, db)

	// Start HTTP server
	httpSrv := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: srv.Router,
	}

	go func() {
		slog.Info("starting FBS node",
			"addr", cfg.Server.Addr,
			"host", cfg.Node.Host,
			"policy", cfg.Federation.Policy,
		)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("shutting down...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpSrv.Shutdown(shutdownCtx)
	slog.Info("shutdown complete")
}

func runInit(configPath string) {
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config file %s already exists. Remove it to re-initialize.\n", configPath)
		return
	}

	cfg := config.Default()
	example := `# FBS Node Configuration
[node]
host = "localhost:8080"
display_name = "My FBS Node"
operated_by = "Your Name"
contact = "admin@example.com"
key_file = "node-key.pem"

[database]
path = "fbs-node.db"

[server]
addr = ":8080"

[federation]
policy = "open"
# peers = ["https://peer1.example.com", "https://peer2.example.com"]
# allowlist = []
# blocklist = []
`
	if err := os.WriteFile(configPath, []byte(example), 0644); err != nil {
		fmt.Printf("Failed to write config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", configPath)

	_, err := fbscrypto.LoadOrGenerateKey(cfg.Node.KeyFile)
	if err != nil {
		fmt.Printf("Failed to generate node key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated node keypair: %s\n", cfg.Node.KeyFile)
	fmt.Println("Run the node with: go run . -config config.toml")
}

func createActorCmd(db *store.Store, cfg *config.Config, nodeKey ed25519.PrivateKey, localID, profile, displayName, capsJSON, nodePubPEM string) {
	nodeBaseURL := "https://" + cfg.Node.Host
	actorID := "fbs://" + cfg.Node.Host + "/" + localID

	// Generate actor keypair
	_, actorPriv, err := fbscrypto.GenerateKeypair()
	if err != nil {
		slog.Error("failed to generate actor keypair", "error", err)
		os.Exit(1)
	}
	actorPubPEM, _ := fbscrypto.EncodePublicKeyPEM(actorPriv.Public().(ed25519.PublicKey))
	actorPrivPEM, _ := fbscrypto.EncodePrivateKeyPEM(actorPriv)

	// Determine capabilities
	var caps []string
	if capsJSON != "" {
		if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
			slog.Error("invalid capabilities JSON", "error", err)
			os.Exit(1)
		}
	} else {
		switch profile {
		case "Manufacturer":
			caps = []string{"record:read", "record:submit", "record:submit:bulk", "record:update:own", "record:retract:own", "namespace:claim", "namespace:verify", "subscription:create"}
		case "RetailOperator":
			caps = []string{"record:read", "record:submit", "record:submit:bulk", "record:update:own", "record:retract:own", "subscription:create"}
		default:
			caps = []string{"record:read", "record:submit"}
		}
	}

	capsBytes, _ := json.Marshal(caps)

	// Generate API token
	token := fmt.Sprintf("fbs_%s_%s", localID, time.Now().Format("20060102150405"))

	actor := &store.ActorForCreate{
		ID:              actorID,
		Profile:         profile,
		DisplayName:     displayName,
		Node:            nodeBaseURL,
		PublicKeyPem:    actorPubPEM,
		CapabilitiesRaw: string(capsBytes),
		PrivateKeyPem:   actorPrivPEM,
	}

	if err := db.CreateActorWithKey(actor, token); err != nil {
		slog.Error("failed to create actor", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Actor created: %s\n", actorID)
	fmt.Printf("  Profile:      %s\n", profile)
	fmt.Printf("  Capabilities: %v\n", caps)
	fmt.Printf("  API Token:    %s\n", token)
	fmt.Printf("  Fingerprint:  %s\n", fbscrypto.KeyFingerprint(actorPriv.Public().(ed25519.PublicKey)))
	fmt.Println("\nStore this API token securely — it cannot be retrieved later.")
}
