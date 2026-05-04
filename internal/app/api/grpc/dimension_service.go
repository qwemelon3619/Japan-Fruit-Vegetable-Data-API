package grpc

import (
	"context"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	"japan_data_project/internal/app/query"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// dimensionService implements the DimensionService gRPC server.
type dimensionService struct {
	japanapiv1.UnimplementedDimensionServiceServer
	q *query.Service
}

func (s *dimensionService) ListMarkets(ctx context.Context, req *japanapiv1.ListMarketsRequest) (*japanapiv1.ListMarketsResponse, error) {
	pbRows, limit, offset, total, err := s.queryDimensions(ctx, "market", req.GetFilter())
	if err != nil {
		return nil, err
	}
	return &japanapiv1.ListMarketsResponse{
		Data: pbRows, Limit: limit, Offset: offset, Total: total,
	}, nil
}

func (s *dimensionService) ListItems(ctx context.Context, req *japanapiv1.ListItemsRequest) (*japanapiv1.ListItemsResponse, error) {
	pbRows, limit, offset, total, err := s.queryDimensions(ctx, "item", req.GetFilter())
	if err != nil {
		return nil, err
	}
	return &japanapiv1.ListItemsResponse{
		Data: pbRows, Limit: limit, Offset: offset, Total: total,
	}, nil
}

func (s *dimensionService) ListOrigins(ctx context.Context, req *japanapiv1.ListOriginsRequest) (*japanapiv1.ListOriginsResponse, error) {
	pbRows, limit, offset, total, err := s.queryDimensions(ctx, "origin", req.GetFilter())
	if err != nil {
		return nil, err
	}
	return &japanapiv1.ListOriginsResponse{
		Data: pbRows, Limit: limit, Offset: offset, Total: total,
	}, nil
}

// queryDimensions is a shared helper that returns protobuf rows and pagination metadata.
func (s *dimensionService) queryDimensions(ctx context.Context, kind string, filter *japanapiv1.DimensionFilter) ([]*japanapiv1.DimensionRow, int32, int32, int64, error) {
	limit := int32(50)
	offset := int32(0)
	q := ""
	sort := ""
	order := ""

	if filter != nil {
		limit = filter.GetLimit()
		offset = filter.GetOffset()
		q = filter.GetQ()
		sort = filter.GetSort()
		order = filter.GetOrder()
	}

	result, err := s.q.ListDimensions(ctx, query.DimensionQuery{
		Kind:   kind,
		Limit:  int(limit),
		Offset: int(offset),
		Q:      q,
		Sort:   sort,
		Order:  order,
	})
	if err != nil {
		return nil, 0, 0, 0, status.Errorf(codes.Internal, "%s query failed: %v", kind, err)
	}

	pbRows := make([]*japanapiv1.DimensionRow, 0, len(result.Rows))
	for _, r := range result.Rows {
		pbRows = append(pbRows, &japanapiv1.DimensionRow{
			Id:   uint32(r.ID),
			Code: r.Code,
			Name: r.Name,
		})
	}

	return pbRows, int32(result.Limit), int32(result.Off), result.Total, nil
}
