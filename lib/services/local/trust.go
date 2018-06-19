package local

import (
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CA is local implementation of Trust service that
// is using local backend
type CA struct {
	backend.Backend
}

// NewCAService returns new instance of CAService
func NewCAService(b backend.Backend) *CA {
	return &CA{
		Backend: b,
	}
}

// DeleteAllCertAuthorities deletes all certificate authorities of a certain type
func (s *CA) DeleteAllCertAuthorities(caType services.CertAuthType) error {
	return s.DeleteBucket([]string{"authorities"}, string(caType))
}

// CreateCertAuthority updates or inserts a new certificate authority
func (s *CA) CreateCertAuthority(ca services.CertAuthority) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), ca.Expiry())
	err = s.CreateVal([]string{"authorities", string(ca.GetType())}, ca.GetName(), data, ttl)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("cluster %q already exists", ca.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (s *CA) UpsertCertAuthority(ca services.CertAuthority) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), ca.Expiry())
	err = s.UpsertVal([]string{"authorities", string(ca.GetType())}, ca.GetName(), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CompareAndSwapCertAuthority updates the cert authority value
// if the existing value matches existing parameter, returns nil if succeeds,
// trace.CompareFailed otherwise.
func (s *CA) CompareAndSwapCertAuthority(new, existing services.CertAuthority) error {
	if err := new.Check(); err != nil {
		return trace.Wrap(err)
	}
	newData, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(new)
	if err != nil {
		return trace.Wrap(err)
	}
	existingData, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(existing)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), new.Expiry())
	err = s.CompareAndSwapVal([]string{"authorities", string(new.GetType())}, new.GetName(), newData, existingData, ttl)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("cluster %v settings have been updated, try again", new.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteCertAuthority deletes particular certificate authority
func (s *CA) DeleteCertAuthority(id services.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	// when removing a services.CertAuthority also remove any deactivated
	// services.CertAuthority as well if they exist.
	err := s.DeleteKey([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	err = s.DeleteKey([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (s *CA) ActivateCertAuthority(id services.CertAuthID) error {
	data, err := s.GetVal([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.BadParameter("can not activate CertAuthority which has not been deactivated: %v: %v", id, err)
	}

	certAuthority, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.DeleteKey([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (s *CA) DeactivateCertAuthority(id services.CertAuthID) error {
	certAuthority, err := s.GetCertAuthority(id, true)
	if err != nil {
		return trace.NotFound("can not deactivate CertAuthority which does not exist: %v", err)
	}

	err = s.DeleteCertAuthority(id)
	if err != nil {
		return trace.Wrap(err)
	}

	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), certAuthority.Expiry())

	err = s.UpsertVal([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (s *CA) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool) (services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := s.GetVal([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ca.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	setSigningKeys(ca, loadSigningKeys)
	return ca, nil
}

func setSigningKeys(ca services.CertAuthority, loadSigningKeys bool) {
	if loadSigningKeys {
		return
	}
	ca.SetSigningKeys(nil)
	keyPairs := ca.GetTLSKeyPairs()
	for i := range keyPairs {
		keyPairs[i].Key = nil
	}
	ca.SetTLSKeyPairs(keyPairs)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (s *CA) GetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool) ([]services.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get all items in the bucket.
	bucket := []string{"authorities", string(caType)}
	items, err := s.GetItems(bucket)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal values into a []services.CertAuthority slice.
	cas := make([]services.CertAuthority, len(items))
	for i, item := range items {
		ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := ca.Check(); err != nil {
			return nil, trace.Wrap(err)
		}
		setSigningKeys(ca, loadSigningKeys)
		cas[i] = ca
	}

	return cas, nil
}
