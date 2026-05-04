package grpc

import (
	"fmt"
	"log/slog"
	"net"

	japanapiv1 "japan_data_project/proto/japanapi/v1"

	"japan_data_project/internal/app/api/handler/monitoring"
	"japan_data_project/internal/app/query"

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
func New(qSvc *query.Service, monSvc *monitoring.Service, port int, logger *slog.Logger) *Server {
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

	// Register services — all share the same query.Service
	japanapiv1.RegisterCoverageServiceServer(grpcSrv, &coverageService{q: qSvc})
	japanapiv1.RegisterDimensionServiceServer(grpcSrv, &dimensionService{q: qSvc})
	japanapiv1.RegisterPriceServiceServer(grpcSrv, &priceService{q: qSvc})
	japanapiv1.RegisterSystemServiceServer(grpcSrv, &systemService{q: qSvc})
	japanapiv1.RegisterAnalysisServiceServer(grpcSrv, &analysisService{q: qSvc})
	japanapiv1.RegisterIngestionServiceServer(grpcSrv, &ingestionService{q: qSvc})

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
