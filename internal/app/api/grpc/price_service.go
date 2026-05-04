package grpc

import (
	"context"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	"japan_data_project/internal/app/query"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// priceService implements the PriceService gRPC server.
type priceService struct {
	japanapiv1.UnimplementedPriceServiceServer
	q *query.Service
}

// GetDailyPrices returns paginated daily price rows.
func (s *priceService) GetDailyPrices(ctx context.Context, req *japanapiv1.GetDailyPricesRequest) (*japanapiv1.GetDailyPricesResponse, error) {
	pf := req.GetFilter()
	if pf == nil || pf.GetItemCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "item_code is required")
	}

	dr := pf.GetDateRange()
	result, err := s.q.GetDailyPrices(ctx, query.PriceFilter{
		ItemCode:   pf.GetItemCode(),
		MarketCode: pf.GetMarketCode(),
		OriginCode: pf.GetOriginCode(),
		DateRange: query.DateRange{
			Date: dr.GetDate(),
			From: dr.GetFrom(),
			To:   dr.GetTo(),
		},
		Limit:  int(pf.GetLimit()),
		Offset: int(pf.GetOffset()),
		Sort:   pf.GetSort(),
		Order:  pf.GetOrder(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "daily prices query failed: %v", err)
	}

	resp := &japanapiv1.GetDailyPricesResponse{
		Data:   toDailyPriceRows(result.Rows),
		Limit:  int32(result.Limit),
		Offset: int32(result.Offset),
	}
	if total := int64(len(result.Rows)); total > 0 {
		resp.Total = &total
	}
	if result.DefaultFrom != nil {
		resp.DefaultFrom = result.DefaultFrom
		wd := int32(result.DefaultWindowDays)
		resp.DefaultWindowDays = &wd
	}
	return resp, nil
}

// GetLatestPrices returns prices for the latest available trade date.
func (s *priceService) GetLatestPrices(ctx context.Context, req *japanapiv1.GetLatestPricesRequest) (*japanapiv1.GetLatestPricesResponse, error) {
	if req.GetItemCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "item_code is required")
	}

	result, err := s.q.GetLatestPrices(ctx, query.PriceFilter{
		ItemCode:   req.GetItemCode(),
		MarketCode: req.GetMarketCode(),
		OriginCode: req.GetOriginCode(),
		Limit:      int(req.GetLimit()),
		Offset:     int(req.GetOffset()),
		Sort:       req.GetSort(),
		Order:      req.GetOrder(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "latest prices query failed: %v", err)
	}

	resp := &japanapiv1.GetLatestPricesResponse{
		Data:            toDailyPriceRows(result.Rows),
		LatestTradeDate: strVal(result.LatestTradeDate),
		Limit:           int32(result.Limit),
		Offset:          int32(result.Offset),
		Total:           result.Total,
	}
	return resp, nil
}

// GetPriceTrend returns daily aggregated trend data.
func (s *priceService) GetPriceTrend(ctx context.Context, req *japanapiv1.GetPriceTrendRequest) (*japanapiv1.GetPriceTrendResponse, error) {
	if req.GetItemCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "item_code is required")
	}

	dr := req.GetDateRange()
	result, err := s.q.GetPriceTrend(ctx, query.PriceFilter{
		ItemCode:   req.GetItemCode(),
		MarketCode: req.GetMarketCode(),
		OriginCode: req.GetOriginCode(),
		DateRange: query.DateRange{
			From: dr.GetFrom(),
			To:   dr.GetTo(),
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "price trend query failed: %v", err)
	}

	resp := &japanapiv1.GetPriceTrendResponse{
		Data: toTrendRows(result.Rows),
	}
	if total := int64(len(result.Rows)); total > 0 {
		resp.Total = &total
	}
	if result.DefaultFrom != nil {
		resp.DefaultFrom = result.DefaultFrom
		wd := int32(result.DefaultWindowDays)
		resp.DefaultWindowDays = &wd
	}
	return resp, nil
}

// GetPriceSummary returns aggregated summary data grouped by period.
func (s *priceService) GetPriceSummary(ctx context.Context, req *japanapiv1.GetPriceSummaryRequest) (*japanapiv1.GetPriceSummaryResponse, error) {
	if req.GetItemCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "item_code is required")
	}

	groupBy := req.GetGroupBy()
	if groupBy == "" {
		groupBy = "day"
	}

	dr := req.GetDateRange()
	result, err := s.q.GetPriceSummary(ctx, query.PriceFilter{
		ItemCode:   req.GetItemCode(),
		MarketCode: req.GetMarketCode(),
		OriginCode: req.GetOriginCode(),
		DateRange: query.DateRange{
			From: dr.GetFrom(),
			To:   dr.GetTo(),
		},
	}, groupBy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "price summary query failed: %v", err)
	}

	resp := &japanapiv1.GetPriceSummaryResponse{
		Data: toSummaryRows(result.Rows),
	}
	if total := int64(len(result.Rows)); total > 0 {
		resp.Total = &total
	}
	if result.DefaultFrom != nil {
		resp.DefaultFrom = result.DefaultFrom
		wd := int32(result.DefaultWindowDays)
		resp.DefaultWindowDays = &wd
	}
	return resp, nil
}

// ---- Conversion helpers ----

func toDailyPriceRows(rows []query.DailyRow) []*japanapiv1.DailyPriceRow {
	out := make([]*japanapiv1.DailyPriceRow, len(rows))
	for i, r := range rows {
		out[i] = toDailyPriceRow(r)
	}
	return out
}

func toDailyPriceRow(r query.DailyRow) *japanapiv1.DailyPriceRow {
	pb := &japanapiv1.DailyPriceRow{
		TradeDate:   r.TradeDate,
		WeekdayJa:   r.WeekdayJA,
		MarketCode:  r.MarketCode,
		MarketName:  r.MarketName,
		ItemCode:    r.ItemCode,
		ItemName:    r.ItemName,
		OriginCode:  r.OriginCode,
		OriginName:  r.OriginName,
		Grade:       r.Grade,
		Class:       r.Class,
		ProductName: r.ProductName,
		UnitWeight:  r.UnitWeight,
		PriceHighYen: int32PtrOrNil(r.PriceHigh),
		PriceMidYen:  int32PtrOrNil(r.PriceMid),
		PriceLowYen:  int32PtrOrNil(r.PriceLow),
		SourceFile:   r.SourceFile,
		SourceRowNo:  int32(r.SourceRowNo),
	}
	if r.ItemTotal != nil {
		pb.ItemTotalTon = float64Ptr(*r.ItemTotal)
	}
	if r.ArrivalTon != nil {
		pb.ArrivalTon = float64Ptr(*r.ArrivalTon)
	}
	if r.TrendLabel != nil {
		pb.TrendLabel = r.TrendLabel
	}
	return pb
}

func int32PtrOrNil(v *int) *int32 {
	if v == nil {
		return nil
	}
	val := int32(*v)
	return &val
}

func float64Ptr(v float64) *float64 {
	return &v
}

func toTrendRows(rows []query.TrendRow) []*japanapiv1.TrendRow {
	out := make([]*japanapiv1.TrendRow, len(rows))
	for i, r := range rows {
		out[i] = toTrendRow(r)
	}
	return out
}

func toTrendRow(r query.TrendRow) *japanapiv1.TrendRow {
	pb := &japanapiv1.TrendRow{
		TradeDate: r.TradeDate,
		RowsCount: r.RowsCount,
	}
	if r.AvgPriceMid != nil {
		pb.AvgPriceMidYen = float64Ptr(*r.AvgPriceMid)
	}
	if r.MaxPriceMid != nil {
		val := int32(*r.MaxPriceMid)
		pb.MaxPriceMidYen = &val
	}
	if r.MinPriceMid != nil {
		val := int32(*r.MinPriceMid)
		pb.MinPriceMidYen = &val
	}
	if r.ArrivalTon != nil {
		pb.ArrivalTonSum = float64Ptr(*r.ArrivalTon)
	}
	return pb
}

func toSummaryRows(rows []query.SummaryRow) []*japanapiv1.SummaryRow {
	out := make([]*japanapiv1.SummaryRow, len(rows))
	for i, r := range rows {
		out[i] = toSummaryRow(r)
	}
	return out
}

func toSummaryRow(r query.SummaryRow) *japanapiv1.SummaryRow {
	pb := &japanapiv1.SummaryRow{
		Period:    r.Period,
		RowsCount: r.RowsCount,
	}
	if r.AvgPriceMid != nil {
		pb.AvgPriceMidYen = float64Ptr(*r.AvgPriceMid)
	}
	if r.ArrivalTon != nil {
		pb.ArrivalTonSum = float64Ptr(*r.ArrivalTon)
	}
	return pb
}
