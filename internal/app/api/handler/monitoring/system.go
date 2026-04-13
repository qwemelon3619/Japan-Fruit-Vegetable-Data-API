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

	h.ready.mu.Lock()
	defer h.ready.mu.Unlock()

	if time.Since(h.ready.lastCheck) <= readyCacheTTL {
		if h.ready.lastOK {
			writeOK(w, map[string]string{"status": "ready"}, nil)
			return
		}
		writeErr(w, http.StatusServiceUnavailable, "DB_NOT_READY", h.ready.lastErrMsg)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		h.ready.lastCheck = time.Now()
		h.ready.lastOK = false
		h.ready.lastErrMsg = "failed to get sql db"
		writeErr(w, http.StatusInternalServerError, "INTERNAL", h.ready.lastErrMsg)
		return
	}
	if err := h.observeDB("ready_ping", func() error { return sqlDB.PingContext(ctx) }); err != nil {
		h.ready.lastCheck = time.Now()
		h.ready.lastOK = false
		h.ready.lastErrMsg = "database ping failed"
		writeErr(w, http.StatusServiceUnavailable, "DB_NOT_READY", h.ready.lastErrMsg)
		return
	}

	h.ready.lastCheck = time.Now()
	h.ready.lastOK = true
	h.ready.lastErrMsg = ""
	writeOK(w, map[string]string{"status": "ready"}, nil)
}
