package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

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

	h := handler.New(gormDB, logger)
	mux := http.NewServeMux()
	h.Register(mux)

	addr := fmt.Sprintf(":%d", cfg.App.HTTPPort)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           h.WrapWithObservability(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("api started", "http_port", cfg.App.HTTPPort, "db", cfg.Database.DBName)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return xerror.Wrap(xerror.CodeInternal, "api serve failed", err)
	}
	return nil
}

func MustRun() {
	if err := Run(); err != nil {
		log.Fatal(err)
	}
}
