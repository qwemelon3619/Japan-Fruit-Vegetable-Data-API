package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"japan_data_project/internal/app/api/grpc"
	"japan_data_project/internal/app/api/handler"
	"japan_data_project/internal/platform/config"
	"japan_data_project/internal/platform/db"
	xlog "japan_data_project/internal/platform/logger"
	"japan_data_project/internal/platform/xerror"
)

func Run() error {
	cfg := config.Load()
	logger := xlog.New(cfg)

	gormDB, err := db.OpenGorm(cfg)
	if err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to open gorm", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to get sql db", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.Ping(); err != nil {
		return xerror.Wrap(xerror.CodeDB, "db ping failed", err)
	}
	if err := db.AutoMigrate(gormDB); err != nil {
		return xerror.Wrap(xerror.CodeDB, "auto migrate failed", err)
	}

	// Shared handler for both HTTP and gRPC
	h := handler.New(gormDB, logger)

	// ----- HTTP Server -----
	mux := http.NewServeMux()
	h.Register(mux)

	httpAddr := fmt.Sprintf(":%d", cfg.App.HTTPPort)
	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           h.WrapWithObservability(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("http server started", "port", cfg.App.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http serve failed", "error", err)
		}
	}()

	// ----- gRPC Server (uses shared query.Service) -----
	grpcServer := grpc.New(h.QueryService(), h.Monitoring(), cfg.App.GRPCPort, logger)
	go func() {
		logger.Info("gRPC server starting", "port", cfg.App.GRPCPort)
		if err := grpcServer.Start(); err != nil {
			logger.Error("gRPC serve failed", "error", err)
		}
	}()

	// ----- Wait for shutdown signal -----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	// Graceful shutdown: HTTP first, then gRPC
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", "error", err)
	}

	grpcServer.GracefulStop()

	logger.Info("shutdown complete")
	return nil
}

func MustRun() {
	if err := Run(); err != nil {
		log.Fatal(err)
	}
}
