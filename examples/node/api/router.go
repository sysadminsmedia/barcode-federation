package api

import (
	"crypto/ed25519"
	"log/slog"
	"net/http"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/config"
	"github.com/fbscommunity/barcode-federation/examples/node/quorum"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
	"github.com/fbscommunity/barcode-federation/examples/node/transparencylog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Dispatcher is an interface for federation dispatch to avoid import cycles.
type FederationDispatcher interface {
	DispatchPublish(rec interface{})
	DispatchRetract(recordID, cbi string)
	DispatchClaimPublish(claim interface{})
	DispatchActivity(activityType string, object interface{})
}

type Server struct {
	Config                  *config.Config
	Store                   *store.Store
	NodeKey                 ed25519.PrivateKey
	Router                  chi.Router
	Dispatcher              FederationDispatcher
	FederationInboxHandler  http.Handler
	FederationOutboxHandler http.Handler
	QuorumCoordinator       *quorum.Coordinator
	TransparencyLog         *transparencylog.TransparencyLog
}

func NewServer(cfg *config.Config, s *store.Store, nodeKey ed25519.PrivateKey) *Server {
	srv := &Server{
		Config:  cfg,
		Store:   s,
		NodeKey: nodeKey,
	}
	srv.Router = srv.buildRouter()
	return srv
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Well-known endpoints (unauthenticated)
	r.Get("/.well-known/fbs/meta", s.handleMeta)
	r.Get("/.well-known/fbs/actors/{actorID}", s.handleActorProfile)
	r.Get("/.well-known/fbs/webrecord", s.handleWebRecord)
	r.Get("/.well-known/fbs/peers", s.handlePeerList)

	// Public API (unauthenticated)
	r.Get("/api/v1/barcodes", s.handleSearchBarcodes)
	r.Get("/api/v1/barcodes/{symbology}/{value}", s.handleLookupBarcode)
	r.Get("/api/v1/records/{recordID}", s.handleGetRecord)
	r.Get("/api/v1/actors/{actorID}/claims", s.handleListClaims)
	r.Get("/api/v1/successions", s.handleListSuccessions)
	r.Get("/api/v1/records/{recordID}/custody", s.handleGetCustodyChain)
	r.Get("/api/v1/related/{symbology}/{value}", s.handleGetRelatedRecords)

	// Authenticated API
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Post("/api/v1/records", s.requireCapability("record:submit", s.handleSubmitRecord))
		r.Put("/api/v1/records/{recordID}", s.requireCapability("record:update:own", s.handleUpdateRecord))

		r.Post("/api/v1/bulk-import", s.requireCapability("record:submit:bulk", s.handleBulkImport))
		r.Get("/api/v1/bulk-import/{jobID}", s.handleBulkJobStatus)

		r.Post("/api/v1/claims", s.requireCapability("namespace:claim", s.handleSubmitClaim))

		r.Post("/api/v1/subscriptions", s.requireCapability("subscription:create", s.handleCreateSubscription))

		r.Post("/api/v1/verification/quorum", s.handleStartVerification)
		r.Get("/api/v1/verification/quorum/{sessionID}", s.handleVerificationStatus)

		r.Post("/api/v1/custody/transfer", s.requireCapability("record:delegate", s.handleCustodyTransfer))
		r.Post("/api/v1/custody/accept", s.handleCustodyAccept)
	})

	// Federation endpoints (handlers set after construction in main.go)
	r.Post("/federation/inbox", func(w http.ResponseWriter, req *http.Request) {
		if s.FederationInboxHandler != nil {
			s.FederationInboxHandler.ServeHTTP(w, req)
		} else {
			Error(w, http.StatusServiceUnavailable, "unavailable", "federation not configured")
		}
	})
	r.Get("/federation/outbox", func(w http.ResponseWriter, req *http.Request) {
		if s.FederationOutboxHandler != nil {
			s.FederationOutboxHandler.ServeHTTP(w, req)
		} else {
			Error(w, http.StatusServiceUnavailable, "unavailable", "federation not configured")
		}
	})

	// Transparency log endpoints (Section 4.7.2)
	// These are registered lazily since TransparencyLog may be set after construction.
	r.Get("/fbs-log/v1/metadata", func(w http.ResponseWriter, req *http.Request) {
		if s.TransparencyLog == nil {
			Error(w, http.StatusServiceUnavailable, "unavailable", "transparency log not configured")
			return
		}
		JSON(w, http.StatusOK, s.TransparencyLog.Metadata())
	})
	r.Get("/fbs-log/v1/get-sth", func(w http.ResponseWriter, req *http.Request) {
		if s.TransparencyLog == nil {
			Error(w, http.StatusServiceUnavailable, "unavailable", "transparency log not configured")
			return
		}
		JSON(w, http.StatusOK, s.TransparencyLog.GetSignedTreeHead())
	})

	// Remaining transparency log endpoints mounted via handler
	r.Route("/fbs-log/v1", func(sub chi.Router) {
		sub.Post("/add-badge", func(w http.ResponseWriter, req *http.Request) {
			if s.TransparencyLog == nil {
				Error(w, http.StatusServiceUnavailable, "unavailable", "transparency log not configured")
				return
			}
			h := &transparencylog.Handler{Log: s.TransparencyLog}
			h.HandleAddBadge(w, req)
		})
		sub.Get("/get-entries", func(w http.ResponseWriter, req *http.Request) {
			if s.TransparencyLog == nil {
				Error(w, http.StatusServiceUnavailable, "unavailable", "transparency log not configured")
				return
			}
			h := &transparencylog.Handler{Log: s.TransparencyLog}
			h.HandleGetEntries(w, req)
		})
		sub.Get("/get-proof-by-hash", func(w http.ResponseWriter, req *http.Request) {
			if s.TransparencyLog == nil {
				Error(w, http.StatusServiceUnavailable, "unavailable", "transparency log not configured")
				return
			}
			h := &transparencylog.Handler{Log: s.TransparencyLog}
			h.HandleGetProofByHash(w, req)
		})
	})

	return r
}

func (s *Server) NodeBaseURL() string {
	return "https://" + s.Config.Node.Host
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start).String(),
		)
	})
}
