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
