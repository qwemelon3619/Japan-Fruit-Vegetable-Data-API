package doc

import (
	_ "embed"
	"net/http"
)

//go:embed doc.html
var docHTML []byte

func (s *Service) handleDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":{"code":"METHOD_NOT_ALLOWED","message":"method not allowed"}}`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docHTML)
}
