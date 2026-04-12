package v1

import (
	"context"
	"net/http"
	"strings"
	"time"

	"japan_data_project/internal/domain/model"
)

func (h *Service) handleMarkets(w http.ResponseWriter, r *http.Request) {
	h.handleDimensionList(w, r, "market")
}

func (h *Service) handleItems(w http.ResponseWriter, r *http.Request) {
	h.handleDimensionList(w, r, "item")
}

func (h *Service) handleOrigins(w http.ResponseWriter, r *http.Request) {
	h.handleDimensionList(w, r, "origin")
}

func (h *Service) handleDimensionList(w http.ResponseWriter, r *http.Request, kind string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 50), 1, 500)
	offset := maxInt(parseIntOrDefault(r.URL.Query().Get("offset"), 0), 0)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	order := parseOrder(r.URL.Query().Get("order"))
	sort := parseDimensionSort(kind, r.URL.Query().Get("sort"))
	if err := validateSearchQuery(q); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	rows := make([]dimensionRow, 0, limit)
	var total int64
	query := h.db.WithContext(ctx)

	switch kind {
	case "market":
		if q != "" {
			query = query.Where("market_code LIKE ? OR market_name LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		if err := h.observeDB("markets_count", func() error { return query.Model(&model.DimMarket{}).Count(&total).Error }); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
			return
		}
		var data []model.DimMarket
		if err := h.observeDB("markets_list", func() error {
			return query.Model(&model.DimMarket{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
			return
		}
		for _, d := range data {
			rows = append(rows, dimensionRow{ID: d.ID, Code: d.MarketCode, Name: d.MarketName})
		}
	case "item":
		if q != "" {
			query = query.Where("item_code LIKE ? OR item_name LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		if err := h.observeDB("items_count", func() error { return query.Model(&model.DimItem{}).Count(&total).Error }); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
			return
		}
		var data []model.DimItem
		if err := h.observeDB("items_list", func() error {
			return query.Model(&model.DimItem{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
			return
		}
		for _, d := range data {
			rows = append(rows, dimensionRow{ID: d.ID, Code: d.ItemCode, Name: d.ItemName})
		}
	case "origin":
		if q != "" {
			query = query.Where("origin_code LIKE ? OR origin_name LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		if err := h.observeDB("origins_count", func() error { return query.Model(&model.DimOrigin{}).Count(&total).Error }); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
			return
		}
		var data []model.DimOrigin
		if err := h.observeDB("origins_list", func() error {
			return query.Model(&model.DimOrigin{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
			return
		}
		for _, d := range data {
			rows = append(rows, dimensionRow{ID: d.ID, Code: d.OriginCode, Name: d.OriginName})
		}
	default:
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid dimension kind")
		return
	}

	writeOK(w, rows, apiMeta{
		"limit":  limit,
		"offset": offset,
		"total":  total,
	})
}
