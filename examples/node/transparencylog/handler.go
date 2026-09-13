package transparencylog

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

// Handler exposes the transparency log HTTP endpoints per FBS Section 4.7.2.
type Handler struct {
	Log *TransparencyLog
}

// HandleAddBadge handles POST /fbs-log/v1/add-badge.
func (h *Handler) HandleAddBadge(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req struct {
		Badge       json.RawMessage `json:"badge"`
		SubmitterID string          `json:"submitterId"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(req.Badge) == 0 {
		writeError(w, http.StatusBadRequest, "badge field is required")
		return
	}

	sct, err := h.Log.AddBadge(req.Badge, req.SubmitterID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sct)
}

// HandleGetEntries handles GET /fbs-log/v1/get-entries.
func (h *Handler) HandleGetEntries(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	start, _ := strconv.Atoi(startStr)
	end, _ := strconv.Atoi(endStr)
	if end == 0 {
		end = h.Log.Metadata().TreeSize
	}

	entries, err := h.Log.GetEntries(start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// HandleGetProofByHash handles GET /fbs-log/v1/get-proof-by-hash.
func (h *Handler) HandleGetProofByHash(w http.ResponseWriter, r *http.Request) {
	badgeHash := r.URL.Query().Get("hash")
	if badgeHash == "" {
		writeError(w, http.StatusBadRequest, "hash parameter required")
		return
	}

	proof, leafIndex, err := h.Log.GetProofByHash(badgeHash)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	meta := h.Log.Metadata()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"leafIndex": leafIndex,
		"treeSize":  meta.TreeSize,
		"rootHash":  meta.CurrentRootHash,
		"proof":     proof,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
