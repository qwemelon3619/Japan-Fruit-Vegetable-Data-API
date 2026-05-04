// Package query provides the shared query layer used by both HTTP and gRPC handlers.
package query

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// dbObserver is a function type for instrumenting database calls.
type dbObserver func(queryName string, fn func() error) error

// Service holds the database connection and provides all query methods.
type Service struct {
	db        *gorm.DB
	observeDB dbObserver
}

// NewService creates a new query Service.
func NewService(db *gorm.DB, observer dbObserver) *Service {
	if observer == nil {
		observer = func(_ string, fn func() error) error {
			if fn == nil {
				return nil
			}
			return fn()
		}
	}
	return &Service{db: db, observeDB: observer}
}

// DB returns the underlying *gorm.DB (used for transaction support if needed).
func (s *Service) DB() *gorm.DB {
	return s.db
}

// Ready checks database connectivity. Used by gRPC GetReady and HTTP /ready.
func (s *Service) Ready(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql db: %w", err)
	}
	return s.observeDB("ready_ping", func() error {
		return sqlDB.PingContext(ctx)
	})
}
