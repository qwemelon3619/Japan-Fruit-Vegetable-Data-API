package doc

import "net/http"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("/doc", s.handleDoc)
}

