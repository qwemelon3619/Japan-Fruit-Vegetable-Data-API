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
	"japan_data_project/internal/app/query"

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

// newTestQueryService creates a query.Service that returns errors for all DB operations.
func newTestQueryService() *query.Service {
	return query.NewService(nil, func(_ string, fn func() error) error {
		if fn == nil {
			return nil
		}
		return fn() // will fail because db is nil
	})
}

// setupTestGRPCServer creates a bufconn-based in-process gRPC server for testing.
// Returns CoverageService, DimensionService, and PriceService clients.
func setupTestGRPCServer(t *testing.T) (japanapiv1.CoverageServiceClient, japanapiv1.DimensionServiceClient, japanapiv1.PriceServiceClient, func()) {
	t.Helper()

	coverageClient, dimClient, priceClient, _, _, _, cleanup := setupAllGRPCClients(t)
	return coverageClient, dimClient, priceClient, cleanup
}

// setupAllGRPCClients creates a bufconn-based in-process gRPC server and returns all 6 service clients.
func setupAllGRPCClients(t *testing.T) (
	japanapiv1.CoverageServiceClient,
	japanapiv1.DimensionServiceClient,
	japanapiv1.PriceServiceClient,
	japanapiv1.SystemServiceClient,
	japanapiv1.AnalysisServiceClient,
	japanapiv1.IngestionServiceClient,
	func(),
) {
	t.Helper()

	qSvc := newTestQueryService()
	monSvc := monitoring.NewService(nil)
	log := testLogger()

	grpcServer := appgrpc.New(qSvc, monSvc, 0, log)
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
		japanapiv1.NewSystemServiceClient(conn),
		japanapiv1.NewAnalysisServiceClient(conn),
		japanapiv1.NewIngestionServiceClient(conn),
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
	_, dimClient, _, _, _, _, cleanup := setupAllGRPCClients(t)
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

// ---- SystemService Tests ----

func TestGRPC_SystemService_GetHealth_ReturnsOK(t *testing.T) {
	_, _, _, sysClient, _, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	resp, err := sysClient.GetHealth(context.Background(), &japanapiv1.GetHealthRequest{})
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if resp.GetStatus() != "ok" {
		t.Fatalf("expected status 'ok', got %q", resp.GetStatus())
	}
}

func TestGRPC_SystemService_GetReady_NoDB_ReturnsInternal(t *testing.T) {
	_, _, _, sysClient, _, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := sysClient.GetReady(context.Background(), &japanapiv1.GetReadyRequest{})
	if err == nil {
		t.Fatal("expected error for no-DB GetReady, got nil")
	}
	// With nil DB, Ready() panics (nil pointer deref) and the recovery interceptor returns Internal.
	// In production with a real DB, a connection failure would return Unavailable instead.
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal (nil DB panic), got %v: %v", status.Code(err), err)
	}
}

// ---- AnalysisService Tests ----

func TestGRPC_AnalysisService_CompareMarkets_MissingDate(t *testing.T) {
	_, _, _, _, analysisClient, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := analysisClient.CompareMarkets(context.Background(), &japanapiv1.CompareMarketsRequest{
		ItemCode: "30100",
	})
	if err == nil {
		t.Fatal("expected error for missing date, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_AnalysisService_CompareMarkets_MissingItemCode(t *testing.T) {
	_, _, _, _, analysisClient, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := analysisClient.CompareMarkets(context.Background(), &japanapiv1.CompareMarketsRequest{
		Date: "2026-04-01",
	})
	if err == nil {
		t.Fatal("expected error for missing item_code, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_AnalysisService_CompareMarkets_NoDB_ReturnsInternal(t *testing.T) {
	_, _, _, _, analysisClient, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := analysisClient.CompareMarkets(context.Background(), &japanapiv1.CompareMarketsRequest{
		Date:     "2026-04-01",
		ItemCode: "30100",
	})
	if err == nil {
		t.Fatal("expected error for no-DB CompareMarkets, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_AnalysisService_RankItems_MissingDate(t *testing.T) {
	_, _, _, _, analysisClient, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := analysisClient.RankItems(context.Background(), &japanapiv1.RankItemsRequest{})
	if err == nil {
		t.Fatal("expected error for missing date, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_AnalysisService_RankItems_NoDB_ReturnsInternal(t *testing.T) {
	_, _, _, _, analysisClient, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := analysisClient.RankItems(context.Background(), &japanapiv1.RankItemsRequest{
		Date: "2026-04-01",
	})
	if err == nil {
		t.Fatal("expected error for no-DB RankItems, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

// ---- IngestionService Tests ----

func TestGRPC_IngestionService_ListIngestionRuns_NoDB_ReturnsInternal(t *testing.T) {
	_, _, _, _, _, ingestionClient, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := ingestionClient.ListIngestionRuns(context.Background(), &japanapiv1.ListIngestionRunsRequest{})
	if err == nil {
		t.Fatal("expected error for no-DB ListIngestionRuns, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_IngestionService_ListIngestionRuns_WithPaging_NoDB(t *testing.T) {
	_, _, _, _, _, ingestionClient, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := ingestionClient.ListIngestionRuns(context.Background(), &japanapiv1.ListIngestionRunsRequest{
		Limit:  10,
		Offset: 0,
	})
	if err == nil {
		t.Fatal("expected error for no-DB ListIngestionRuns with paging, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_IngestionService_ListIngestionFiles_NoDB_ReturnsInternal(t *testing.T) {
	_, _, _, _, _, ingestionClient, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := ingestionClient.ListIngestionFiles(context.Background(), &japanapiv1.ListIngestionFilesRequest{})
	if err == nil {
		t.Fatal("expected error for no-DB ListIngestionFiles, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_IngestionService_ListIngestionFiles_WithRunID_NoDB(t *testing.T) {
	_, _, _, _, _, ingestionClient, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	runID := uint32(1)
	_, err := ingestionClient.ListIngestionFiles(context.Background(), &japanapiv1.ListIngestionFilesRequest{
		RunId: &runID,
	})
	if err == nil {
		t.Fatal("expected error for no-DB ListIngestionFiles with run_id, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

// ---- Enhanced PriceService Tests (valid params -> Internal from no DB) ----

func TestGRPC_PriceService_GetDailyPrices_ValidRequest_NoDB(t *testing.T) {
	_, _, priceClient, _, _, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := priceClient.GetDailyPrices(context.Background(), &japanapiv1.GetDailyPricesRequest{
		Filter: &japanapiv1.PriceFilter{
			ItemCode: "30100",
			Limit:    10,
		},
	})
	if err == nil {
		t.Fatal("expected error for no-DB GetDailyPrices, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_PriceService_GetLatestPrices_ValidRequest_NoDB(t *testing.T) {
	_, _, priceClient, _, _, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := priceClient.GetLatestPrices(context.Background(), &japanapiv1.GetLatestPricesRequest{
		ItemCode: "30100",
		Limit:    10,
	})
	if err == nil {
		t.Fatal("expected error for no-DB GetLatestPrices, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_PriceService_GetPriceTrend_ValidRequest_NoDB(t *testing.T) {
	_, _, priceClient, _, _, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := priceClient.GetPriceTrend(context.Background(), &japanapiv1.GetPriceTrendRequest{
		ItemCode: "30100",
	})
	if err == nil {
		t.Fatal("expected error for no-DB GetPriceTrend, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

func TestGRPC_PriceService_GetPriceSummary_ValidRequest_NoDB(t *testing.T) {
	_, _, priceClient, _, _, _, cleanup := setupAllGRPCClients(t)
	defer cleanup()

	_, err := priceClient.GetPriceSummary(context.Background(), &japanapiv1.GetPriceSummaryRequest{
		ItemCode: "30100",
	})
	if err == nil {
		t.Fatal("expected error for no-DB GetPriceSummary, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v: %v", status.Code(err), err)
	}
}

// ---- Helpers ----

// strPtr returns a pointer to the given string value.
func strPtr(s string) *string {
	return &s
}
