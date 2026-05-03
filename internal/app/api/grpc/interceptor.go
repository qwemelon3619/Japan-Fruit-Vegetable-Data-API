package grpc

import (
	"context"
	"log/slog"
	"time"

	"japan_data_project/internal/app/api/handler/monitoring"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryLoggingInterceptor returns a unary interceptor that logs gRPC calls.
func UnaryLoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)

		code := status.Code(err)
		logger.Info("grpc_request",
			"method", info.FullMethod,
			"code", code.String(),
			"latency_ms", float64(elapsed.Microseconds())/1000.0,
		)

		return resp, err
	}
}

// UnaryRecoveryInterceptor recovers from panics in gRPC handlers.
func UnaryRecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = status.Errorf(codes.Internal, "panic recovered: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

// UnaryMetricsInterceptor returns a unary interceptor that records gRPC call metrics.
func UnaryMetricsInterceptor(mon *monitoring.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		mon.ObserveGRPC(info.FullMethod, time.Since(start), err)
		return resp, err
	}
}
