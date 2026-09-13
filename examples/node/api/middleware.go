package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/fbscommunity/barcode-federation/examples/node/model"
)

type contextKey string

const actorContextKey contextKey = "actor"

func ActorFromContext(ctx context.Context) *model.Actor {
	if a, ok := ctx.Value(actorContextKey).(*model.Actor); ok {
		return a
	}
	return nil
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			Unauthorized(w)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		actor, err := s.Store.GetActorByToken(token)
		if err != nil {
			Unauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), actorContextKey, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireCapability(capability string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := ActorFromContext(r.Context())
		if actor == nil {
			Unauthorized(w)
			return
		}
		if !actor.HasCapability(capability) {
			Forbidden(w, capability)
			return
		}
		handler(w, r)
	}
}
