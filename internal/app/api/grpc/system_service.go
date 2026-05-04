package grpc

import (
	"context"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	v1svc "japan_data_project/internal/app/api/handler/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// systemService implements the SystemService gRPC server.
type systemService struct {
	japanapiv1.UnimplementedSystemServiceServer
	svc *v1svc.Service
}

// GetHealth returns the health status of the service.
// This is a lightweight check that does not require database connectivity.
func (s *systemService) GetHealth(ctx context.Context, req *japanapiv1.GetHealthRequest) (*japanapiv1.GetHealthResponse, error) {
	return &japanapiv1.GetHealthResponse{
		Status: "ok",
	}, nil
}

// GetReady checks whether the service is ready to serve requests.
// This performs a database ping to verify connectivity.
func (s *systemService) GetReady(ctx context.Context, req *japanapiv1.GetReadyRequest) (*japanapiv1.GetReadyResponse, error) {
	if err := s.svc.Ready(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "service not ready: %v", err)
	}

	return &japanapiv1.GetReadyResponse{
		Ready:    true,
		Database: "connected",
	}, nil
}
