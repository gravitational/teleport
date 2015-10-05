package services

import (
	"time"

	"github.com/gravitational/teleport/lib/backend"
)

type LockService struct {
	backend backend.Backend
}

func NewLockService(backend backend.Backend) *LockService {
	return &LockService{backend}
}

// Grab a lock that will be released automatically in ttl time
func (s *LockService) AcquireLock(token string, ttl time.Duration) error {
	return s.backend.AcquireLock(token, ttl)
}

func (s *LockService) ReleaseLock(token string) error {
	return s.backend.ReleaseLock(token)
}
