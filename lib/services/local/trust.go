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
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sort"
	"strings"

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
	_, err := s.CreateCertAuthorities(ctx, ca)
	return trace.Wrap(err)
}

// CreateCertAuthorities creates multiple cert authorities atomically.
func (s *CA) CreateCertAuthorities(ctx context.Context, cas ...types.CertAuthority) (revision string, err error) {
	var condacts []backend.ConditionalAction
	var clusterNames []string
	for _, ca := range cas {
		if !slices.Contains(clusterNames, ca.GetName()) {
			clusterNames = append(clusterNames, ca.GetName())
		}
		if err := services.ValidateCertAuthority(ca); err != nil {
			return "", trace.Wrap(err)
		}

		item, err := caToItem(nil, ca)
		if err != nil {
			return "", trace.Wrap(err)
		}

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
	}

	rev, err := s.AtomicWrite(ctx, condacts)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return "", trace.AlreadyExists("one or more CAs from cluster(s) %q already exist", strings.Join(clusterNames, ","))
		}
		return "", trace.Wrap(err)
	}

	return rev, nil
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
	var condacts []backend.ConditionalAction
	for _, id := range ids {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
		for _, key := range [][]byte{activeCAKey(id), inactiveCAKey(id)} {
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
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.Get(ctx, activeCAKey(id))
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

// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
func (s *CA) UpsertTrustedCluster(ctx context.Context, trustedCluster types.TrustedCluster) (types.TrustedCluster, error) {
	if err := services.ValidateTrustedCluster(trustedCluster); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := trustedCluster.GetRevision()
	value, err := services.MarshalTrustedCluster(trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = s.Put(ctx, backend.Item{
		Key:      backend.Key(trustedClustersPrefix, trustedCluster.GetName()),
		Value:    value,
		Expires:  trustedCluster.Expiry(),
		ID:       trustedCluster.GetResourceID(),
		Revision: rev,
	})
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
	item, err := s.Get(ctx, backend.Key(trustedClustersPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalTrustedCluster(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
			services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
	if name == "" {
		return trace.BadParameter("missing trusted cluster name")
	}
	err := s.Delete(ctx, backend.Key(trustedClustersPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("trusted cluster %q is not found", name)
		}
	}
	return trace.Wrap(err)
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
		Key:      backend.Key(tunnelConnectionsPrefix, conn.GetClusterName(), conn.GetName()),
		Value:    value,
		Expires:  conn.Expiry(),
		ID:       conn.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTunnelConnection returns connection by cluster name and connection name
func (s *CA) GetTunnelConnection(clusterName, connectionName string, opts ...services.MarshalOption) (types.TunnelConnection, error) {
	item, err := s.Get(context.TODO(), backend.Key(tunnelConnectionsPrefix, clusterName, connectionName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("trusted cluster connection %q is not found", connectionName)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := services.UnmarshalTunnelConnection(item.Value,
		services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
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
			services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
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
				services.WithResourceID(item.ID),
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
	return s.Delete(context.TODO(), backend.Key(tunnelConnectionsPrefix, clusterName, connectionName))
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

// CreateRemoteCluster creates remote cluster
func (s *CA) CreateRemoteCluster(rc types.RemoteCluster) error {
	value, err := json.Marshal(rc)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(remoteClustersPrefix, rc.GetName()),
		Value:   value,
		Expires: rc.Expiry(),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateRemoteCluster updates selected remote cluster fields: expiry and labels
// other changed fields will be ignored by the method
func (s *CA) UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) error {
	if err := services.CheckAndSetDefaults(rc); err != nil {
		return trace.Wrap(err)
	}
	existingItem, update, err := s.getRemoteCluster(ctx, rc.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	update.SetExpiry(rc.Expiry())
	update.SetLastHeartbeat(rc.GetLastHeartbeat())
	update.SetConnectionStatus(rc.GetConnectionStatus())
	update.SetMetadata(rc.GetMetadata())

	rev := update.GetRevision()
	updateValue, err := services.MarshalRemoteCluster(update)
	if err != nil {
		return trace.Wrap(err)
	}
	updateItem := backend.Item{
		Key:      backend.Key(remoteClustersPrefix, update.GetName()),
		Value:    updateValue,
		Expires:  update.Expiry(),
		Revision: rev,
	}

	_, err = s.CompareAndSwap(ctx, *existingItem, updateItem)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("remote cluster %v has been updated by another client, try again", rc.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetRemoteClusters returns a list of remote clusters
func (s *CA) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	startKey := backend.ExactKey(remoteClustersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]types.RemoteCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalRemoteCluster(item.Value,
			services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters[i] = cluster
	}
	return clusters, nil
}

// getRemoteCluster returns a remote cluster in raw form and unmarshaled
func (s *CA) getRemoteCluster(ctx context.Context, clusterName string) (*backend.Item, types.RemoteCluster, error) {
	if clusterName == "" {
		return nil, nil, trace.BadParameter("missing parameter cluster name")
	}
	item, err := s.Get(ctx, backend.Key(remoteClustersPrefix, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, nil, trace.NotFound("remote cluster %q is not found", clusterName)
		}
		return nil, nil, trace.Wrap(err)
	}
	rc, err := services.UnmarshalRemoteCluster(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return item, rc, nil
}

// GetRemoteCluster returns a remote cluster by name
func (s *CA) GetRemoteCluster(clusterName string) (types.RemoteCluster, error) {
	_, rc, err := s.getRemoteCluster(context.TODO(), clusterName)
	return rc, trace.Wrap(err)
}

// DeleteRemoteCluster deletes remote cluster by name
func (s *CA) DeleteRemoteCluster(ctx context.Context, clusterName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	return s.Delete(ctx, backend.Key(remoteClustersPrefix, clusterName))
}

// DeleteAllRemoteClusters deletes all remote clusters
func (s *CA) DeleteAllRemoteClusters() error {
	startKey := backend.ExactKey(remoteClustersPrefix)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// catToItem builds a backend.Item corresponding to the supplied CA.
func caToItem(key []byte, ca types.CertAuthority) (backend.Item, error) {
	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:      key,
		Value:    value,
		Expires:  ca.Expiry(),
		ID:       ca.GetResourceID(),
		Revision: ca.GetRevision(),
	}, nil
}

// activeCAKey builds the active key variant for the supplied ca id.
func activeCAKey(id types.CertAuthID) []byte {
	return backend.Key(authoritiesPrefix, string(id.Type), id.DomainName)
}

// inactiveCAKey builds the inactive key variant for the supplied ca id.
func inactiveCAKey(id types.CertAuthID) []byte {
	return backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName)
}

const (
	authoritiesPrefix = "authorities"
	deactivatedPrefix = "deactivated"
)
