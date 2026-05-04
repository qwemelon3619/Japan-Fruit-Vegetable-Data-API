package query

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	maxDateRangeDays  = 366
	defaultRecentDays = 31
)

// parseOrder returns "ASC" or "DESC" based on the input string.
func parseOrder(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "asc") {
		return "ASC"
	}
	return "DESC"
}

// parseDailySortColumn maps a column key to a gorm column expression.
func parseDailySortColumn(sort string) string {
	switch sort {
	case "f.trade_date":
		return "trade_date"
	case "m.market_code":
		return "market_code"
	case "i.item_code":
		return "item_code"
	case "o.origin_code":
		return "origin_code"
	case "f.price_mid_yen":
		return "price_mid_yen"
	case "f.arrival_ton":
		return "arrival_ton"
	default:
		return "trade_date"
	}
}

// parseDailySort maps a user-facing sort key to a database column.
func parseDailySort(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "trade_date":
		return "f.trade_date"
	case "market_code":
		return "m.market_code"
	case "item_code":
		return "i.item_code"
	case "origin_code":
		return "o.origin_code"
	case "price_mid":
		return "f.price_mid_yen"
	case "arrival":
		return "f.arrival_ton"
	default:
		return "f.trade_date"
	}
}

// parseDimensionSort maps a dimension sort key to a database column.
func parseDimensionSort(kind, v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch kind {
	case "market":
		switch s {
		case "code":
			return "market_code"
		case "id":
			return "id"
		default:
			return "market_name"
		}
	case "item":
		switch s {
		case "code":
			return "item_code"
		case "id":
			return "id"
		default:
			return "item_name"
		}
	case "origin":
		switch s {
		case "code":
			return "origin_code"
		case "id":
			return "id"
		default:
			return "origin_name"
		}
	default:
		return "id"
	}
}

// ParseIntOrDefault parses an int from a string, returning a default on failure.
func ParseIntOrDefault(v string, d int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return d
	}
	return n
}

// ClampInt restricts a value to [minV, maxV].
func ClampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

// maxInt returns the larger of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// roundFloatPtr2 rounds a *float64 to 2 decimal places.
func roundFloatPtr2(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := math.Round((*v)*100) / 100
	return &r
}

// joinClauses joins SQL clauses with a separator.
func joinClauses(clauses []string, sep string) string {
	if len(clauses) == 0 {
		return ""
	}
	result := clauses[0]
	for _, c := range clauses[1:] {
		result += sep + c
	}
	return result
}

// mustParseDate parses a YYYY-MM-DD date string.
func mustParseDate(v, field string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", strings.TrimSpace(v))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s format must be YYYY-MM-DD", field)
	}
	return d, nil
}

// validateDateRange checks that the date range is valid and within limits.
func validateDateRange(fromDate, toDate *time.Time) error {
	if fromDate == nil || toDate == nil {
		return nil
	}
	if toDate.Before(*fromDate) {
		return fmt.Errorf("to must be greater than or equal to from")
	}
	if toDate.Sub(*fromDate) > maxDateRangeDays*24*time.Hour {
		return fmt.Errorf("date range too large: max %d days", maxDateRangeDays)
	}
	return nil
}

// buildFactDateClauses builds date-related SQL clauses from a PriceFilter.
func buildFactDateClauses(filter PriceFilter, includeSingleDate bool) ([]string, []any, *string, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	var defaultFrom *string

	dr := filter.DateRange
	if includeSingleDate && dr.Date != "" {
		if _, err := mustParseDate(dr.Date, "date"); err != nil {
			return nil, nil, nil, err
		}
		clauses = append(clauses, "f.trade_date = ?")
		args = append(args, dr.Date)
	}

	var fromDate, toDate *time.Time
	if dr.From != "" {
		d, err := mustParseDate(dr.From, "from")
		if err != nil {
			return nil, nil, nil, err
		}
		fromDate = &d
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, dr.From)
	}
	if dr.To != "" {
		d, err := mustParseDate(dr.To, "to")
		if err != nil {
			return nil, nil, nil, err
		}
		toDate = &d
		clauses = append(clauses, "f.trade_date <= ?")
		args = append(args, dr.To)
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return nil, nil, nil, err
	}

	if dr.Date == "" && fromDate == nil && toDate == nil {
		df := time.Now().AddDate(0, 0, -defaultRecentDays).Format("2006-01-02")
		defaultFrom = &df
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, df)
	}

	return clauses, args, defaultFrom, nil
}

// latestFilterIDs holds resolved dimension IDs for price queries.
type latestFilterIDs struct {
	ItemID   *uint
	MarketID *uint
	OriginID *uint
}
