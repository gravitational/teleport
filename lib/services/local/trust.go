package local

import (
	"encoding/json"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CA is local implementation of Trust service that
// is using local backend
type CA struct {
	backend backend.Backend
}

// NewCAService returns new instance of CAService
func NewCAService(backend backend.Backend) *CA {
	return &CA{backend: backend}
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (s *CA) UpsertCertAuthority(ca services.CertAuthority, ttl time.Duration) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	out, err := json.Marshal(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"authorities", string(ca.Type)}, ca.DomainName, out, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteCertAuthority deletes particular certificate authority
func (s *CA) DeleteCertAuthority(id services.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	err := s.backend.DeleteKey([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (s *CA) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool) (*services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	val, err := s.backend.GetVal([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ca *services.CertAuthority
	if err := json.Unmarshal(val, &ca); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ca.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if !loadSigningKeys {
		ca.SigningKeys = nil
	}
	return ca, nil
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (s *CA) GetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool) ([]*services.CertAuthority, error) {
	cas := []*services.CertAuthority{}
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	domains, err := s.backend.GetKeys([]string{"authorities", string(caType)})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, domain := range domains {
		ca, err := s.GetCertAuthority(services.CertAuthID{DomainName: domain, Type: caType}, loadSigningKeys)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cas = append(cas, ca)
	}
	return cas, nil
}
