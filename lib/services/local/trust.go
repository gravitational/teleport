/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
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
func (s *CA) DeleteAllCertAuthorities(caType types.CertAuthType) error {
	// The backend stores CAs like /authorities/<caType>/<name>, so caType is a
	// prefix.
	// If we do not use ExactKey here, then if a caType is a prefix of another
	// caType, it will delete both, e.g.: DeleteAllCertAuthorities("foo") would
	// also delete all authorities of caType "foo_some_suffix".
	startKey := backend.ExactKey(authoritiesPrefix, string(caType))
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// CreateCertAuthority updates or inserts a new certificate authority
func (s *CA) CreateCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:   value,
		Expires: ca.Expiry(),
	}

	_, err = s.Create(ctx, item)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("cluster %q already exists", ca.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (s *CA) UpsertCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}

	// try to skip writes that would have no effect
	if existing, err := s.GetCertAuthority(ctx, types.CertAuthID{
		Type:       ca.GetType(),
		DomainName: ca.GetClusterName(),
	}, true); err == nil {
		if services.CertAuthoritiesEquivalent(existing, ca) {
			return nil
		}
	}

	rev := ca.GetRevision()
	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:    value,
		Expires:  ca.Expiry(),
		ID:       ca.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CompareAndSwapCertAuthority updates the cert authority value
// if the existing value matches expected parameter, returns nil if succeeds,
// trace.CompareFailed otherwise.
func (s *CA) CompareAndSwapCertAuthority(new, expected types.CertAuthority) error {
	if err := services.ValidateCertAuthority(new); err != nil {
		return trace.Wrap(err)
	}

	key := backend.Key(authoritiesPrefix, string(new.GetType()), new.GetName())

	actualItem, err := s.Get(context.TODO(), key)
	if err != nil {
		return trace.Wrap(err)
	}
	actual, err := services.UnmarshalCertAuthority(actualItem.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	if !services.CertAuthoritiesEquivalent(actual, expected) {
		return trace.CompareFailed("cluster %v settings have been updated, try again", new.GetName())
	}

	rev := new.GetRevision()
	newValue, err := services.MarshalCertAuthority(new)
	if err != nil {
		return trace.Wrap(err)
	}
	newItem := backend.Item{
		Key:      key,
		Value:    newValue,
		Expires:  new.Expiry(),
		Revision: rev,
	}

	_, err = s.CompareAndSwap(context.TODO(), *actualItem, newItem)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("cluster %v settings have been updated, try again", new.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteCertAuthority deletes particular certificate authority
func (s *CA) DeleteCertAuthority(ctx context.Context, id types.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	// when removing a types.CertAuthority also remove any deactivated
	// types.CertAuthority as well if they exist.
	err := s.Delete(ctx, backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	err = s.Delete(ctx, backend.Key(authoritiesPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (s *CA) ActivateCertAuthority(id types.CertAuthID) error {
	item, err := s.Get(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("can not activate cert authority %q which has not been deactivated", id.DomainName)
		}
		return trace.Wrap(err)
	}

	certAuthority, err := services.UnmarshalCertAuthority(
		item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertCertAuthority(context.TODO(), certAuthority)
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
func (s *CA) DeactivateCertAuthority(id types.CertAuthID) error {
	certAuthority, err := s.GetCertAuthority(context.TODO(), id, true)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("can not deactivate cert authority %q which does not exist", id.DomainName)
		}
		return trace.Wrap(err)
	}

	err = s.DeleteCertAuthority(context.TODO(), id)
	if err != nil {
		return trace.Wrap(err)
	}

	rev := certAuthority.GetRevision()
	value, err := services.MarshalCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName),
		Value:    value,
		Expires:  certAuthority.Expiry(),
		ID:       certAuthority.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (s *CA) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.Get(ctx, backend.Key(authoritiesPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := services.UnmarshalCertAuthority(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	setSigningKeys(ca, loadSigningKeys)
	return ca, nil
}

func setSigningKeys(ca types.CertAuthority, loadSigningKeys bool) {
	if loadSigningKeys {
		return
	}
	types.RemoveCASecrets(ca)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (s *CA) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadSigningKeys bool) ([]types.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get all items in the bucket.
	// The backend stores CAs like /authorities/<caType>/<name>, so caType is a
	// prefix.
	// If we do not use ExactKey here, then if a caType is a prefix of another
	// caType, it will get both, e.g.: GetCertAuthorities("foo") would
	// also get authorities of caType "foo_some_suffix".
	startKey := backend.ExactKey(authoritiesPrefix, string(caType))
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal values into a []types.CertAuthority slice.
	cas := make([]types.CertAuthority, len(result.Items))
	for i, item := range result.Items {
		ca, err := services.UnmarshalCertAuthority(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := services.ValidateCertAuthority(ca); err != nil {
			return nil, trace.Wrap(err)
		}
		setSigningKeys(ca, loadSigningKeys)
		cas[i] = ca
	}

	return cas, nil
}

// UpdateUserCARoleMap updates the role map of the userCA of the specified existing cluster.
func (s *CA) UpdateUserCARoleMap(ctx context.Context, name string, roleMap types.RoleMap, activated bool) error {
	key := backend.Key(authoritiesPrefix, string(types.UserCA), name)
	if !activated {
		key = backend.Key(authoritiesPrefix, deactivatedPrefix, string(types.UserCA), name)
	}

	actualItem, err := s.Get(ctx, key)
	if err != nil {
		return trace.Wrap(err)
	}
	actual, err := services.UnmarshalCertAuthority(actualItem.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	actual.SetRoleMap(roleMap)

	rev := actual.GetRevision()
	newValue, err := services.MarshalCertAuthority(actual)
	if err != nil {
		return trace.Wrap(err)
	}
	newItem := backend.Item{
		Key:      key,
		Value:    newValue,
		Expires:  actual.Expiry(),
		Revision: rev,
	}
	_, err = s.CompareAndSwap(ctx, *actualItem, newItem)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("cluster %v settings have been updated, try again", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

const (
	authoritiesPrefix = "authorities"
	deactivatedPrefix = "deactivated"
)
