package services

import (
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/backend"
)

type ProvisioningService struct {
	backend backend.Backend
}

func NewProvisioningService(backend backend.Backend) *ProvisioningService {
	return &ProvisioningService{backend}
}

// Tokens are provisioning tokens for the auth server
func (s *ProvisioningService) UpsertToken(token, fqdn string, ttl time.Duration) error {
	err := s.backend.UpsertVal([]string{"tokens"}, token, []byte(fqdn), ttl)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return nil

}
func (s *ProvisioningService) GetToken(token string) (string, error) {
	fqdn, err := s.backend.GetVal([]string{"tokens"}, token)
	if err != nil {
		return "", err
	}
	return string(fqdn), nil
}
func (s *ProvisioningService) DeleteToken(token string) error {
	err := s.backend.DeleteKey([]string{"tokens"}, token)
	return err
}
