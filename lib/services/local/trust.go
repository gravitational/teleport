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
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
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
	_, err := s.CreateCertAuthorities(ctx, ca)
	return trace.Wrap(err)
}

// CreateCertAuthorities creates multiple cert authorities atomically.
func (s *CA) CreateCertAuthorities(ctx context.Context, cas ...types.CertAuthority) (revision string, err error) {
	condacts, err := createCertAuthoritiesCondActs(cas, true /* active */)
	if err != nil {
		return "", trace.Wrap(err)
	}

	rev, err := s.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			var clusterNames []string
			for _, ca := range cas {
				if slices.Contains(clusterNames, ca.GetClusterName()) {
					continue
				}
				clusterNames = append(clusterNames, ca.GetClusterName())
			}
			return "", trace.AlreadyExists("one or more CAs from cluster(s) %q already exist", strings.Join(clusterNames, ","))
		}
		return "", trace.Wrap(err)
	}

	return rev, nil
}

// createCertAuthoritiesCondActs sets up conditional actions for creating a set of CAs.
func createCertAuthoritiesCondActs(cas []types.CertAuthority, active bool) ([]backend.ConditionalAction, error) {
	condacts := make([]backend.ConditionalAction, 0, len(cas)*2)
	for _, ca := range cas {
		if err := services.ValidateCertAuthority(ca); err != nil {
			return nil, trace.Wrap(err)
		}

		item, err := caToItem(backend.Key{}, ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if active {
			// for an enabled tc, we perform a conditional create for the active CA key
			// and an unconditional delete for the inactive CA key since the active range
			// is given priority over the inactive range.
			condacts = append(condacts, []backend.ConditionalAction{
				{
					Key:       activeCAKey(ca.GetID()),
					Condition: backend.NotExists(),
					Action:    backend.Put(item),
				},
				{
					Key:       inactiveCAKey(ca.GetID()),
					Condition: backend.Whatever(),
					Action:    backend.Delete(),
				},
			}...)
		} else {
			// for a disabled tc, we perform a conditional create for the inactive CA key
			// and assert the non-existence of the active CA key.
			condacts = append(condacts, []backend.ConditionalAction{
				{
					Key:       inactiveCAKey(ca.GetID()),
					Condition: backend.NotExists(),
					Action:    backend.Put(item),
				},
				{
					Key:       activeCAKey(ca.GetID()),
					Condition: backend.NotExists(),
					Action:    backend.Nop(),
				},
			}...)
		}
	}

	return condacts, nil
}

func updateCertAuthoritiesCondActs(cas []types.CertAuthority, active bool, currentlyActive bool) ([]backend.ConditionalAction, error) {
	condacts := make([]backend.ConditionalAction, 0, len(cas)*2)
	for _, ca := range cas {
		if err := services.ValidateCertAuthority(ca); err != nil {
			return nil, trace.Wrap(err)
		}

		item, err := caToItem(backend.Key{}, ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if active {
			if currentlyActive {
				// we are updating an active CA without changing its active status. we want to perform
				// a conditional update on the acitve CA key and an unconditonal delete on the inactive
				// CA key in order to correctly model active range priority.
				condacts = append(condacts, []backend.ConditionalAction{
					{
						Key:       activeCAKey(ca.GetID()),
						Condition: backend.Revision(item.Revision),
						Action:    backend.Put(item),
					},
					{
						Key:       inactiveCAKey(ca.GetID()),
						Condition: backend.Whatever(),
						Action:    backend.Delete(),
					},
				}...)
			} else {
				// we are updating a currently inactive CA to the active state. we want to perform
				// a create on the active CA key and a revision-conditional delete on the inactive CA key
				// to affect a "move-and-update" that respects the active range priority.
				condacts = append(condacts, []backend.ConditionalAction{
					{
						Key:       activeCAKey(ca.GetID()),
						Condition: backend.NotExists(),
						Action:    backend.Put(item),
					},
					{
						Key:       inactiveCAKey(ca.GetID()),
						Condition: backend.Revision(item.Revision),
						Action:    backend.Delete(),
					},
				}...)
			}
		} else {
			if currentlyActive {
				// we are updating an active CA to the inactive state. we want to perform a conditional
				// delete on the active CA key and an unconditional put on the inactive CA key to
				// affect a "move-and-update" that respects the active range priority.
				condacts = append(condacts, []backend.ConditionalAction{
					{
						Key:       activeCAKey(ca.GetID()),
						Condition: backend.Revision(item.Revision),
						Action:    backend.Delete(),
					},
					{
						Key:       inactiveCAKey(ca.GetID()),
						Condition: backend.Whatever(),
						Action:    backend.Put(item),
					},
				}...)

			} else {
				// we are updating an inactive CA without changing its active status. we want to perform
				// a conditional update on the inactive CA key and assert the non-existence of the active
				// CA key.
				condacts = append(condacts, []backend.ConditionalAction{
					{
						Key:       inactiveCAKey(ca.GetID()),
						Condition: backend.Revision(item.Revision),
						Action:    backend.Put(item),
					},
					{
						Key:       activeCAKey(ca.GetID()),
						Condition: backend.NotExists(),
						Action:    backend.Nop(),
					},
				}...)
			}
		}
	}

	return condacts, nil
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (s *CA) UpsertCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}

	item, err := caToItem(activeCAKey(ca.GetID()), ca)
	if err != nil {
		return trace.Wrap(err)
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

	item, err := caToItem(activeCAKey(ca.GetID()), ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ca = ca.Clone()
	ca.SetRevision(lease.Revision)
	return ca, nil
}

// CompareAndSwapCertAuthority updates the cert authority value
// if the existing value matches expected parameter, returns nil if succeeds,
// trace.CompareFailed otherwise.
func (s *CA) CompareAndSwapCertAuthority(new, expected types.CertAuthority) error {
	if err := services.ValidateCertAuthority(new); err != nil {
		return trace.Wrap(err)
	}

	key := backend.NewKey(authoritiesPrefix, string(new.GetType()), new.GetName())

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

	newItem, err := caToItem(key, new)
	if err != nil {
		return trace.Wrap(err)
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
	_, err := s.AtomicWrite(ctx, s.deleteCertAuthoritiesCondActs(ids))
	return trace.Wrap(err)
}

func (s *CA) deleteCertAuthoritiesCondActs(ids []types.CertAuthID) []backend.ConditionalAction {
	var condacts []backend.ConditionalAction
	for _, id := range ids {
		if err := id.Check(); err != nil {
			continue
		}
		for _, key := range []backend.Key{activeCAKey(id), inactiveCAKey(id)} {
			condacts = append(condacts, backend.ConditionalAction{
				Key:       key,
				Condition: backend.Whatever(),
				Action:    backend.Delete(),
			})
		}
	}
	return condacts
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

		item, err := s.Get(ctx, inactiveCAKey(id))
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.Errorf("can not activate cert authority %q of type %q (not a currently inactive ca)", id.DomainName, id.Type)
			}
			return trace.Wrap(err)
		}

		condacts = append(condacts, []backend.ConditionalAction{
			{
				Key:       inactiveCAKey(id),
				Condition: backend.Revision(item.Revision),
				Action:    backend.Delete(),
			},
			{
				Key: activeCAKey(id),
				// active CAs take priority over inactive CAs, so never overwrite an
				// active CA with an inactive CA.
				Condition: backend.NotExists(),
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

		item, err := s.Get(ctx, activeCAKey(id))
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.Errorf("can not deactivate cert authority %q of type %q (not a currently active ca)", id.DomainName, id.Type)
			}
			return trace.Wrap(err)
		}

		condacts = append(condacts, []backend.ConditionalAction{
			{
				Key:       activeCAKey(id),
				Condition: backend.Revision(item.Revision),
				Action:    backend.Delete(),
			},
			{
				Key: inactiveCAKey(id),
				// active CAs always take priority over inactive CAs, so deactivating
				// an active CA should overwrite any dangling inactive CAs.
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
	return s.getCertAuthority(ctx, id, loadSigningKeys, true /* active */)
}

// GetInactiveCertAuthority returns inactive certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded.
func (s *CA) GetInactiveCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	return s.getCertAuthority(ctx, id, loadSigningKeys, false /* inactive */)
}

func (s *CA) getCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool, active bool) (types.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	key := activeCAKey(id)
	if !active {
		key = inactiveCAKey(id)
	}

	item, err := s.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := services.UnmarshalCertAuthority(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
		ca, err := services.UnmarshalCertAuthority(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			slog.WarnContext(ctx, "Failed to unmarshal cert authority", "key", item.Key, "error", err)
			continue
		}
		if err := services.ValidateCertAuthority(ca); err != nil {
			slog.WarnContext(ctx, "Failed to validate cert authority", "key", item.Key, "error", err)
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
	key := activeCAKey(id)
	if !activated {
		key = inactiveCAKey(id)
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

	newItem, err := caToItem(key, actual)
	if err != nil {
		return trace.Wrap(err)
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

// CreateTrustedCluster atomically creates a new trusted cluster along with associated resources.
func (s *CA) CreateTrustedCluster(ctx context.Context, tc types.TrustedCluster, cas []types.CertAuthority) (revision string, err error) {
	if err := services.ValidateTrustedCluster(tc); err != nil {
		return "", trace.Wrap(err)
	}

	item, err := trustedClusterToItem(tc)
	if err != nil {
		return "", trace.Wrap(err)
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(item),
		},
		// also assert that no remote cluster exists by this name, as
		// we currently do not allow for a trusted cluster and remote
		// cluster to share a name (CAs end up stored at the same location).
		{
			Key:       remoteClusterKey(tc.GetName()),
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
	}

	// perform some initial trusted-cluster related validation. common ca validation is handled later
	// on by the createCertAuthoritiesCondActs helper.
	for _, ca := range cas {
		if tc.GetName() != ca.GetClusterName() {
			return "", trace.BadParameter("trusted cluster name %q does not match CA cluster name %q", tc.GetName(), ca.GetClusterName())
		}
	}

	ccas, err := createCertAuthoritiesCondActs(cas, tc.GetEnabled())
	if err != nil {
		return "", trace.Wrap(err)
	}

	condacts = append(condacts, ccas...)

	rev, err := s.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			if _, err := s.GetRemoteCluster(ctx, tc.GetName()); err == nil {
				return "", trace.BadParameter("cannot create trusted cluster with same name as remote cluster %q, bidirectional trust is not supported", tc.GetName())
			}

			return "", trace.AlreadyExists("trusted cluster %q and/or one or more of its cert authorities already exists", tc.GetName())
		}
		return "", trace.Wrap(err)
	}

	return rev, nil
}

// UpdateTrustedCluster atomically updates a trusted cluster along with associated resources.
func (s *CA) UpdateTrustedCluster(ctx context.Context, tc types.TrustedCluster, cas []types.CertAuthority) (revision string, err error) {
	if err := services.ValidateTrustedCluster(tc); err != nil {
		return "", trace.Wrap(err)
	}

	// fetch the current state. we'll need this later on to correctly construct our CA condacts, and
	// it doesn't hurt to reject mismatched revisions early.
	extant, err := s.GetTrustedCluster(ctx, tc.GetName())
	if err != nil {
		return "", trace.Wrap(err)
	}

	if tc.GetRevision() != extant.GetRevision() {
		return "", trace.CompareFailed("trusted cluster %q has been modified, please retry", tc.GetName())
	}

	item, err := trustedClusterToItem(tc)
	if err != nil {
		return "", trace.Wrap(err)
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: backend.Revision(item.Revision),
			Action:    backend.Put(item),
		},
	}

	// perform some initial trusted-cluster related validation. common ca validation is handled later
	// on by the createCertAuthoritiesCondActs helper.
	for _, ca := range cas {
		if tc.GetName() != ca.GetClusterName() {
			return "", trace.BadParameter("trusted cluster name %q does not match CA cluster name %q", tc.GetName(), ca.GetClusterName())
		}
	}

	ccas, err := updateCertAuthoritiesCondActs(cas, tc.GetEnabled(), extant.GetEnabled())
	if err != nil {
		return "", trace.Wrap(err)
	}

	condacts = append(condacts, ccas...)

	rev, err := s.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return "", trace.CompareFailed("trusted cluster %q and/or one or more of its cert authorities have been modified, please retry", tc.GetName())
		}
		return "", trace.Wrap(err)
	}

	return rev, nil
}

// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
func (s *CA) UpsertTrustedCluster(ctx context.Context, trustedCluster types.TrustedCluster) (types.TrustedCluster, error) {
	if err := services.ValidateTrustedCluster(trustedCluster); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := trustedClusterToItem(trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return trustedCluster, nil
}

// GetTrustedCluster returns a single TrustedCluster by name.
func (s *CA) GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error) {
	if name == "" {
		return nil, trace.BadParameter("missing trusted cluster name")
	}
	item, err := s.Get(ctx, backend.NewKey(trustedClustersPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalTrustedCluster(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// GetTrustedClusters returns all TrustedClusters in the backend.
func (s *CA) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	startKey := backend.ExactKey(trustedClustersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.TrustedCluster, len(result.Items))
	for i, item := range result.Items {
		tc, err := services.UnmarshalTrustedCluster(item.Value,
			services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = tc
	}

	sort.Sort(types.SortedTrustedCluster(out))
	return out, nil
}

// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
func (s *CA) DeleteTrustedCluster(ctx context.Context, name string) error {
	return s.DeleteTrustedClusterInternal(ctx, name, nil /* no cert authorities */)
}

// DeleteTrustedClusterInternal removes a trusted cluster and associated resources atomically.
func (s *CA) DeleteTrustedClusterInternal(ctx context.Context, name string, caIDs []types.CertAuthID) error {
	if name == "" {
		return trace.BadParameter("missing trusted cluster name")
	}

	for _, id := range caIDs {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}

		if id.DomainName != name {
			return trace.BadParameter("ca %q does not belong to trusted cluster %q", id.DomainName, name)
		}
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       backend.NewKey(trustedClustersPrefix, name),
			Condition: backend.Exists(),
			Action:    backend.Delete(),
		},
	}

	condacts = append(condacts, s.deleteCertAuthoritiesCondActs(caIDs)...)

	if _, err := s.AtomicWrite(ctx, condacts); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.NotFound("trusted cluster %q is not found", name)
		}

		return trace.Wrap(err)
	}

	return nil
}

// UpsertTunnelConnection updates or creates tunnel connection
func (s *CA) UpsertTunnelConnection(conn types.TunnelConnection) error {
	if err := services.CheckAndSetDefaults(conn); err != nil {
		return trace.Wrap(err)
	}

	rev := conn.GetRevision()
	value, err := services.MarshalTunnelConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
		Key:      backend.NewKey(tunnelConnectionsPrefix, conn.GetClusterName(), conn.GetName()),
		Value:    value,
		Expires:  conn.Expiry(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTunnelConnection returns connection by cluster name and connection name
func (s *CA) GetTunnelConnection(clusterName, connectionName string, opts ...services.MarshalOption) (types.TunnelConnection, error) {
	item, err := s.Get(context.TODO(), backend.NewKey(tunnelConnectionsPrefix, clusterName, connectionName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("trusted cluster connection %q is not found", connectionName)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := services.UnmarshalTunnelConnection(item.Value,
		services.AddOptions(opts, services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// GetTunnelConnections returns connections for a trusted cluster
func (s *CA) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing cluster name")
	}
	startKey := backend.ExactKey(tunnelConnectionsPrefix, clusterName)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conns := make([]types.TunnelConnection, len(result.Items))
	for i, item := range result.Items {
		conn, err := services.UnmarshalTunnelConnection(item.Value,
			services.AddOptions(opts, services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}

	return conns, nil
}

// GetAllTunnelConnections returns all tunnel connections
func (s *CA) GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	startKey := backend.ExactKey(tunnelConnectionsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conns := make([]types.TunnelConnection, len(result.Items))
	for i, item := range result.Items {
		conn, err := services.UnmarshalTunnelConnection(item.Value,
			services.AddOptions(opts,
				services.WithExpires(item.Expires),
				services.WithRevision(item.Revision))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}

	return conns, nil
}

// DeleteTunnelConnection deletes tunnel connection by name
func (s *CA) DeleteTunnelConnection(clusterName, connectionName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if connectionName == "" {
		return trace.BadParameter("missing connection name")
	}
	return s.Delete(context.TODO(), backend.NewKey(tunnelConnectionsPrefix, clusterName, connectionName))
}

// DeleteTunnelConnections deletes all tunnel connections for cluster
func (s *CA) DeleteTunnelConnections(clusterName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	startKey := backend.ExactKey(tunnelConnectionsPrefix, clusterName)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// DeleteAllTunnelConnections deletes all tunnel connections
func (s *CA) DeleteAllTunnelConnections() error {
	startKey := backend.ExactKey(tunnelConnectionsPrefix)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// CreateRemoteCluster creates a remote cluster
func (s *CA) CreateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error) {
	rev, err := s.CreateRemoteClusterInternal(ctx, rc, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc.SetRevision(rev)
	return rc, nil
}

// CreateRemoteCluster atomically creates a new remote cluster along with associated resources.
func (s *CA) CreateRemoteClusterInternal(ctx context.Context, rc types.RemoteCluster, cas []types.CertAuthority) (revision string, err error) {
	if err := services.CheckAndSetDefaults(rc); err != nil {
		return "", trace.Wrap(err)
	}

	item, err := remoteClusterToItem(rc)
	if err != nil {
		return "", trace.Wrap(err)
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       item.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(item),
		},
		// also assert that no trusted cluster exists by this name, as
		// we currently do not allow for a trusted cluster and remote
		// cluster to share a name (CAs end up stored at the same location).
		{
			Key:       trustedClusterKey(rc.GetName()),
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
	}

	// perform some initial remote-cluster related validation. common ca validation is handled later
	// on by the createCertAuthoritiesCondActs helper.
	for _, ca := range cas {
		if rc.GetName() != ca.GetClusterName() {
			return "", trace.BadParameter("remote cluster name %q does not match CA cluster name %q", rc.GetName(), ca.GetClusterName())
		}
	}

	ccas, err := createCertAuthoritiesCondActs(cas, true /* remote cluster cas always considered active */)
	if err != nil {
		return "", trace.Wrap(err)
	}

	condacts = append(condacts, ccas...)

	rev, err := s.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			if _, err := s.GetTrustedCluster(ctx, rc.GetName()); err == nil {
				return "", trace.BadParameter("cannot create remote cluster with same name as trusted cluster %q, bidirectional trust is not supported", rc.GetName())
			}
			return "", trace.AlreadyExists("remote cluster %q and/or one or more of its cert authorities already exists", rc.GetName())
		}
		return "", trace.Wrap(err)
	}

	return rev, nil
}

// UpdateRemoteCluster updates selected remote cluster fields: expiry and labels
// other changed fields will be ignored by the method
func (s *CA) UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error) {
	if err := services.CheckAndSetDefaults(rc); err != nil {
		return nil, trace.Wrap(err)
	}

	// Small retry loop to catch cases where there's a concurrent update which
	// could cause conditional update to fail. This is needed because of the
	// unusual way updates are handled in this method meaning that the revision
	// in the provided remote cluster is not used. We should eventually make a
	// breaking change to this behavior.
	const iterationLimit = 3
	for i := 0; i < iterationLimit; i++ {
		existing, err := s.GetRemoteCluster(ctx, rc.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		existing.SetExpiry(rc.Expiry())
		existing.SetLastHeartbeat(rc.GetLastHeartbeat())
		existing.SetConnectionStatus(rc.GetConnectionStatus())
		existing.SetMetadata(rc.GetMetadata())

		item, err := remoteClusterToItem(existing)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		lease, err := s.ConditionalUpdate(ctx, item)
		if err != nil {
			if trace.IsCompareFailed(err) {
				// Retry!
				continue
			}
			return nil, trace.Wrap(err)
		}
		existing.SetRevision(lease.Revision)
		return existing, nil
	}
	return nil, trace.CompareFailed("failed to update remote cluster within %v iterations", iterationLimit)
}

// PatchRemoteCluster fetches a remote cluster and then calls updateFn
// to apply any changes, before persisting the updated remote cluster.
func (s *CA) PatchRemoteCluster(
	ctx context.Context,
	name string,
	updateFn func(types.RemoteCluster) (types.RemoteCluster, error),
) (types.RemoteCluster, error) {
	// Retry to update the remote cluster in case of a conflict.
	const iterationLimit = 3
	for i := 0; i < 3; i++ {
		existing, err := s.GetRemoteCluster(ctx, name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		updated, err := updateFn(existing.Clone())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch {
		case updated.GetName() != name:
			return nil, trace.BadParameter("metadata.name: cannot be patched")
		case updated.GetRevision() != existing.GetRevision():
			// We don't allow revision to be specified when performing a patch.
			// This is because it creates some complex semantics. Instead they
			// should use the Update method if they wish to specify the
			// revision.
			return nil, trace.BadParameter("metadata.revision: cannot be patched")
		}

		item, err := remoteClusterToItem(updated)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		lease, err := s.ConditionalUpdate(ctx, item)
		if err != nil {
			if trace.IsCompareFailed(err) {
				// Retry!
				continue
			}
			return nil, trace.Wrap(err)
		}
		updated.SetRevision(lease.Revision)
		return updated, nil
	}
	return nil, trace.CompareFailed("failed to update remote cluster within %v iterations", iterationLimit)
}

// GetRemoteClusters returns a list of remote clusters
// Prefer ListRemoteClusters. This will eventually be deprecated.
// TODO(noah): REMOVE IN 17.0.0 - replace calls with ListRemoteClusters
func (s *CA) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	startKey := backend.ExactKey(remoteClustersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]types.RemoteCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalRemoteCluster(item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters[i] = cluster
	}
	return clusters, nil
}

// ListRemoteClusters returns a page of remote clusters
func (s *CA) ListRemoteClusters(
	ctx context.Context, pageSize int, pageToken string,
) ([]types.RemoteCluster, string, error) {
	rangeStart := backend.NewKey(remoteClustersPrefix, pageToken)
	rangeEnd := backend.RangeEnd(backend.ExactKey(remoteClustersPrefix))

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > defaults.DefaultChunkSize {
		pageSize = defaults.DefaultChunkSize
	}

	limit := pageSize + 1

	result, err := s.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	clusters := make([]types.RemoteCluster, 0, len(result.Items))
	for _, item := range result.Items {
		cluster, err := services.UnmarshalRemoteCluster(item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			slog.WarnContext(ctx, "Skipping item during ListRemoteClusters because conversion from backend item failed", "key", item.Key, "error", err)
			continue
		}
		clusters = append(clusters, cluster)
	}

	next := ""
	if len(clusters) > pageSize {
		next = backend.GetPaginationKey(clusters[pageSize])
		clear(clusters[pageSize:])
		// Truncate the last item that was used to determine next row existence.
		clusters = clusters[:pageSize]
	}
	return clusters, next, nil
}

// GetRemoteCluster returns a remote cluster by name
func (s *CA) GetRemoteCluster(
	ctx context.Context, clusterName string,
) (types.RemoteCluster, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing parameter cluster name")
	}
	item, err := s.Get(ctx, backend.NewKey(remoteClustersPrefix, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("remote cluster %q is not found", clusterName)
		}
		return nil, trace.Wrap(err)
	}
	rc, err := services.UnmarshalRemoteCluster(item.Value,
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rc, nil
}

// DeleteRemoteCluster deletes remote cluster by name
func (s *CA) DeleteRemoteCluster(ctx context.Context, clusterName string) error {
	return s.DeleteRemoteClusterInternal(ctx, clusterName, nil /* no cert authorities */)
}

// DeleteRemoteClusterInternal atomically deletes a remote cluster along with associated resources.
func (s *CA) DeleteRemoteClusterInternal(ctx context.Context, name string, ids []types.CertAuthID) error {
	if name == "" {
		return trace.BadParameter("missing parameter cluster name")
	}

	for _, id := range ids {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}

		if id.DomainName != name {
			return trace.BadParameter("ca %q does not belong to remote cluster %q", id.DomainName, name)
		}
	}

	condacts := []backend.ConditionalAction{
		{
			Key:       remoteClusterKey(name),
			Condition: backend.Exists(),
			Action:    backend.Delete(),
		},
	}

	condacts = append(condacts, s.deleteCertAuthoritiesCondActs(ids)...)

	if _, err := s.AtomicWrite(ctx, condacts); err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return trace.NotFound("remote cluster %q is not found", name)
		}
		return trace.Wrap(err)
	}

	return nil
}

// DeleteAllRemoteClusters deletes all remote clusters
func (s *CA) DeleteAllRemoteClusters(ctx context.Context) error {
	startKey := backend.ExactKey(remoteClustersPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// catToItem builds a backend.Item corresponding to the supplied CA.
func caToItem(key backend.Key, ca types.CertAuthority) (backend.Item, error) {
	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      key,
		Value:    value,
		Expires:  ca.Expiry(),
		Revision: ca.GetRevision(),
	}, nil
}

func trustedClusterToItem(tc types.TrustedCluster) (backend.Item, error) {
	value, err := services.MarshalTrustedCluster(tc)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      trustedClusterKey(tc.GetName()),
		Value:    value,
		Expires:  tc.Expiry(),
		Revision: tc.GetRevision(),
	}, nil
}

func trustedClusterKey(name string) backend.Key {
	return backend.NewKey(trustedClustersPrefix, name)
}

func remoteClusterToItem(rc types.RemoteCluster) (backend.Item, error) {
	value, err := services.MarshalRemoteCluster(rc)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      remoteClusterKey(rc.GetName()),
		Value:    value,
		Expires:  rc.Expiry(),
		Revision: rc.GetRevision(),
	}, nil
}

func remoteClusterKey(name string) backend.Key {
	return backend.NewKey(remoteClustersPrefix, name)
}

// activeCAKey builds the active key variant for the supplied ca id.
func activeCAKey(id types.CertAuthID) backend.Key {
	return backend.NewKey(authoritiesPrefix, string(id.Type), id.DomainName)
}

// inactiveCAKey builds the inactive key variant for the supplied ca id.
func inactiveCAKey(id types.CertAuthID) backend.Key {
	return backend.NewKey(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName)
}

const (
	authoritiesPrefix = "authorities"
	deactivatedPrefix = "deactivated"
)
