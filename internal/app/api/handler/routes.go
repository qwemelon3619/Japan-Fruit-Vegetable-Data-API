package handler

import (
	"log/slog"
	"net/http"

	doc "japan_data_project/internal/app/api/handler/doc"
	"japan_data_project/internal/app/api/handler/monitoring"
	v1 "japan_data_project/internal/app/api/handler/v1"

	"gorm.io/gorm"
)

type Handler struct {
	logger     *slog.Logger
	doc        *doc.Service
	monitoring *monitoring.Service
	apiV1      *v1.Service
}

func New(db *gorm.DB, logger *slog.Logger) *Handler {
	monitoringSvc := monitoring.NewService(db)

	return &Handler{
		logger:     logger,
		doc:        doc.NewService(),
		monitoring: monitoringSvc,
		apiV1:      v1.NewService(db, monitoringSvc.ObserveDB),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	h.doc.Register(mux)
	h.monitoring.Register(mux)
	h.apiV1.Register(mux)
}

func (h *Handler) WrapWithObservability(next http.Handler) http.Handler {
	return h.monitoring.WrapWithObservability(h.logger, next)
}
