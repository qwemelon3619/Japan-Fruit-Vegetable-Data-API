package grpc

import (
	"context"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	"japan_data_project/internal/app/query"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// systemService implements the SystemService gRPC server.
type systemService struct {
	japanapiv1.UnimplementedSystemServiceServer
	q *query.Service
}

// GetHealth returns the health status of the service.
func (s *systemService) GetHealth(ctx context.Context, req *japanapiv1.GetHealthRequest) (*japanapiv1.GetHealthResponse, error) {
	return &japanapiv1.GetHealthResponse{
		Status: "ok",
	}, nil
}

// GetReady checks whether the service is ready to serve requests.
func (s *systemService) GetReady(ctx context.Context, req *japanapiv1.GetReadyRequest) (*japanapiv1.GetReadyResponse, error) {
	if err := s.q.Ready(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "service not ready: %v", err)
	}

	return &japanapiv1.GetReadyResponse{
		Ready:    true,
		Database: "connected",
	}, nil
}
