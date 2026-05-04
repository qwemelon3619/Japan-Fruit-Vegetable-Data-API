package grpc

import (
	"fmt"
	"log/slog"
	"net"

	japanapiv1 "japan_data_project/proto/japanapi/v1"

	"japan_data_project/internal/app/api/handler/monitoring"
	v1svc "japan_data_project/internal/app/api/handler/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server wraps the gRPC server and its dependencies.
type Server struct {
	grpcServer *grpc.Server
	port       int
	logger     *slog.Logger
}

// New creates a new gRPC server with all services registered.
func New(v1Service *v1svc.Service, monSvc *monitoring.Service, port int, logger *slog.Logger) *Server {
	s := &Server{
		port:   port,
		logger: logger,
	}

	// Create gRPC server with interceptors
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			UnaryRecoveryInterceptor(),
			UnaryMetricsInterceptor(monSvc),
			UnaryLoggingInterceptor(logger),
		),
	)

	// Register services
	japanapiv1.RegisterCoverageServiceServer(grpcSrv, &coverageService{svc: v1Service})
	japanapiv1.RegisterDimensionServiceServer(grpcSrv, &dimensionService{svc: v1Service})
	japanapiv1.RegisterPriceServiceServer(grpcSrv, &priceService{svc: v1Service})
	japanapiv1.RegisterSystemServiceServer(grpcSrv, &systemService{svc: v1Service})
	japanapiv1.RegisterAnalysisServiceServer(grpcSrv, &analysisService{svc: v1Service})
	japanapiv1.RegisterIngestionServiceServer(grpcSrv, &ingestionService{svc: v1Service})

	// Enable reflection for grpcurl and debugging
	reflection.Register(grpcSrv)

	s.grpcServer = grpcSrv
	return s
}

// GRPCServer returns the underlying *grpc.Server (used for testing with bufconn).
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Start starts the gRPC server on the configured port. Blocks until shutdown.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("gRPC listen failed on %s: %w", addr, err)
	}
	s.logger.Info("gRPC server started", "address", addr)
	return s.grpcServer.Serve(lis)
}

// GracefulStop gracefully stops the gRPC server.
func (s *Server) GracefulStop() {
	s.logger.Info("gRPC server shutting down")
	s.grpcServer.GracefulStop()
}
