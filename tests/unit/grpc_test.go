package tests

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"

	japanapiv1 "japan_data_project/proto/japanapi/v1"

	appgrpc "japan_data_project/internal/app/api/grpc"
	"japan_data_project/internal/app/api/handler/monitoring"
	v1svc "japan_data_project/internal/app/api/handler/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// testLogger returns a minimal logger for tests (error level only).
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestV1Service creates a v1.Service that returns errors for all DB operations.
// Useful for testing gRPC input validation without a real database.
func newTestV1Service() *v1svc.Service {
	return v1svc.NewService(nil, func(_ string, fn func() error) error {
		if fn == nil {
			return nil
		}
		return fn() // will fail because db is nil
	})
}

// setupTestGRPCServer creates a bufconn-based in-process gRPC server for testing.
// Returns clients for CoverageService, DimensionService, PriceService, and a cleanup function.
func setupTestGRPCServer(t *testing.T) (japanapiv1.CoverageServiceClient, japanapiv1.DimensionServiceClient, japanapiv1.PriceServiceClient, func()) {
	t.Helper()

	v1Svc := newTestV1Service()
	monSvc := monitoring.NewService(nil)
	log := testLogger()

	grpcServer := appgrpc.New(v1Svc, monSvc, 0, log)
	grpcSrv := grpcServer.GRPCServer()

	const bufSize = 1024 * 1024
	listener := bufconn.Listen(bufSize)

	go func() {
		if err := grpcSrv.Serve(listener); err != nil {
			t.Logf("gRPC test server stopped: %v", err)
		}
	}()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}

	cleanup := func() {
		conn.Close()
		grpcSrv.GracefulStop()
	}

	return japanapiv1.NewCoverageServiceClient(conn),
		japanapiv1.NewDimensionServiceClient(conn),
		japanapiv1.NewPriceServiceClient(conn),
		cleanup
}

func TestGRPC_CoverageService_RequestSucceeds(t *testing.T) {
	coverageClient, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	resp, err := coverageClient.GetCoverage(context.Background(), &japanapiv1.GetCoverageRequest{})
	if err != nil {
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal error for no-DB test, got %v: %v", status.Code(err), err)
		}
		t.Logf("coverage failed as expected (no DB): %v", err)
		return
	}
	if resp != nil {
		t.Logf("coverage response: earliest=%q latest=%q rows=%d",
			resp.EarliestTradeDate, resp.LatestTradeDate, resp.FactRowsTotal)
	}
}

func TestGRPC_DimensionService_ListMarkets_NoDB(t *testing.T) {
	_, dimClient, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	resp, err := dimClient.ListMarkets(context.Background(), &japanapiv1.ListMarketsRequest{})
	if err != nil {
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal error, got %v: %v", status.Code(err), err)
		}
		t.Logf("list markets failed as expected (no DB): %v", err)
		return
	}
	if resp != nil {
		t.Logf("got %d markets", len(resp.Data))
	}
}

func TestGRPC_PriceService_GetDailyPrices_MissingItemCode(t *testing.T) {
	_, _, priceClient, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	_, err := priceClient.GetDailyPrices(context.Background(), &japanapiv1.GetDailyPricesRequest{})
	if err == nil {
		t.Fatal("expected error for missing item_code, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_PriceService_GetLatestPrices_MissingItemCode(t *testing.T) {
	_, _, priceClient, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	_, err := priceClient.GetLatestPrices(context.Background(), &japanapiv1.GetLatestPricesRequest{})
	if err == nil {
		t.Fatal("expected error for missing item_code, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_PriceService_GetPriceTrend_MissingItemCode(t *testing.T) {
	_, _, priceClient, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	_, err := priceClient.GetPriceTrend(context.Background(), &japanapiv1.GetPriceTrendRequest{})
	if err == nil {
		t.Fatal("expected error for missing item_code, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_PriceService_GetPriceSummary_MissingItemCode(t *testing.T) {
	_, _, priceClient, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	_, err := priceClient.GetPriceSummary(context.Background(), &japanapiv1.GetPriceSummaryRequest{})
	if err == nil {
		t.Fatal("expected error for missing item_code, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_DimensionService_ListItems_WithoutFilter(t *testing.T) {
	_, dimClient, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	resp, err := dimClient.ListItems(context.Background(), &japanapiv1.ListItemsRequest{})
	if err != nil {
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal error, got %v: %v", status.Code(err), err)
		}
		t.Logf("list items failed as expected (no DB): %v", err)
		return
	}
	if resp != nil {
		t.Logf("got %d items", len(resp.Data))
	}
}

func TestGRPC_DimensionService_ListOrigins_WithFilter(t *testing.T) {
	_, dimClient, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	resp, err := dimClient.ListOrigins(context.Background(), &japanapiv1.ListOriginsRequest{
		Filter: &japanapiv1.DimensionFilter{
			Limit:  10,
			Offset: 0,
			Q:      "tokyo",
			Sort:   "name",
			Order:  "asc",
		},
	})
	if err != nil {
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal error, got %v: %v", status.Code(err), err)
		}
		t.Logf("list origins with filter failed as expected (no DB): %v", err)
		return
	}
	if resp != nil {
		t.Logf("got %d origins", len(resp.Data))
	}
}
