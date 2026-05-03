package v1

import (
	"context"
	"net/http"
	"strings"
	"time"
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
	if err := validateSearchQuery(q); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	result, err := h.ListDimensions(ctx, DimensionQuery{
		Kind:   kind,
		Limit:  parseIntOrDefault(r.URL.Query().Get("limit"), 50),
		Offset: parseIntOrDefault(r.URL.Query().Get("offset"), 0),
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
