package services

import (
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/backend"
)

type LockService struct {
	backend backend.Backend
}

func NewLockService(backend backend.Backend) *LockService {
	return &LockService{backend}
}

// Grab a lock that will be released automatically in ttl time
func (s *LockService) AcquireLock(token string, ttl time.Duration) error {
	_, err := s.backend.GetVal([]string{"locks"}, token)
	if err == nil {
		return &AlreadyAcquiredError{""}
	} else {
		switch err.(type) {
		case *backend.NotFoundError:
		default:
			log.Errorf(err.Error())
			return err
		}
	}

	err = s.backend.UpsertVal([]string{"locks"}, token, []byte("lock"), ttl)
	if err != nil {
		log.Errorf(err.Error())
	}
	return err

}

func (s *LockService) ReleaseLock(token string) error {
	return convertErr(s.backend.DeleteKey([]string{"locks"}, token))
}
