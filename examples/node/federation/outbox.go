package federation

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/fbscommunity/barcode-federation/examples/node/api"
	"github.com/fbscommunity/barcode-federation/examples/node/model"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
)

type OutboxHandler struct {
	Store       *store.Store
	NodeBaseURL string
}

func (h *OutboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 100

	items, total, err := h.Store.GetOutboxPage(page, pageSize)
	if err != nil {
		api.InternalError(w, err)
		return
	}

	resp := model.OutboxPage{
		FBS:        "1.0",
		Type:       "OutboxPage",
		NodeID:     h.NodeBaseURL,
		TotalItems: total,
		Page:       page,
		PageSize:   pageSize,
		Items:      items,
	}

	if page*pageSize < total {
		resp.NextPage = fmt.Sprintf("%s/federation/outbox?page=%d", h.NodeBaseURL, page+1)
	}
	if page > 1 {
		resp.PreviousPage = fmt.Sprintf("%s/federation/outbox?page=%d", h.NodeBaseURL, page-1)
	}

	api.JSON(w, http.StatusOK, resp)
}
