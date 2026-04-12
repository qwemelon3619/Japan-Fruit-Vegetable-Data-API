package monitoring

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	metrics *metricsStore
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db:      db,
		metrics: newMetricsStore(),
	}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/monitoring/dashboard", s.handleDashboard)
	mux.HandleFunc("/monitoring/snapshots.csv", s.handleSnapshotsCSV)
}

func (s *Service) WrapWithObservability(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.metrics.incInflight()
		defer s.metrics.decInflight()

		requestID := newRequestID()
		sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}
		sw.Header().Set("X-Request-Id", requestID)
		if !strings.HasPrefix(r.URL.Path, "/metrics") &&
			!strings.HasPrefix(r.URL.Path, "/doc") &&
			!strings.HasPrefix(r.URL.Path, "/monitoring/dashboard") &&
			!strings.HasPrefix(r.URL.Path, "/monitoring/snapshots.csv") {
			sw.Header().Set("Content-Type", "application/json; charset=utf-8")
		}

		next.ServeHTTP(sw, r)

		elapsed := time.Since(start)
		s.metrics.observeHTTP(r.Method, normalizeMetricPath(r.URL.Path), sw.statusCode, elapsed.Seconds())
		errorCode := sw.Header().Get("X-Error-Code")
		logger.Info("http_request",
			"request_id", requestID,
			"path", r.URL.Path,
			"method", r.Method,
			"status", sw.statusCode,
			"latency_ms", float64(elapsed.Microseconds())/1000.0,
			"error_code", errorCode,
		)
	})
}

func normalizeMetricPath(path string) string {
	switch {
	case path == "/health":
		return "/health"
	case path == "/ready":
		return "/ready"
	case path == "/metrics":
		return "/metrics"
	case path == "/doc":
		return "/doc"
	case path == "/monitoring/dashboard":
		return "/monitoring/dashboard"
	case path == "/monitoring/snapshots.csv":
		return "/monitoring/snapshots.csv"
	case strings.HasPrefix(path, "/v1/markets"):
		return "/v1/markets"
	case strings.HasPrefix(path, "/v1/items"):
		return "/v1/items"
	case strings.HasPrefix(path, "/v1/origins"):
		return "/v1/origins"
	case strings.HasPrefix(path, "/v1/prices/daily"):
		return "/v1/prices/daily"
	case strings.HasPrefix(path, "/v1/prices/trend"):
		return "/v1/prices/trend"
	case strings.HasPrefix(path, "/v1/prices/summary"):
		return "/v1/prices/summary"
	case strings.HasPrefix(path, "/v1/compare/markets"):
		return "/v1/compare/markets"
	case strings.HasPrefix(path, "/v1/rankings/items"):
		return "/v1/rankings/items"
	case strings.HasPrefix(path, "/v1/ingestion/runs"):
		return "/v1/ingestion/runs"
	case strings.HasPrefix(path, "/v1/ingestion/files"):
		return "/v1/ingestion/files"
	default:
		return "/other"
	}
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
