package query

import (
	"context"
)

// GetCompareMarkets returns market comparison data for a given date and item.
func (s *Service) GetCompareMarkets(ctx context.Context, date, itemCode, metric, order string) (*CompareMarketsResult, error) {
	metricExpr := "AVG(f.price_mid_yen)::float8"
	switch metric {
	case "price_mid":
		metricExpr = "AVG(f.price_mid_yen)::float8"
	case "arrival":
		metricExpr = "SUM(f.arrival_ton)::float8"
	default:
		return &CompareMarketsResult{Rows: []CompareMarketRow{}, Metric: metric}, nil
	}
	ord := parseOrder(order)

	ids, err := s.resolveCodeIDs(ctx, itemCode, "", "")
	if err != nil {
		return nil, err
	}
	if ids == nil || ids.ItemID == nil {
		return &CompareMarketsResult{Rows: []CompareMarketRow{}, Metric: metric}, nil
	}

	query := `
		SELECT
			m.market_code,
			m.market_name,
			COUNT(*) AS rows_count,
			` + metricExpr + ` AS metric_value
		FROM fact_prices_daily f
		JOIN dim_market m ON m.id = f.market_id
		WHERE f.trade_date = ? AND f.item_id = ?
		GROUP BY m.market_code, m.market_name
		ORDER BY metric_value ` + ord + `, m.market_code ASC`

	rows := make([]CompareMarketRow, 0, 64)
	if err := s.observeDB("compare_markets", func() error {
		return s.db.WithContext(ctx).Raw(query, date, *ids.ItemID).Scan(&rows).Error
	}); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].MetricValue = roundFloatPtr2(rows[i].MetricValue)
	}

	return &CompareMarketsResult{Rows: rows, Metric: metric, Total: len(rows)}, nil
}

// GetRankItems returns ranked items for a given date.
func (s *Service) GetRankItems(ctx context.Context, date, metric, marketCode, order string, limit int) (*RankItemsResult, error) {
	l := ClampInt(limit, 1, 500)

	metricExpr := "SUM(f.arrival_ton)::float8"
	metricCountExpr := "COUNT(f.arrival_ton)"
	switch metric {
	case "arrival":
		metricExpr = "SUM(f.arrival_ton)::float8"
		metricCountExpr = "COUNT(f.arrival_ton)"
	case "price_mid":
		metricExpr = "AVG(f.price_mid_yen)::float8"
		metricCountExpr = "COUNT(f.price_mid_yen)"
	default:
		return &RankItemsResult{Rows: []RankingItemRow{}, Metric: metric, Limit: l}, nil
	}
	ord := parseOrder(order)

	where := "WHERE f.trade_date = ?"
	args := []any{date}
	if marketCode != "" {
		ids, err := s.resolveCodeIDs(ctx, "", marketCode, "")
		if err != nil {
			return nil, err
		}
		if ids == nil || ids.MarketID == nil {
			return &RankItemsResult{Rows: []RankingItemRow{}, Metric: metric, Limit: l}, nil
		}
		where += " AND f.market_id = ?"
		args = append(args, *ids.MarketID)
	}

	query := `
WITH ranked AS (
	SELECT
		f.item_id,
		` + metricCountExpr + ` AS rows_count,
		` + metricExpr + ` AS metric_value
	FROM fact_prices_daily f
	` + where + `
	GROUP BY f.item_id
	HAVING ` + metricCountExpr + ` > 0
	ORDER BY metric_value ` + ord + ` NULLS LAST, f.item_id ASC
	LIMIT ?
)
SELECT
	i.item_code,
	i.item_name,
	r.rows_count,
	r.metric_value
FROM ranked r
JOIN dim_item i ON i.id = r.item_id
ORDER BY r.metric_value ` + ord + ` NULLS LAST, i.item_code ASC`
	args = append(args, l)

	rows := make([]RankingItemRow, 0, l)
	if err := s.observeDB("rankings_items", func() error {
		return s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	}); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].MetricValue = roundFloatPtr2(rows[i].MetricValue)
	}

	return &RankItemsResult{Rows: rows, Metric: metric, Limit: l, Total: len(rows)}, nil
}
