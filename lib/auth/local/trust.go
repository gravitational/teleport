/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// Trust manages certificate authorities on the auth server
type Trust interface {
	auth.Trust

	// CreateCertAuthority inserts a new certificate authority
	CreateCertAuthority(ca types.CertAuthority) error

	// ActivateCertAuthority moves a CertAuthority from the deactivated list to
	// the normal list.
	ActivateCertAuthority(id types.CertAuthID) error

	// DeactivateCertAuthority moves a CertAuthority from the normal list to
	// the deactivated list.
	DeactivateCertAuthority(id types.CertAuthID) error

	// DeleteAllCertAuthorities deletes cert authorities of a certain type
	DeleteAllCertAuthorities(caType types.CertAuthType) error
}

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
	startKey := backend.Key(authoritiesPrefix, string(caType))
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// CreateCertAuthority updates or inserts a new certificate authority
func (s *CA) CreateCertAuthority(ca services.CertAuthority) error {
	if err := auth.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	value, err := resource.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:   value,
		Expires: ca.Expiry(),
	}

	_, err = s.Create(context.TODO(), item)
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
	if err := auth.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	value, err := resource.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:   value,
		Expires: ca.Expiry(),
		ID:      ca.GetResourceID(),
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CompareAndSwapCertAuthority updates the cert authority value
// if the existing value matches existing parameter, returns nil if succeeds,
// trace.CompareFailed otherwise.
func (s *CA) CompareAndSwapCertAuthority(new, existing services.CertAuthority) error {
	if err := auth.ValidateCertAuthority(new); err != nil {
		return trace.Wrap(err)
	}
	newValue, err := resource.MarshalCertAuthority(new)
	if err != nil {
		return trace.Wrap(err)
	}
	newItem := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(new.GetType()), new.GetName()),
		Value:   newValue,
		Expires: new.Expiry(),
	}

	existingValue, err := resource.MarshalCertAuthority(existing)
	if err != nil {
		return trace.Wrap(err)
	}
	existingItem := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(existing.GetType()), existing.GetName()),
		Value:   existingValue,
		Expires: existing.Expiry(),
	}

	_, err = s.CompareAndSwap(context.TODO(), existingItem, newItem)
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
	err := s.Delete(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	err = s.Delete(context.TODO(), backend.Key(authoritiesPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (s *CA) ActivateCertAuthority(id services.CertAuthID) error {
	item, err := s.Get(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("can not activate cert authority %q which has not been deactivated", id.DomainName)
		}
		return trace.Wrap(err)
	}

	certAuthority, err := resource.UnmarshalCertAuthority(
		item.Value, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.Delete(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
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
		if trace.IsNotFound(err) {
			return trace.NotFound("can not deactivate cert authority %q which does not exist", id.DomainName)
		}
		return trace.Wrap(err)
	}

	err = s.DeleteCertAuthority(id)
	if err != nil {
		return trace.Wrap(err)
	}

	value, err := resource.MarshalCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName),
		Value:   value,
		Expires: certAuthority.Expiry(),
		ID:      certAuthority.GetResourceID(),
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (s *CA) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool, opts ...auth.MarshalOption) (services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.Get(context.TODO(), backend.Key(authoritiesPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := resource.UnmarshalCertAuthority(
		item.Value, resource.AddOptions(opts, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	setSigningKeys(ca, loadSigningKeys)
	return ca, nil
}

func setSigningKeys(ca services.CertAuthority, loadSigningKeys bool) {
	if loadSigningKeys {
		return
	}
	services.RemoveCASecrets(ca)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (s *CA) GetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool, opts ...auth.MarshalOption) ([]services.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get all items in the bucket.
	startKey := backend.Key(authoritiesPrefix, string(caType))
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal values into a []services.CertAuthority slice.
	cas := make([]services.CertAuthority, len(result.Items))
	for i, item := range result.Items {
		ca, err := resource.UnmarshalCertAuthority(
			item.Value, resource.AddOptions(opts,
				resource.WithResourceID(item.ID),
				resource.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := auth.ValidateCertAuthority(ca); err != nil {
			return nil, trace.Wrap(err)
		}
		setSigningKeys(ca, loadSigningKeys)
		cas[i] = ca
	}

	return cas, nil
}

const (
	authoritiesPrefix = "authorities"
	deactivatedPrefix = "deactivated"
)
