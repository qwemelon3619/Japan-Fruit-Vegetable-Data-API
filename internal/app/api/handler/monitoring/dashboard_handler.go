package monitoring

import (
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed dashboard.html
var dashboardHTML []byte

func (s *Service) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(dashboardHTML)
}

func (s *Service) handleSnapshotsCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	snapshotPath, err := resolveSnapshotPath()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SNAPSHOT_PATH_INVALID", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	http.ServeFile(w, r, snapshotPath)
}

func resolveSnapshotPath() (string, error) {
	baseDir := os.Getenv("MONITOR_OUT_DIR")
	if baseDir == "" {
		baseDir = "./data/monitoring/csv"
	}
	snapshotPath := os.Getenv("SNAPSHOT_FILE")
	if snapshotPath == "" {
		snapshotPath = filepath.Join(baseDir, "snapshots.csv")
	}
	if !strings.EqualFold(filepath.Ext(snapshotPath), ".csv") {
		return "", os.ErrPermission
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(snapshotPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", os.ErrPermission
	}
	return pathAbs, nil
}
