package v1

import (
	"context"
	"net/http"
	"strings"
	"time"

	"japan_data_project/internal/app/query"
)

func (h *Service) handleMarkets(w http.ResponseWriter, r *http.Request) {
	h.handleDimensionListHTTP(w, r, "market")
}

func (h *Service) handleItems(w http.ResponseWriter, r *http.Request) {
	h.handleDimensionListHTTP(w, r, "item")
}

func (h *Service) handleOrigins(w http.ResponseWriter, r *http.Request) {
	h.handleDimensionListHTTP(w, r, "origin")
}

func (h *Service) handleDimensionListHTTP(w http.ResponseWriter, r *http.Request, kind string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 100 {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "q too long: max 100 chars")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	result, err := h.q.ListDimensions(ctx, query.DimensionQuery{
		Kind:   kind,
		Limit:  queryParseIntOrDefault(r.URL.Query().Get("limit"), 50),
		Offset: queryParseIntOrDefault(r.URL.Query().Get("offset"), 0),
		Q:      q,
		Sort:   r.URL.Query().Get("sort"),
		Order:  r.URL.Query().Get("order"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	writeOK(w, result.Rows, apiMeta{
		"limit":  result.Limit,
		"offset": result.Off,
		"total":  result.Total,
	})
}
