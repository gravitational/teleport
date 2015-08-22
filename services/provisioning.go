package services

import (
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/backend"
)

type ProvisioningService struct {
	backend backend.Backend
}

// Tokens are provisioning tokens for the auth server
func (s *ProvisioningService) UpsertToken(token, fqdn string, ttl time.Duration) error {
	err := s.backend.UpsertVal([]string{"tokens"}, token, []byte(fqdn), ttl)
	return err

}
func (s *ProvisioningService) GetToken(token string) (string, error) {
	fqdn, err := s.backend.GetVal([]string{"tokens"}, token)
	if err != nil {
		log.Errorf(err.Error())
		return "", err
	}
	return string(fqdn), nil
}
func (s *ProvisioningService) DeleteToken(token string) error {
	err := s.backend.DeleteKey([]string{"tokens"}, token)
	if err != nil {
		log.Errorf(err.Error())
	}
	return err
}
