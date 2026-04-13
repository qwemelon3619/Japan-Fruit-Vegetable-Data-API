package v1

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxDateRangeDays  = 366
	maxCodeLength     = 32
	maxSearchLength   = 100
	defaultRecentDays = 31
)

func buildPriceFilters(r *http.Request, includeSingleDate bool) (string, []any, *string, error) {
	q := r.URL.Query()
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	var defaultFrom *string
	date := strings.TrimSpace(q.Get("date"))
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))

	if includeSingleDate {
		if date != "" {
			if _, err := mustParseDate(date, "date"); err != nil {
				return "", nil, nil, err
			}
			clauses = append(clauses, "f.trade_date = ?")
			args = append(args, date)
		}
	}

	var fromDate, toDate *time.Time
	if from != "" {
		d, err := mustParseDate(from, "from")
		if err != nil {
			return "", nil, nil, err
		}
		fromDate = &d
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, from)
	}
	if to != "" {
		d, err := mustParseDate(to, "to")
		if err != nil {
			return "", nil, nil, err
		}
		toDate = &d
		clauses = append(clauses, "f.trade_date <= ?")
		args = append(args, to)
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return "", nil, nil, err
	}

	if date == "" && fromDate == nil && toDate == nil {
		df := time.Now().AddDate(0, 0, -defaultRecentDays).Format("2006-01-02")
		defaultFrom = &df
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, df)
	}

	if v := strings.TrimSpace(q.Get("market_code")); v != "" {
		if err := validateCode("market_code", v); err != nil {
			return "", nil, nil, err
		}
		clauses = append(clauses, "m.market_code = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("item_code")); v != "" {
		if err := validateCode("item_code", v); err != nil {
			return "", nil, nil, err
		}
		clauses = append(clauses, "i.item_code = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("origin_code")); v != "" {
		if err := validateCode("origin_code", v); err != nil {
			return "", nil, nil, err
		}
		clauses = append(clauses, "o.origin_code = ?")
		args = append(args, v)
	}

	if len(clauses) == 0 {
		return "", args, defaultFrom, nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, defaultFrom, nil
}

func buildLatestFilters(r *http.Request) (string, []any, error) {
	q := r.URL.Query()
	if strings.TrimSpace(q.Get("date")) != "" || strings.TrimSpace(q.Get("from")) != "" || strings.TrimSpace(q.Get("to")) != "" {
		return "", nil, errors.New("date, from, and to are not supported for this endpoint")
	}

	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if v := strings.TrimSpace(q.Get("market_code")); v != "" {
		if err := validateCode("market_code", v); err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "m.market_code = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("item_code")); v != "" {
		if err := validateCode("item_code", v); err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "i.item_code = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("origin_code")); v != "" {
		if err := validateCode("origin_code", v); err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "o.origin_code = ?")
		args = append(args, v)
	}

	if len(clauses) == 0 {
		return "", args, nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, nil
}

func parseOrder(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "asc") {
		return "ASC"
	}
	return "DESC"
}

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

func parseIntOrDefault(v string, d int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return d
	}
	return n
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mustParseDate(v, field string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", strings.TrimSpace(v))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s format must be YYYY-MM-DD", field)
	}
	return d, nil
}

func validateDateRange(fromDate, toDate *time.Time) error {
	if fromDate == nil || toDate == nil {
		return nil
	}
	if toDate.Before(*fromDate) {
		return errors.New("to must be greater than or equal to from")
	}
	if toDate.Sub(*fromDate) > maxDateRangeDays*24*time.Hour {
		return fmt.Errorf("date range too large: max %d days", maxDateRangeDays)
	}
	return nil
}

func validateCode(field, v string) error {
	if len(v) > maxCodeLength {
		return fmt.Errorf("%s too long: max %d chars", field, maxCodeLength)
	}
	return nil
}

func validateSearchQuery(q string) error {
	if len(q) > maxSearchLength {
		return fmt.Errorf("q too long: max %d chars", maxSearchLength)
	}
	return nil
}
