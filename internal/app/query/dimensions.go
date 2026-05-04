package query

import (
	"context"

	"japan_data_project/internal/domain/model"
)

// ListDimensions returns a paginated list of dimension entries.
func (s *Service) ListDimensions(ctx context.Context, q DimensionQuery) (*DimensionListResult, error) {
	limit := ClampInt(q.Limit, 1, 500)
	offset := maxInt(q.Offset, 0)
	sort := parseDimensionSort(q.Kind, q.Sort)
	order := parseOrder(q.Order)

	query := s.db.WithContext(ctx)

	switch q.Kind {
	case "market":
		if q.Q != "" {
			query = query.Where("market_code LIKE ? OR market_name LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
		}
		var total int64
		if err := s.observeDB("markets_count", func() error {
			return query.Model(&model.DimMarket{}).Count(&total).Error
		}); err != nil {
			return nil, err
		}
		var data []model.DimMarket
		if err := s.observeDB("markets_list", func() error {
			return query.Model(&model.DimMarket{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			return nil, err
		}
		rows := make([]DimensionRow, 0, len(data))
		for _, d := range data {
			rows = append(rows, DimensionRow{ID: d.ID, Code: d.MarketCode, Name: d.MarketName})
		}
		return &DimensionListResult{Rows: rows, Total: total, Limit: limit, Off: offset}, nil

	case "item":
		if q.Q != "" {
			query = query.Where("item_code LIKE ? OR item_name LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
		}
		var total int64
		if err := s.observeDB("items_count", func() error {
			return query.Model(&model.DimItem{}).Count(&total).Error
		}); err != nil {
			return nil, err
		}
		var data []model.DimItem
		if err := s.observeDB("items_list", func() error {
			return query.Model(&model.DimItem{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			return nil, err
		}
		rows := make([]DimensionRow, 0, len(data))
		for _, d := range data {
			rows = append(rows, DimensionRow{ID: d.ID, Code: d.ItemCode, Name: d.ItemName})
		}
		return &DimensionListResult{Rows: rows, Total: total, Limit: limit, Off: offset}, nil

	default: // "origin"
		if q.Q != "" {
			query = query.Where("origin_code LIKE ? OR origin_name LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
		}
		var total int64
		if err := s.observeDB("origins_count", func() error {
			return query.Model(&model.DimOrigin{}).Count(&total).Error
		}); err != nil {
			return nil, err
		}
		var data []model.DimOrigin
		if err := s.observeDB("origins_list", func() error {
			return query.Model(&model.DimOrigin{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			return nil, err
		}
		rows := make([]DimensionRow, 0, len(data))
		for _, d := range data {
			rows = append(rows, DimensionRow{ID: d.ID, Code: d.OriginCode, Name: d.OriginName})
		}
		return &DimensionListResult{Rows: rows, Total: total, Limit: limit, Off: offset}, nil
	}
}
