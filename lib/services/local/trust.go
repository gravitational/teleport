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
	"errors"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// CA is local implementation of Trust service that
// is using local backend
type CA struct {
	backend.Backend
	log *logrus.Entry
}

// NewCAService returns new instance of CAService
func NewCAService(b backend.Backend) *CA {
	return &CA{
		Backend: b,
		log:     logrus.WithFields(logrus.Fields{trace.Component: "CA"}),
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
	return s.CreateCertAuthorities(ctx, ca)
}

// CreateCertAuthorities creates multiple cert authorities atomically.
func (s *CA) CreateCertAuthorities(ctx context.Context, cas ...types.CertAuthority) error {
	var condacts []backend.ConditionalAction
	var clusterNames []string
	for _, ca := range cas {
		if !slices.Contains(clusterNames, ca.GetName()) {
			clusterNames = append(clusterNames, ca.GetName())
		}
		if err := services.ValidateCertAuthority(ca); err != nil {
			return trace.Wrap(err)
		}

		value, err := services.MarshalCertAuthority(ca)
		if err != nil {
			return trace.Wrap(err)
		}

		condacts = append(condacts, []backend.ConditionalAction{
			{
				Key:       activeKey(ca.GetID()),
				Condition: backend.NotExists(),
				Action: backend.Put(backend.Item{
					Value:   value,
					Expires: ca.Expiry(),
				}),
			},
			{
				Key:       inactiveKey(ca.GetID()),
				Condition: backend.Whatever(),
				Action:    backend.Delete(),
			},
		}...)
	}

	_, err := s.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.AlreadyExists("one or more CAs from cluster(s) %q already exist", strings.Join(clusterNames, ","))
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
	if existing, err := s.GetCertAuthority(ctx, ca.GetID(), true); err == nil {
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
		Key:      activeKey(ca.GetID()),
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

// UpdateCertAuthority updates an existing cert authority if the revisions match.
func (s *CA) UpdateCertAuthority(ctx context.Context, ca types.CertAuthority) (types.CertAuthority, error) {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      activeKey(ca.GetID()),
		Value:    value,
		Expires:  ca.Expiry(),
		ID:       ca.GetResourceID(),
		Revision: ca.GetRevision(),
	}

	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ca = ca.Clone()
	ca.SetRevision(lease.Revision)
	ca.SetResourceID(lease.ID)
	return ca, nil
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
	return s.DeleteCertAuthorities(ctx, id)
}

// DeleteCertAuthorities deletes multiple cert authorities atomically.
func (s *CA) DeleteCertAuthorities(ctx context.Context, ids ...types.CertAuthID) error {
	var condacts []backend.ConditionalAction
	for _, id := range ids {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
		for _, key := range [][]byte{activeKey(id), inactiveKey(id)} {
			condacts = append(condacts, backend.ConditionalAction{
				Key:       key,
				Condition: backend.Whatever(),
				Action:    backend.Delete(),
			})
		}
	}

	_, err := s.AtomicWrite(ctx, condacts)
	return trace.Wrap(err)
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (s *CA) ActivateCertAuthority(id types.CertAuthID) error {
	return s.ActivateCertAuthorities(context.TODO(), id)
}

// ActivateCertAuthorities activates multiple cert authorities atomically.
func (s *CA) ActivateCertAuthorities(ctx context.Context, ids ...types.CertAuthID) error {
	var condacts []backend.ConditionalAction
	var domainNames []string
	for _, id := range ids {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}

		if !slices.Contains(domainNames, id.DomainName) {
			domainNames = append(domainNames, id.DomainName)
		}

		item, err := s.Get(ctx, inactiveKey(id))
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.Errorf("can not activate cert authority %q of type %q (not a currently inactive ca)", id.DomainName, id.Type)
			}
			return trace.Wrap(err)
		}

		condacts = append(condacts, []backend.ConditionalAction{
			{
				Key:       inactiveKey(id),
				Condition: backend.Revision(item.Revision),
				Action:    backend.Delete(),
			},
			{
				Key:       activeKey(id),
				Condition: backend.Whatever(),
				Action:    backend.Put(*item),
			},
		}...)
	}

	if _, err := s.AtomicWrite(ctx, condacts); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.Errorf("failed to activate one or more cert authorities for cluster(s) %q due to concurrent modification", strings.Join(domainNames, ","))
		}
		return trace.Wrap(err)
	}

	return nil
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (s *CA) DeactivateCertAuthority(id types.CertAuthID) error {
	return s.DeactivateCertAuthorities(context.TODO(), id)
}

// DeactivateCertAuthorities deactivates multiple cert authorities atomically.
func (s *CA) DeactivateCertAuthorities(ctx context.Context, ids ...types.CertAuthID) error {
	var condacts []backend.ConditionalAction
	var domainNames []string
	for _, id := range ids {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}

		if !slices.Contains(domainNames, id.DomainName) {
			domainNames = append(domainNames, id.DomainName)
		}

		item, err := s.Get(ctx, activeKey(id))
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.Errorf("can not deactivate cert authority %q of type %q (not a currently active ca)", id.DomainName, id.Type)
			}
			return trace.Wrap(err)
		}

		condacts = append(condacts, []backend.ConditionalAction{
			{
				Key:       activeKey(id),
				Condition: backend.Revision(item.Revision),
				Action:    backend.Delete(),
			},
			{
				Key:       inactiveKey(id),
				Condition: backend.Whatever(),
				Action:    backend.Put(*item),
			},
		}...)
	}

	if _, err := s.AtomicWrite(ctx, condacts); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.Errorf("failed to deactivate one or more cert authorities for cluster(s) %q due to concurrent modification", strings.Join(domainNames, ","))
		}
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
	item, err := s.Get(ctx, activeKey(id))
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
			s.log.Warnf("Failed to unmarshal cert authority at %q: %v", item.Key, err)
			continue
		}
		if err := services.ValidateCertAuthority(ca); err != nil {
			s.log.Warnf("Failed to validate cert authority at %q: %v", item.Key, err)
			continue
		}
		setSigningKeys(ca, loadSigningKeys)
		cas[i] = ca
	}

	return cas, nil
}

// UpdateUserCARoleMap updates the role map of the userCA of the specified existing cluster.
func (s *CA) UpdateUserCARoleMap(ctx context.Context, name string, roleMap types.RoleMap, activated bool) error {
	id := types.CertAuthID{
		Type:       types.UserCA,
		DomainName: name,
	}
	key := activeKey(id)
	if !activated {
		key = inactiveKey(id)
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

// activeKey builds the active key variant for the supplied ca id.
func activeKey(id types.CertAuthID) []byte {
	return backend.Key(authoritiesPrefix, string(id.Type), id.DomainName)
}

// inactiveKey builds the inactive key variant for the supplied ca id.
func inactiveKey(id types.CertAuthID) []byte {
	return backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName)
}

const (
	authoritiesPrefix = "authorities"
	deactivatedPrefix = "deactivated"
)
