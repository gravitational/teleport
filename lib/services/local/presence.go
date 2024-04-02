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
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// PresenceService records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type PresenceService struct {
	log    *logrus.Entry
	jitter retryutils.Jitter
	backend.Backend
}

// backendItemToResourceFunc defines a function that unmarshals a
// `backend.Item` into the implementation of `types.Resource`.
type backendItemToResourceFunc func(item backend.Item) (types.ResourceWithLabels, error)

// NewPresenceService returns new presence service instance
func NewPresenceService(b backend.Backend) *PresenceService {
	return &PresenceService{
		log:     logrus.WithFields(logrus.Fields{teleport.ComponentKey: "Presence"}),
		jitter:  retryutils.NewFullJitter(),
		Backend: b,
	}
}

// DeleteAllNamespaces deletes all namespaces
func (s *PresenceService) DeleteAllNamespaces() error {
	startKey := backend.ExactKey(namespacesPrefix)
	endKey := backend.RangeEnd(startKey)
	return s.DeleteRange(context.TODO(), startKey, endKey)
}

// GetNamespaces returns a list of namespaces
func (s *PresenceService) GetNamespaces() ([]types.Namespace, error) {
	startKey := backend.ExactKey(namespacesPrefix)
	endKey := backend.RangeEnd(startKey)
	result, err := s.GetRange(context.TODO(), startKey, endKey, backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.Namespace, 0, len(result.Items))
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			continue
		}
		ns, err := services.UnmarshalNamespace(
			item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, *ns)
	}
	sort.Sort(types.SortedNamespaces(out))
	return out, nil
}

// UpsertNamespace upserts namespace
func (s *PresenceService) UpsertNamespace(n types.Namespace) error {
	if err := n.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	rev := n.GetRevision()
	value, err := services.MarshalNamespace(n)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.Key(namespacesPrefix, n.Metadata.Name, paramsPrefix),
		Value:    value,
		Expires:  n.Metadata.Expiry(),
		ID:       n.Metadata.ID,
		Revision: rev,
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetNamespace returns a namespace by name
func (s *PresenceService) GetNamespace(name string) (*types.Namespace, error) {
	if name == "" {
		return nil, trace.BadParameter("missing namespace name")
	}
	item, err := s.Get(context.TODO(), backend.Key(namespacesPrefix, name, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("namespace %q is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalNamespace(
		item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// DeleteNamespace deletes a namespace with all the keys from the backend
func (s *PresenceService) DeleteNamespace(namespace string) error {
	if namespace == "" {
		return trace.BadParameter("missing namespace name")
	}
	err := s.Delete(context.TODO(), backend.Key(namespacesPrefix, namespace, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("namespace %q is not found", namespace)
		}
	}
	return trace.Wrap(err)
}

// GetServerInfos returns a stream of ServerInfos.
func (s *PresenceService) GetServerInfos(ctx context.Context) stream.Stream[types.ServerInfo] {
	startKey := backend.ExactKey(serverInfoPrefix)
	endKey := backend.RangeEnd(startKey)
	items := backend.StreamRange(ctx, s, startKey, endKey, apidefaults.DefaultChunkSize)
	return stream.FilterMap(items, func(item backend.Item) (types.ServerInfo, bool) {
		si, err := services.UnmarshalServerInfo(
			item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			s.log.Warnf("Skipping server info at %s, failed to unmarshal: %v", item.Key, err)
			return nil, false
		}
		return si, true
	})
}

// GetServerInfo returns a ServerInfo by name.
func (s *PresenceService) GetServerInfo(ctx context.Context, name string) (types.ServerInfo, error) {
	if name == "" {
		return nil, trace.BadParameter("missing server info name")
	}
	item, err := s.Get(ctx, serverInfoKey(types.SubKindCloudInfo, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("server info %q is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	si, err := services.UnmarshalServerInfo(
		item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision),
	)
	return si, trace.Wrap(err)
}

// DeleteAllServerInfos deletes all ServerInfos.
func (s *PresenceService) DeleteAllServerInfos(ctx context.Context) error {
	startKey := backend.ExactKey(serverInfoPrefix)
	return trace.Wrap(s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// UpsertServerInfo upserts a ServerInfo.
func (s *PresenceService) UpsertServerInfo(ctx context.Context, si types.ServerInfo) error {
	if err := services.CheckAndSetDefaults(si); err != nil {
		return trace.Wrap(err)
	}
	rev := si.GetRevision()
	value, err := services.MarshalServerInfo(si)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      serverInfoKey(si.GetSubKind(), si.GetName()),
		Value:    value,
		Expires:  si.Expiry(),
		ID:       si.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	return trace.Wrap(err)
}

// DeleteServerInfo deletes a ServerInfo by name.
func (s *PresenceService) DeleteServerInfo(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing server info name")
	}
	err := s.Delete(ctx, serverInfoKey(types.SubKindCloudInfo, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("server info %q is not found", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

func serverInfoKey(subkind, name string) []byte {
	switch subkind {
	case types.SubKindCloudInfo:
		return backend.Key(serverInfoPrefix, cloudLabelsPrefix, name)
	default:
		return backend.Key(serverInfoPrefix, name)
	}
}

func (s *PresenceService) getServers(ctx context.Context, kind, prefix string) ([]types.Server, error) {
	startKey := backend.ExactKey(prefix)
	endKey := backend.RangeEnd(startKey)
	result, err := s.GetRange(ctx, startKey, endKey, backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]types.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalServer(
			item.Value, kind,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}
	// sorting helps with tests and makes it all deterministic
	sort.Sort(services.SortedServers(servers))
	return servers, nil
}

func (s *PresenceService) upsertServer(ctx context.Context, prefix string, server types.Server) error {
	rev := server.GetRevision()
	value, err := services.MarshalServer(server)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(ctx, backend.Item{
		Key:      backend.Key(prefix, server.GetName()),
		Value:    value,
		Expires:  server.Expiry(),
		ID:       server.GetResourceID(),
		Revision: rev,
	})
	return trace.Wrap(err)
}

// DeleteAllNodes deletes all nodes in a namespace
func (s *PresenceService) DeleteAllNodes(ctx context.Context, namespace string) error {
	startKey := backend.ExactKey(nodesPrefix, namespace)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// DeleteNode deletes node
func (s *PresenceService) DeleteNode(ctx context.Context, namespace string, name string) error {
	key := backend.Key(nodesPrefix, namespace, name)
	return s.Delete(ctx, key)
}

// GetNode returns a node by name and namespace.
func (s *PresenceService) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.Key(nodesPrefix, namespace, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalServer(
		item.Value,
		types.KindNode,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
}

// GetNodes returns a list of registered servers
func (s *PresenceService) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing namespace value")
	}

	// Get all items in the bucket.
	startKey := backend.ExactKey(nodesPrefix, namespace)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Marshal values into a []services.Server slice.
	servers := make([]types.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalServer(
			item.Value,
			types.KindNode,
			[]services.MarshalOption{
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires),
				services.WithRevision(item.Revision),
			}...,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}

	return servers, nil
}

// UpsertNode registers node presence, permanently if TTL is 0 or for the
// specified duration with second resolution if it's >= 1 second.
func (s *PresenceService) UpsertNode(ctx context.Context, server types.Server) (*types.KeepAlive, error) {
	if server.GetNamespace() == "" {
		server.SetNamespace(apidefaults.Namespace)
	}

	if n := server.GetNamespace(); n != apidefaults.Namespace {
		return nil, trace.BadParameter("cannot place node in namespace %q, custom namespaces are deprecated", n)
	}
	rev := server.GetRevision()
	value, err := services.MarshalServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := s.Put(ctx, backend.Item{
		Key:      backend.Key(nodesPrefix, server.GetNamespace(), server.GetName()),
		Value:    value,
		Expires:  server.Expiry(),
		ID:       server.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:    types.KeepAlive_NODE,
		LeaseID: lease.ID,
		Name:    server.GetName(),
	}, nil
}

// GetAuthServers returns a list of registered servers
func (s *PresenceService) GetAuthServers() ([]types.Server, error) {
	return s.getServers(context.TODO(), types.KindAuthServer, authServersPrefix)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertAuthServer(ctx context.Context, server types.Server) error {
	return s.upsertServer(ctx, authServersPrefix, server)
}

// DeleteAllAuthServers deletes all auth servers
func (s *PresenceService) DeleteAllAuthServers() error {
	startKey := backend.ExactKey(authServersPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// DeleteAuthServer deletes auth server by name
func (s *PresenceService) DeleteAuthServer(name string) error {
	key := backend.Key(authServersPrefix, name)
	return s.Delete(context.TODO(), key)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertProxy(ctx context.Context, server types.Server) error {
	return s.upsertServer(ctx, proxiesPrefix, server)
}

// GetProxies returns a list of registered proxies
func (s *PresenceService) GetProxies() ([]types.Server, error) {
	return s.getServers(context.TODO(), types.KindProxy, proxiesPrefix)
}

// DeleteAllProxies deletes all proxies
func (s *PresenceService) DeleteAllProxies() error {
	startKey := backend.ExactKey(proxiesPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// DeleteProxy deletes proxy
func (s *PresenceService) DeleteProxy(ctx context.Context, name string) error {
	key := backend.Key(proxiesPrefix, name)
	return s.Delete(ctx, key)
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (s *PresenceService) DeleteAllReverseTunnels() error {
	startKey := backend.ExactKey(reverseTunnelsPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (s *PresenceService) UpsertReverseTunnel(tunnel types.ReverseTunnel) error {
	if err := services.ValidateReverseTunnel(tunnel); err != nil {
		return trace.Wrap(err)
	}
	rev := tunnel.GetRevision()
	value, err := services.MarshalReverseTunnel(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
		Key:      backend.Key(reverseTunnelsPrefix, tunnel.GetName()),
		Value:    value,
		Expires:  tunnel.Expiry(),
		ID:       tunnel.GetResourceID(),
		Revision: rev,
	})
	return trace.Wrap(err)
}

// GetReverseTunnel returns reverse tunnel by name
func (s *PresenceService) GetReverseTunnel(name string, opts ...services.MarshalOption) (types.ReverseTunnel, error) {
	item, err := s.Get(context.TODO(), backend.Key(reverseTunnelsPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalReverseTunnel(item.Value,
		services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
}

// GetReverseTunnels returns a list of registered servers
func (s *PresenceService) GetReverseTunnels(ctx context.Context, opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	startKey := backend.ExactKey(reverseTunnelsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]types.ReverseTunnel, len(result.Items))
	if len(result.Items) == 0 {
		return tunnels, nil
	}
	for i, item := range result.Items {
		tunnel, err := services.UnmarshalReverseTunnel(
			item.Value, services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tunnels[i] = tunnel
	}
	// sorting helps with tests and makes it all deterministic
	sort.Sort(services.SortedReverseTunnels(tunnels))
	return tunnels, nil
}

// DeleteReverseTunnel deletes reverse tunnel by it's cluster name
func (s *PresenceService) DeleteReverseTunnel(clusterName string) error {
	err := s.Delete(context.TODO(), backend.Key(reverseTunnelsPrefix, clusterName))
	return trace.Wrap(err)
}

// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
func (s *PresenceService) UpsertTrustedCluster(ctx context.Context, trustedCluster types.TrustedCluster) (types.TrustedCluster, error) {
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
func (s *PresenceService) GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error) {
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
func (s *PresenceService) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
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
func (s *PresenceService) DeleteTrustedCluster(ctx context.Context, name string) error {
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
func (s *PresenceService) UpsertTunnelConnection(conn types.TunnelConnection) error {
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
func (s *PresenceService) GetTunnelConnection(clusterName, connectionName string, opts ...services.MarshalOption) (types.TunnelConnection, error) {
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
func (s *PresenceService) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
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
func (s *PresenceService) GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
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
func (s *PresenceService) DeleteTunnelConnection(clusterName, connectionName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if connectionName == "" {
		return trace.BadParameter("missing connection name")
	}
	return s.Delete(context.TODO(), backend.Key(tunnelConnectionsPrefix, clusterName, connectionName))
}

// DeleteTunnelConnections deletes all tunnel connections for cluster
func (s *PresenceService) DeleteTunnelConnections(clusterName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	startKey := backend.ExactKey(tunnelConnectionsPrefix, clusterName)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// DeleteAllTunnelConnections deletes all tunnel connections
func (s *PresenceService) DeleteAllTunnelConnections() error {
	startKey := backend.ExactKey(tunnelConnectionsPrefix)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// CreateRemoteCluster creates remote cluster
func (s *PresenceService) CreateRemoteCluster(
	ctx context.Context, rc types.RemoteCluster,
) (types.RemoteCluster, error) {
	value, err := json.Marshal(rc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(remoteClustersPrefix, rc.GetName()),
		Value:   value,
		Expires: rc.Expiry(),
	}
	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rc.SetRevision(lease.Revision)
	return rc, nil
}

// UpdateRemoteCluster updates selected remote cluster fields: expiry and labels
// other changed fields will be ignored by the method
func (s *PresenceService) UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error) {
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

		updateValue, err := services.MarshalRemoteCluster(existing)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		lease, err := s.ConditionalUpdate(ctx, backend.Item{
			Key:      backend.Key(remoteClustersPrefix, existing.GetName()),
			Value:    updateValue,
			Expires:  existing.Expiry(),
			Revision: existing.GetRevision(),
		})
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
func (s *PresenceService) PatchRemoteCluster(
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

		updatedValue, err := services.MarshalRemoteCluster(updated)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		lease, err := s.ConditionalUpdate(ctx, backend.Item{
			Key:      backend.Key(remoteClustersPrefix, name),
			Value:    updatedValue,
			Expires:  updated.Expiry(),
			Revision: updated.GetRevision(),
		})
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
func (s *PresenceService) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	startKey := backend.ExactKey(remoteClustersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]types.RemoteCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalRemoteCluster(item.Value,
			services.WithResourceID(item.ID),
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
func (s *PresenceService) ListRemoteClusters(
	ctx context.Context, pageSize int, pageToken string,
) ([]types.RemoteCluster, string, error) {
	rangeStart := backend.Key(remoteClustersPrefix, pageToken)
	rangeEnd := backend.RangeEnd(backend.ExactKey(remoteClustersPrefix))

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	limit := pageSize + 1

	result, err := s.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	clusters := make([]types.RemoteCluster, 0, len(result.Items))
	for _, item := range result.Items {
		cluster, err := services.UnmarshalRemoteCluster(item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			s.log.WithError(err).WithField("key", item.Key).Warn("Skipping item during ListRemoteClusters because conversion from backend item failed")
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
func (s *PresenceService) GetRemoteCluster(
	ctx context.Context, clusterName string,
) (types.RemoteCluster, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing parameter cluster name")
	}
	item, err := s.Get(ctx, backend.Key(remoteClustersPrefix, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("remote cluster %q is not found", clusterName)
		}
		return nil, trace.Wrap(err)
	}
	rc, err := services.UnmarshalRemoteCluster(item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rc, nil
}

// DeleteRemoteCluster deletes remote cluster by name
func (s *PresenceService) DeleteRemoteCluster(
	ctx context.Context, clusterName string,
) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	return s.Delete(ctx, backend.Key(remoteClustersPrefix, clusterName))
}

// DeleteAllRemoteClusters deletes all remote clusters
func (s *PresenceService) DeleteAllRemoteClusters(ctx context.Context) error {
	startKey := backend.ExactKey(remoteClustersPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// this combination of backoff parameters leads to worst-case total time spent
// in backoff between 1ms and 2000ms depending on jitter.  tests are in
// place to verify that this is sufficient to resolve a 20-lease contention
// event, which is worse than should ever occur in practice.
const (
	baseBackoff              = time.Millisecond * 400
	leaseRetryAttempts int64 = 6
)

// AcquireSemaphore attempts to acquire the specified semaphore.  AcquireSemaphore will automatically handle
// retry on contention.  If the semaphore has already reached MaxLeases, or there is too much contention,
// a LimitExceeded error is returned (contention in this context means concurrent attempts to update the
// *same* semaphore, separate semaphores can be modified concurrently without issue).  Note that this function
// is the only semaphore method that handles retries internally.  This is because this method both blocks
// user-facing operations, and contains multiple different potential contention points.
func (s *PresenceService) AcquireSemaphore(ctx context.Context, req types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Expires.Before(s.Clock().Now().UTC()) {
		return nil, trace.BadParameter("cannot acquire expired semaphore lease")
	}

	leaseID := uuid.New().String()

	// key is not modified, so allocate it once
	key := backend.Key(semaphoresPrefix, req.SemaphoreKind, req.SemaphoreName)

Acquire:
	for i := int64(0); i < leaseRetryAttempts; i++ {
		if i > 0 {
			// Not our first attempt, apply backoff. If we knew that we were only in
			// contention with one other acquire attempt we could retry immediately
			// since we got here because some other attempt *succeeded*.  It is safer,
			// however, to assume that we are under high contention and attempt to
			// spread out retries via random backoff.
			select {
			case <-time.After(s.jitter(baseBackoff * time.Duration(i))):
			case <-ctx.Done():
				return nil, trace.Wrap(ctx.Err())
			}
		}

		// attempt to acquire an existing semaphore
		lease, err := s.acquireSemaphore(ctx, key, leaseID, req)
		switch {
		case err == nil:
			// acquire was successful, return the lease.
			return lease, nil
		case trace.IsNotFound(err):
			// semaphore does not exist, attempt to perform a
			// simultaneous init+acquire.
			lease, err = s.initSemaphore(ctx, key, leaseID, req)
			if err != nil {
				if trace.IsAlreadyExists(err) {
					// semaphore was concurrently created
					continue Acquire
				}
				return nil, trace.Wrap(err)
			}
			return lease, nil
		case trace.IsCompareFailed(err):
			// semaphore was concurrently updated
			continue Acquire
		default:
			// If we get here then we encountered an error other than NotFound or CompareFailed,
			// meaning that contention isn't the issue.  No point in re-attempting.
			return nil, trace.Wrap(err)
		}
	}
	return nil, trace.LimitExceeded("too much contention on semaphore %s/%s", req.SemaphoreKind, req.SemaphoreName)
}

// initSemaphore attempts to initialize/acquire a semaphore which does not yet exist.
// Returns AlreadyExistsError if the semaphore is concurrently created.
func (s *PresenceService) initSemaphore(ctx context.Context, key []byte, leaseID string, req types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	// create a new empty semaphore resource configured to specifically match
	// this acquire request.
	sem, err := req.ConfigureSemaphore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := sem.Acquire(leaseID, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rev := sem.GetRevision()
	value, err := services.MarshalSemaphore(sem)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      key,
		Value:    value,
		Expires:  sem.Expiry(),
		Revision: rev,
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lease, nil
}

// acquireSemaphore attempts to acquire an existing semaphore.  Returns NotFoundError if no semaphore exists,
// and CompareFailed if the semaphore was concurrently updated.
func (s *PresenceService) acquireSemaphore(ctx context.Context, key []byte, leaseID string, req types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	item, err := s.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sem, err := services.UnmarshalSemaphore(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sem.RemoveExpiredLeases(s.Clock().Now().UTC())

	lease, err := sem.Acquire(leaseID, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rev := sem.GetRevision()
	newValue, err := services.MarshalSemaphore(sem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:      key,
		Value:    newValue,
		Expires:  sem.Expiry(),
		Revision: rev,
	}

	if _, err := s.CompareAndSwap(ctx, *item, newItem); err != nil {
		return nil, trace.Wrap(err)
	}
	return lease, nil
}

// KeepAliveSemaphoreLease updates semaphore lease, if the lease expiry is updated,
// semaphore is renewed
func (s *PresenceService) KeepAliveSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	if err := lease.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if lease.Expires.Before(s.Clock().Now().UTC()) {
		return trace.BadParameter("lease %v has expired at %v", lease.LeaseID, lease.Expires)
	}

	key := backend.Key(semaphoresPrefix, lease.SemaphoreKind, lease.SemaphoreName)
	item, err := s.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cannot keepalive, semaphore not found: %s/%s", lease.SemaphoreKind, lease.SemaphoreName)
		}
		return trace.Wrap(err)
	}

	sem, err := services.UnmarshalSemaphore(item.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	sem.RemoveExpiredLeases(s.Clock().Now().UTC())

	if err := sem.KeepAlive(lease); err != nil {
		return trace.Wrap(err)
	}

	rev := sem.GetRevision()
	newValue, err := services.MarshalSemaphore(sem)
	if err != nil {
		return trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:      key,
		Value:    newValue,
		Expires:  sem.Expiry(),
		Revision: rev,
	}

	_, err = s.CompareAndSwap(ctx, *item, newItem)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("semaphore %v/%v has been concurrently updated, try again", sem.GetSubKind(), sem.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// CancelSemaphoreLease cancels semaphore lease early.
func (s *PresenceService) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	if err := lease.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if lease.Expires.Before(s.Clock().Now()) {
		return trace.BadParameter("the lease %v has expired at %v", lease.LeaseID, lease.Expires)
	}

	for i := int64(0); i < leaseRetryAttempts; i++ {
		if i > 0 {
			// Not our first attempt, apply backoff. If we knew that we were only in
			// contention with one other cancel attempt we could retry immediately
			// since we got here because some other attempt *succeeded*.  It is safer,
			// however, to assume that we are under high contention and attempt to
			// spread out retries via random backoff.
			select {
			case <-time.After(s.jitter(baseBackoff * time.Duration(i))):
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		}

		key := backend.Key(semaphoresPrefix, lease.SemaphoreKind, lease.SemaphoreName)
		item, err := s.Get(ctx, key)
		if err != nil {
			return trace.Wrap(err)
		}

		sem, err := services.UnmarshalSemaphore(item.Value)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := sem.Cancel(lease); err != nil {
			return trace.Wrap(err)
		}

		rev := sem.GetRevision()
		newValue, err := services.MarshalSemaphore(sem)
		if err != nil {
			return trace.Wrap(err)
		}

		newItem := backend.Item{
			Key:      key,
			Value:    newValue,
			Expires:  sem.Expiry(),
			Revision: rev,
		}

		_, err = s.CompareAndSwap(ctx, *item, newItem)
		switch {
		case err == nil:
			return nil
		case trace.IsCompareFailed(err):
			// semaphore was concurrently updated
			continue
		default:
			return trace.Wrap(err)
		}
	}

	return trace.LimitExceeded("too much contention on semaphore %s/%s", lease.SemaphoreKind, lease.SemaphoreName)
}

// GetSemaphores returns all semaphores matching the supplied filter.
func (s *PresenceService) GetSemaphores(ctx context.Context, filter types.SemaphoreFilter) ([]types.Semaphore, error) {
	var items []backend.Item
	if filter.SemaphoreKind != "" && filter.SemaphoreName != "" {
		// special case: filter corresponds to a single semaphore
		item, err := s.Get(ctx, backend.Key(semaphoresPrefix, filter.SemaphoreKind, filter.SemaphoreName))
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, nil
			}
			return nil, trace.Wrap(err)
		}
		items = append(items, *item)
	} else {
		var startKey []byte
		if filter.SemaphoreKind != "" {
			startKey = backend.ExactKey(semaphoresPrefix, filter.SemaphoreKind)
		} else {
			startKey = backend.ExactKey(semaphoresPrefix)
		}
		result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, nil
			}
			return nil, trace.Wrap(err)
		}
		items = result.Items
	}

	sems := make([]types.Semaphore, 0, len(items))

	for _, item := range items {
		sem, err := services.UnmarshalSemaphore(item.Value, services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if filter.Match(sem) {
			sems = append(sems, sem)
		}
	}

	return sems, nil
}

// DeleteSemaphore deletes a semaphore matching the supplied filter
func (s *PresenceService) DeleteSemaphore(ctx context.Context, filter types.SemaphoreFilter) error {
	if filter.SemaphoreKind == "" || filter.SemaphoreName == "" {
		return trace.BadParameter("semaphore kind and name must be specified for deletion")
	}
	return trace.Wrap(s.Delete(ctx, backend.Key(semaphoresPrefix, filter.SemaphoreKind, filter.SemaphoreName)))
}

// UpsertKubernetesServer registers an kubernetes server.
func (s *PresenceService) UpsertKubernetesServer(ctx context.Context, server types.KubeServer) (*types.KeepAlive, error) {
	if err := services.CheckAndSetDefaults(server); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := server.GetRevision()
	value, err := services.MarshalKubeServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Since a kube server represents a single proxied cluster, there may
	// be multiple kubernetes servers on a single host, so they are stored under
	// the following path in the backend:
	//   /kubeServers/<host-uuid>/<name>
	lease, err := s.Put(ctx, backend.Item{
		Key: backend.Key(kubeServersPrefix,
			server.GetHostID(),
			server.GetName()),
		Value:    value,
		Expires:  server.Expiry(),
		ID:       server.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:      types.KeepAlive_KUBERNETES,
		LeaseID:   lease.ID,
		Name:      server.GetName(),
		Namespace: server.GetNamespace(),
		HostID:    server.GetHostID(),
		Expires:   server.Expiry(),
	}, nil
}

// DeleteKubernetesServer removes specified kubernetes server.
func (s *PresenceService) DeleteKubernetesServer(ctx context.Context, hostID, name string) error {
	if name == "" {
		return trace.BadParameter("no name specified for kubernetes server deletion")
	}
	if hostID == "" {
		return trace.BadParameter("no hostID specified for kubernetes server deletion")
	}
	key := backend.Key(kubeServersPrefix, hostID, name)
	return s.Delete(ctx, key)
}

// DeleteAllKubernetesServers removes all registered kubernetes servers.
func (s *PresenceService) DeleteAllKubernetesServers(ctx context.Context) error {
	startKey := backend.ExactKey(kubeServersPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// GetKubernetesServers returns all registered kubernetes servers.
func (s *PresenceService) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	servers, err := s.getKubernetesServers(ctx)
	return servers, trace.Wrap(err)
}

func (s *PresenceService) getKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	startKey := backend.ExactKey(kubeServersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]types.KubeServer, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalKubeServer(
			item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}
	return servers, nil
}

// GetDatabaseServers returns all registered database proxy servers.
func (s *PresenceService) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing database server namespace")
	}
	startKey := backend.ExactKey(dbServersPrefix, namespace)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]types.DatabaseServer, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalDatabaseServer(
			item.Value,
			services.AddOptions(opts,
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires),
				services.WithRevision(item.Revision))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}
	return servers, nil
}

// UpsertDatabaseServer registers new database proxy server.
func (s *PresenceService) UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error) {
	if err := services.CheckAndSetDefaults(server); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := server.GetRevision()
	value, err := services.MarshalDatabaseServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Because there may be multiple database servers on a single host,
	// they are stored under the following path in the backend:
	//   /databaseServers/<namespace>/<host-uuid>/<name>
	lease, err := s.Put(ctx, backend.Item{
		Key: backend.Key(dbServersPrefix,
			server.GetNamespace(),
			server.GetHostID(),
			server.GetName()),
		Value:    value,
		Expires:  server.Expiry(),
		ID:       server.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:      types.KeepAlive_DATABASE,
		LeaseID:   lease.ID,
		Name:      server.GetName(),
		Namespace: server.GetNamespace(),
		HostID:    server.GetHostID(),
		Expires:   server.Expiry(),
	}, nil
}

// DeleteDatabaseServer removes the specified database proxy server.
func (s *PresenceService) DeleteDatabaseServer(ctx context.Context, namespace, hostID, name string) error {
	if namespace == "" {
		return trace.BadParameter("missing database server namespace")
	}
	if hostID == "" {
		return trace.BadParameter("missing database server host ID")
	}
	if name == "" {
		return trace.BadParameter("missing database server name")
	}
	key := backend.Key(dbServersPrefix, namespace, hostID, name)
	return s.Delete(ctx, key)
}

// DeleteAllDatabaseServers removes all registered database proxy servers.
func (s *PresenceService) DeleteAllDatabaseServers(ctx context.Context, namespace string) error {
	if namespace == "" {
		return trace.BadParameter("missing database servers namespace")
	}
	startKey := backend.ExactKey(dbServersPrefix, namespace)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// GetApplicationServers returns all registered application servers.
func (s *PresenceService) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing namespace")
	}
	servers, err := s.getApplicationServers(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

func (s *PresenceService) getApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	startKey := backend.ExactKey(appServersPrefix, namespace)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]types.AppServer, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalAppServer(
			item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}
	return servers, nil
}

// UpsertApplicationServer registers an application server.
func (s *PresenceService) UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error) {
	if err := services.CheckAndSetDefaults(server); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := server.GetRevision()
	value, err := services.MarshalAppServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Since an app server represents a single proxied application, there may
	// be multiple database servers on a single host, so they are stored under
	// the following path in the backend:
	//   /appServers/<namespace>/<host-uuid>/<name>
	lease, err := s.Put(ctx, backend.Item{
		Key: backend.Key(appServersPrefix,
			server.GetNamespace(),
			server.GetHostID(),
			server.GetName()),
		Value:    value,
		Expires:  server.Expiry(),
		ID:       server.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:      types.KeepAlive_APP,
		LeaseID:   lease.ID,
		Name:      server.GetName(),
		Namespace: server.GetNamespace(),
		HostID:    server.GetHostID(),
		Expires:   server.Expiry(),
	}, nil
}

// DeleteApplicationServer removes specified application server.
func (s *PresenceService) DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error {
	key := backend.Key(appServersPrefix, namespace, hostID, name)
	return s.Delete(ctx, key)
}

// DeleteAllApplicationServers removes all registered application servers.
func (s *PresenceService) DeleteAllApplicationServers(ctx context.Context, namespace string) error {
	startKey := backend.ExactKey(appServersPrefix, namespace)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// KeepAliveServer updates expiry time of a server resource.
func (s *PresenceService) KeepAliveServer(ctx context.Context, h types.KeepAlive) error {
	if err := h.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Update the prefix off the type information in the keep alive.
	var key []byte
	switch h.GetType() {
	case constants.KeepAliveNode:
		key = backend.Key(nodesPrefix, h.Namespace, h.Name)
	case constants.KeepAliveApp:
		if h.HostID != "" {
			key = backend.Key(appServersPrefix, h.Namespace, h.HostID, h.Name)
		} else { // DELETE IN 9.0. Legacy app server is heartbeating back.
			key = backend.Key(appsPrefix, serversPrefix, h.Namespace, h.Name)
		}
	case constants.KeepAliveDatabase:
		key = backend.Key(dbServersPrefix, h.Namespace, h.HostID, h.Name)
	case constants.KeepAliveWindowsDesktopService:
		key = backend.Key(windowsDesktopServicesPrefix, h.Name)
	case constants.KeepAliveKube:
		key = backend.Key(kubeServersPrefix, h.HostID, h.Name)
	case constants.KeepAliveDatabaseService:
		key = backend.Key(databaseServicePrefix, h.Name)
	default:
		return trace.BadParameter("unknown keep-alive type %q", h.GetType())
	}

	err := s.KeepAlive(ctx, backend.Lease{
		ID:  h.LeaseID,
		Key: key,
	}, h.Expires)
	return trace.Wrap(err)
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (s *PresenceService) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	startKey := backend.ExactKey(windowsDesktopServicesPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srvs := make([]types.WindowsDesktopService, len(result.Items))
	for i, item := range result.Items {
		s, err := services.UnmarshalWindowsDesktopService(
			item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		srvs[i] = s
	}
	return srvs, nil
}

func (s *PresenceService) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	result, err := s.Get(ctx, backend.Key(windowsDesktopServicesPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	service, err := services.UnmarshalWindowsDesktopService(
		result.Value,
		services.WithResourceID(result.ID),
		services.WithExpires(result.Expires),
		services.WithRevision(result.Revision),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return service, nil
}

// UpsertWindowsDesktopService registers new Windows desktop service.
func (s *PresenceService) UpsertWindowsDesktopService(ctx context.Context, srv types.WindowsDesktopService) (*types.KeepAlive, error) {
	if err := services.CheckAndSetDefaults(srv); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := srv.GetRevision()
	value, err := services.MarshalWindowsDesktopService(srv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := s.Put(ctx, backend.Item{
		Key:      backend.Key(windowsDesktopServicesPrefix, srv.GetName()),
		Value:    value,
		Expires:  srv.Expiry(),
		ID:       srv.GetResourceID(),
		Revision: rev,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if srv.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:      types.KeepAlive_WINDOWS_DESKTOP,
		LeaseID:   lease.ID,
		Name:      srv.GetName(),
		Namespace: apidefaults.Namespace,
		Expires:   srv.Expiry(),
	}, nil
}

// DeleteWindowsDesktopService removes the specified Windows desktop service.
func (s *PresenceService) DeleteWindowsDesktopService(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing windows desktop service name")
	}
	key := backend.Key(windowsDesktopServicesPrefix, name)
	return s.Delete(ctx, key)
}

// DeleteAllWindowsDesktopServices removes all registered Windows desktop services.
func (s *PresenceService) DeleteAllWindowsDesktopServices(ctx context.Context) error {
	startKey := backend.ExactKey(windowsDesktopServicesPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// UpsertHostUserInteractionTime upserts a unix user's interaction time
func (s *PresenceService) UpsertHostUserInteractionTime(ctx context.Context, name string, loginTime time.Time) error {
	val, err := utils.FastMarshal(loginTime.UTC())
	if err != nil {
		return err
	}
	_, err = s.Put(ctx, backend.Item{
		Key:   backend.Key(loginTimePrefix, name),
		Value: val,
	})
	return trace.Wrap(err)
}

// GetHostUserInteractionTime retrieves a unix user's interaction time
func (s *PresenceService) GetHostUserInteractionTime(ctx context.Context, name string) (time.Time, error) {
	item, err := s.Get(ctx, backend.Key(loginTimePrefix, name))
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	var t time.Time
	if err := utils.FastUnmarshal(item.Value, &t); err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	return t, nil
}

// GetUserGroups returns all registered user groups.
func (s *PresenceService) GetUserGroups(ctx context.Context, opts ...services.MarshalOption) ([]types.UserGroup, error) {
	startKey := backend.ExactKey(userGroupPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userGroups := make([]types.UserGroup, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalUserGroup(
			item.Value,
			services.AddOptions(opts,
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires),
				services.WithRevision(item.Revision))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userGroups[i] = server
	}
	return userGroups, nil
}

// ListResources returns a paginated list of resources.
// It implements various filtering for scenarios where the call comes directly
// here (without passing through the RBAC).
func (s *PresenceService) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	switch {
	case req.RequiresFakePagination():
		return s.listResourcesWithSort(ctx, req)
	default:
		return s.listResources(ctx, req)
	}
}

func (s *PresenceService) listResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var keyPrefix []string
	var unmarshalItemFunc backendItemToResourceFunc

	switch req.ResourceType {
	case types.KindDatabaseServer:
		keyPrefix = []string{dbServersPrefix, req.Namespace}
		unmarshalItemFunc = backendItemToDatabaseServer
	case types.KindDatabaseService:
		keyPrefix = []string{databaseServicePrefix}
		unmarshalItemFunc = backendItemToDatabaseService
	case types.KindAppServer:
		keyPrefix = []string{appServersPrefix, req.Namespace}
		unmarshalItemFunc = backendItemToApplicationServer
	case types.KindNode:
		keyPrefix = []string{nodesPrefix, req.Namespace}
		unmarshalItemFunc = backendItemToServer(types.KindNode)
	case types.KindWindowsDesktopService:
		keyPrefix = []string{windowsDesktopServicesPrefix}
		unmarshalItemFunc = backendItemToWindowsDesktopService
	case types.KindKubeServer:
		keyPrefix = []string{kubeServersPrefix}
		unmarshalItemFunc = backendItemToKubernetesServer
	case types.KindUserGroup:
		keyPrefix = []string{userGroupPrefix}
		unmarshalItemFunc = backendItemToUserGroup
	default:
		return nil, trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	}

	rangeStart := backend.Key(append(keyPrefix, req.StartKey)...)
	rangeEnd := backend.RangeEnd(backend.ExactKey(keyPrefix...))
	filter := services.MatchResourceFilter{
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	// Get most limit+1 results to determine if there will be a next key.
	reqLimit := int(req.Limit)
	maxLimit := reqLimit + 1
	var resources []types.ResourceWithLabels
	if err := backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, maxLimit, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(resources) == maxLimit {
				break
			}

			resource, err := unmarshalItemFunc(item)
			if err != nil {
				return false, trace.Wrap(err)
			}

			switch match, err := services.MatchResourceByFilters(resource, filter, nil /* ignore dup matches */); {
			case err != nil:
				return false, trace.Wrap(err)
			case match:
				resources = append(resources, resource)
			}
		}

		return len(resources) == maxLimit, nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	var nextKey string
	if len(resources) > reqLimit {
		nextKey = backend.GetPaginationKey(resources[len(resources)-1])
		// Truncate the last item that was used to determine next row existence.
		resources = resources[:reqLimit]
	}

	return &types.ListResourcesResponse{
		Resources: resources,
		NextKey:   nextKey,
	}, nil
}

// listResourcesWithSort supports sorting by falling back to retrieving all resources
// with GetXXXs, filter, and then fake pagination.
func (s *PresenceService) listResourcesWithSort(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var resources []types.ResourceWithLabels
	switch req.ResourceType {
	case types.KindNode:
		nodes, err := s.GetNodes(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers := types.Servers(nodes)
		if err := servers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = servers.AsResources()

	case types.KindAppServer:
		appservers, err := s.GetApplicationServers(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers := types.AppServers(appservers)
		if err := servers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = servers.AsResources()

	case types.KindDatabaseServer:
		dbservers, err := s.GetDatabaseServers(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		servers := types.DatabaseServers(dbservers)
		if err := servers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = servers.AsResources()

	case types.KindKubernetesCluster:
		// GetKubernetesServers returns KubernetesServersV3 and legacy kubernetes services of type ServerV2
		kubeServers, err := s.GetKubernetesServers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Extract kube clusters into its own list.
		var clusters []types.KubeCluster
		for _, svc := range kubeServers {
			clusters = append(clusters, svc.GetCluster())
		}

		sortedClusters := types.KubeClusters(clusters)
		if err := sortedClusters.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = sortedClusters.AsResources()
	case types.KindKubeServer:
		servers, err := s.GetKubernetesServers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		kubeServers := types.KubeServers(servers)
		if err := kubeServers.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = kubeServers.AsResources()
	case types.KindUserGroup:
		userGroups, err := s.GetUserGroups(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sortedUserGroups := types.UserGroups(userGroups)
		if err := sortedUserGroups.SortByCustom(req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
		resources = sortedUserGroups.AsResources()

	default:
		return nil, trace.NotImplemented("resource type %q is not supported for ListResourcesWithSort", req.ResourceType)
	}

	params := FakePaginateParams{
		ResourceType:   req.ResourceType,
		Limit:          req.Limit,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
		StartKey:       req.StartKey,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		params.PredicateExpression = expression
	}

	return FakePaginate(resources, params)
}

// FakePaginateParams is used in FakePaginate to help filter down listing of resources into pages
// and includes required fields to support ListResources and ListUnifiedResources requests
type FakePaginateParams struct {
	// ResourceType is the resource that is going to be retrieved.
	// This only needs to be set explicitly for the `ListResources` rpc.
	ResourceType string
	// Namespace is the namespace of resources.
	Namespace string
	// Limit is the maximum amount of resources to retrieve.
	Limit int32
	// StartKey is used to start listing resources from a specific spot. It
	// should be set to the previous NextKey value if using pagination, or
	// left empty.
	StartKey string
	// Labels is a label-based matcher if non-empty.
	Labels map[string]string
	// PredicateExpression defines boolean conditions that will be matched against the resource.
	PredicateExpression typical.Expression[types.ResourceWithLabels, bool]
	// SearchKeywords is a list of search keywords to match against resource field values.
	SearchKeywords []string
	// SortBy describes which resource field and which direction to sort by.
	SortBy types.SortBy
	// WindowsDesktopFilter specifies windows desktop specific filters.
	WindowsDesktopFilter types.WindowsDesktopFilter
	// Kinds is a list of kinds to match against a resource's kind. This can be used in a
	// unified resource request that can include multiple types.
	Kinds []string
	// NeedTotalCount indicates whether or not the caller also wants the total number of resources after filtering.
	NeedTotalCount bool
	// EnrichResourceFn if provided allows the resource to be enriched with additional
	// information (logins, db names, etc.) before being added to the response.
	EnrichResourceFn func(types.ResourceWithLabels) (types.ResourceWithLabels, error)
}

// GetWindowsDesktopFilter retrieves the WindowsDesktopFilter from params
func (req *FakePaginateParams) GetWindowsDesktopFilter() types.WindowsDesktopFilter {
	if req != nil {
		return req.WindowsDesktopFilter
	}
	return types.WindowsDesktopFilter{}
}

// CheckAndSetDefaults checks and sets default values.
func (req *FakePaginateParams) CheckAndSetDefaults() error {
	if req.Namespace == "" {
		req.Namespace = apidefaults.Namespace
	}
	// If the Limit parameter was not provided instead of returning an error fallback to the default limit.
	if req.Limit == 0 {
		req.Limit = apidefaults.DefaultChunkSize
	}

	if req.Limit < 0 {
		return trace.BadParameter("negative parameter limit")
	}

	return nil
}

// FakePaginate is used when we are working with an entire list of resources upfront but still requires pagination.
// While applying filters, it will also deduplicate matches found.
func FakePaginate(resources []types.ResourceWithLabels, req FakePaginateParams) (*types.ListResourcesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	limit := int(req.Limit)
	var filtered []types.ResourceWithLabels
	filter := services.MatchResourceFilter{
		ResourceKind:        req.ResourceType,
		Labels:              req.Labels,
		SearchKeywords:      req.SearchKeywords,
		PredicateExpression: req.PredicateExpression,
		Kinds:               req.Kinds,
	}

	// Iterate and filter every resource, deduplicating while matching.
	seenResourceMap := make(map[services.ResourceSeenKey]struct{})
	for _, resource := range resources {
		switch match, err := services.MatchResourceByFilters(resource, filter, seenResourceMap); {
		case err != nil:
			return nil, trace.Wrap(err)
		case !match:
			continue
		}

		if req.EnrichResourceFn != nil {
			if enriched, err := req.EnrichResourceFn(resource); err == nil {
				resource = enriched
			}
		}

		filtered = append(filtered, resource)
	}

	totalCount := len(filtered)
	pageStart := 0
	pageEnd := limit

	// Trim resources that precede start key.
	if req.StartKey != "" {
		for i, resource := range filtered {
			if backend.GetPaginationKey(resource) == req.StartKey {
				pageStart = i
				break
			}
		}
		pageEnd = limit + pageStart
	}

	var nextKey string
	if pageEnd >= len(filtered) {
		pageEnd = len(filtered)
	} else {
		nextKey = backend.GetPaginationKey(filtered[pageEnd])
	}

	return &types.ListResourcesResponse{
		Resources:  filtered[pageStart:pageEnd],
		NextKey:    nextKey,
		TotalCount: totalCount,
	}, nil
}

// backendItemToDatabaseServer unmarshals `backend.Item` into a
// `types.DatabaseServer`, returning it as a `types.ResourceWithLabels`.
func backendItemToDatabaseServer(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalDatabaseServer(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
}

// backendItemToDatabaseService unmarshals `backend.Item` into a
// `types.DatabaseService`, returning it as a `types.ResourceWithLabels`.
func backendItemToDatabaseService(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalDatabaseService(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
}

// backendItemToApplicationServer unmarshals `backend.Item` into a
// `types.AppServer`, returning it as a `types.ResourceWithLabels`.
func backendItemToApplicationServer(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalAppServer(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
		services.WithRevision(item.Revision),
	)
}

// backendItemToKubernetesServer unmarshals `backend.Item` into a
// `types.KubeServer`, returning it as a `types.ResourceWithLabels`.
func backendItemToKubernetesServer(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalKubeServer(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
}

// backendItemToServer returns `backendItemToResourceFunc` to unmarshal a
// `backend.Item` into a `types.ServerV2` with a specific `kind`, returning it
// as a `types.ResourceWithLabels`.
func backendItemToServer(kind string) backendItemToResourceFunc {
	return func(item backend.Item) (types.ResourceWithLabels, error) {
		return services.UnmarshalServer(
			item.Value, kind,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
	}
}

// backendItemToWindowsDesktopService unmarshals `backend.Item` into a
// `types.WindowsDesktopService`, returning it as a `types.ResourceWithLabels`.
func backendItemToWindowsDesktopService(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalWindowsDesktopService(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
}

// backendItemToUserGroup unmarshals `backend.Item` into a
// `types.UserGroup`, returning it as a `types.ResourceWithLabels`.
func backendItemToUserGroup(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalUserGroup(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
}

const (
	reverseTunnelsPrefix         = "reverseTunnels"
	tunnelConnectionsPrefix      = "tunnelConnections"
	trustedClustersPrefix        = "trustedclusters"
	remoteClustersPrefix         = "remoteClusters"
	nodesPrefix                  = "nodes"
	appsPrefix                   = "apps"
	snowflakePrefix              = "snowflake"
	samlIdPPrefix                = "saml_idp" //nolint:revive // Because we want this to be IdP.
	serversPrefix                = "servers"
	dbServersPrefix              = "databaseServers"
	appServersPrefix             = "appServers"
	kubeServersPrefix            = "kubeServers"
	namespacesPrefix             = "namespaces"
	authServersPrefix            = "authservers"
	proxiesPrefix                = "proxies"
	semaphoresPrefix             = "semaphores"
	windowsDesktopServicesPrefix = "windowsDesktopServices"
	loginTimePrefix              = "hostuser_interaction_time"
	serverInfoPrefix             = "serverInfos"
	cloudLabelsPrefix            = "cloudLabels"
)
