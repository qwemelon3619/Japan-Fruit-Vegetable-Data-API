package grpc

import (
	"context"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	v1svc "japan_data_project/internal/app/api/handler/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// analysisService implements the AnalysisService gRPC server.
type analysisService struct {
	japanapiv1.UnimplementedAnalysisServiceServer
	svc *v1svc.Service
}

// CompareMarkets returns market comparison data for a given date and item.
func (s *analysisService) CompareMarkets(ctx context.Context, req *japanapiv1.CompareMarketsRequest) (*japanapiv1.CompareMarketsResponse, error) {
	if req.GetDate() == "" {
		return nil, status.Error(codes.InvalidArgument, "date is required")
	}
	if req.GetItemCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "item_code is required")
	}

	metric := req.GetMetric()
	if metric == "" {
		metric = "price_mid"
	}
	order := req.GetOrder()

	result, err := s.svc.GetCompareMarkets(ctx, req.GetDate(), req.GetItemCode(), metric, order)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "compare markets query failed: %v", err)
	}

	pbRows := make([]*japanapiv1.CompareMarketRow, 0, len(result.Rows))
	for _, r := range result.Rows {
		pbRow := &japanapiv1.CompareMarketRow{
			MarketCode: r.MarketCode,
			MarketName: r.MarketName,
			RowsCount:  r.RowsCount,
		}
		if r.MetricValue != nil {
			pbRow.MetricValue = r.MetricValue
		}
		pbRows = append(pbRows, pbRow)
	}

	return &japanapiv1.CompareMarketsResponse{
		Data:   pbRows,
		Metric: result.Metric,
		Total:  int64(result.Total),
	}, nil
}

// RankItems returns ranked items for a given date.
func (s *analysisService) RankItems(ctx context.Context, req *japanapiv1.RankItemsRequest) (*japanapiv1.RankItemsResponse, error) {
	if req.GetDate() == "" {
		return nil, status.Error(codes.InvalidArgument, "date is required")
	}

	metric := req.GetMetric()
	if metric == "" {
		metric = "arrival"
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 50
	}

	result, err := s.svc.GetRankItems(ctx, req.GetDate(), metric, req.GetMarketCode(), req.GetOrder(), limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rank items query failed: %v", err)
	}

	pbRows := make([]*japanapiv1.RankingItemRow, 0, len(result.Rows))
	for _, r := range result.Rows {
		pbRow := &japanapiv1.RankingItemRow{
			ItemCode:  r.ItemCode,
			ItemName:  r.ItemName,
			RowsCount: r.RowsCount,
		}
		if r.MetricValue != nil {
			pbRow.MetricValue = r.MetricValue
		}
		pbRows = append(pbRows, pbRow)
	}

	return &japanapiv1.RankItemsResponse{
		Data:   pbRows,
		Metric: result.Metric,
		Limit:  int32(result.Limit),
		Total:  int64(result.Total),
	}, nil
}

// compile-time interface check
var _ japanapiv1.AnalysisServiceServer = (*analysisService)(nil)
