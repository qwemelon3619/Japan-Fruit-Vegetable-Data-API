package handler

import (
	"log/slog"
	"net/http"

	"japan_data_project/internal/app/query"
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
	q          *query.Service
}

func New(db *gorm.DB, logger *slog.Logger) *Handler {
	monitoringSvc := monitoring.NewService(db)
	qSvc := query.NewService(db, monitoringSvc.ObserveDB)

	return &Handler{
		logger:     logger,
		doc:        doc.NewService(),
		monitoring: monitoringSvc,
		apiV1:      v1.NewService(qSvc),
		q:          qSvc,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	h.doc.Register(mux)
	h.monitoring.Register(mux)
	h.apiV1.Register(mux)
}

func (h *Handler) APIV1() *v1.Service {
	return h.apiV1
}

// QueryService returns the shared query service for gRPC handlers.
func (h *Handler) QueryService() *query.Service {
	return h.q
}

func (h *Handler) Monitoring() *monitoring.Service {
	return h.monitoring
}

func (h *Handler) WrapWithObservability(next http.Handler) http.Handler {
	return h.monitoring.WrapWithObservability(h.logger, next)
}
