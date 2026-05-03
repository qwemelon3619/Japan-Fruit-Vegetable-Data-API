package monitoring

import "time"

func (s *Service) observeDB(queryName string, fn func() error) error {
	start := time.Now()
	err := fn()
	s.metrics.observeDB(queryName, time.Since(start).Seconds(), err)
	return err
}

func (s *Service) ObserveDB(queryName string, fn func() error) error {
	return s.observeDB(queryName, fn)
}

// ObserveGRPC records a gRPC call in the shared metrics store.
func (s *Service) ObserveGRPC(method string, duration time.Duration, err error) {
	s.metrics.observeGRPC(method, duration.Seconds(), err)
}
