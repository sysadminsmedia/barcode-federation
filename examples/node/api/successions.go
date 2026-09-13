package api

import (
	"net/http"
)

func (s *Server) handleListSuccessions(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	if since == "" {
		since = "1970-01-01T00:00:00Z"
	}

	decls, err := s.Store.ListSuccessionsSince(since)
	if err != nil {
		InternalError(w, err)
		return
	}

	JSON(w, http.StatusOK, decls)
}
