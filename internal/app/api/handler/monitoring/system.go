package monitoring

import (
	"context"
	"net/http"
	"time"
)

func (h *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeOK(w, map[string]string{"status": "ok"}, nil)
}

func (h *Service) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "failed to get sql db")
		return
	}
	if err := h.observeDB("ready_ping", func() error { return sqlDB.PingContext(ctx) }); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_NOT_READY", "database ping failed")
		return
	}
	writeOK(w, map[string]string{"status": "ready"}, nil)
}
