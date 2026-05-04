package grpc

import (
	"context"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	"japan_data_project/internal/app/query"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// coverageService implements the CoverageService gRPC server.
type coverageService struct {
	japanapiv1.UnimplementedCoverageServiceServer
	q *query.Service
}

// GetCoverage returns coverage information.
func (s *coverageService) GetCoverage(ctx context.Context, req *japanapiv1.GetCoverageRequest) (*japanapiv1.GetCoverageResponse, error) {
	result, err := s.q.GetCoverage(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "coverage query failed: %v", err)
	}

	resp := &japanapiv1.GetCoverageResponse{
		EarliestTradeDate: strVal(result.EarliestTradeDate),
		LatestTradeDate:   strVal(result.LatestTradeDate),
		FactRowsTotal:     result.FactRowsTotal,
	}

	if result.LastIngestionRunID != nil {
		id := uint32(*result.LastIngestionRunID)
		resp.LastIngestionRunId = &id
	}
	if result.LastIngestionStatus != nil {
		resp.LastIngestionStatus = result.LastIngestionStatus
	}
	if result.LastIngestionRunType != nil {
		resp.LastIngestionRunType = result.LastIngestionRunType
	}
	if result.LastIngestionAt != nil {
		formatted := result.LastIngestionAt.Format("2006-01-02T15:04:05Z")
		resp.LastIngestionFinishedAt = &formatted
	}

	return resp, nil
}

// strVal returns the string value or empty string for nil.
func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
