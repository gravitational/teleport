/*
Copyright 2015-2020 Gravitational, Inc.

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
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// PresenceService records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type PresenceService struct {
	log    *logrus.Entry
	jitter utils.Jitter
	backend.Backend
}

// backendItemToResourceFunc defines a function that unmarshals a
// `backend.Item` into the implementation of `types.Resource`.
type backendItemToResourceFunc func(item backend.Item) (types.ResourceWithLabels, error)

// NewPresenceService returns new presence service instance
func NewPresenceService(b backend.Backend) *PresenceService {
	return &PresenceService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "Presence"}),
		jitter:  utils.NewJitter(),
		Backend: b,
	}
}

const (
	valPrefix = "val"
)

// UpsertLocalClusterName upserts local cluster name
func (s *PresenceService) UpsertLocalClusterName(name string) error {
	_, err := s.Put(context.TODO(), backend.Item{
		Key:   backend.Key(localClusterPrefix, valPrefix),
		Value: []byte(name),
	})
	return trace.Wrap(err)
}

// GetLocalClusterName upserts local domain
func (s *PresenceService) GetLocalClusterName() (string, error) {
	item, err := s.Get(context.TODO(), backend.Key(localClusterPrefix, valPrefix))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(item.Value), nil
}

// DeleteAllNamespaces deletes all namespaces
func (s *PresenceService) DeleteAllNamespaces() error {
	return s.DeleteRange(context.TODO(), backend.Key(namespacesPrefix), backend.RangeEnd(backend.Key(namespacesPrefix)))
}

// GetNamespaces returns a list of namespaces
func (s *PresenceService) GetNamespaces() ([]types.Namespace, error) {
	result, err := s.GetRange(context.TODO(), backend.Key(namespacesPrefix), backend.RangeEnd(backend.Key(namespacesPrefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.Namespace, 0, len(result.Items))
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			continue
		}
		ns, err := services.UnmarshalNamespace(
			item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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
	value, err := services.MarshalNamespace(n)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(namespacesPrefix, n.Metadata.Name, paramsPrefix),
		Value:   value,
		Expires: n.Metadata.Expiry(),
		ID:      n.Metadata.ID,
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
		item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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

func (s *PresenceService) getServers(ctx context.Context, kind, prefix string) ([]types.Server, error) {
	result, err := s.GetRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]types.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalServer(
			item.Value, kind,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
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
	value, err := services.MarshalServer(server)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(ctx, backend.Item{
		Key:     backend.Key(prefix, server.GetName()),
		Value:   value,
		Expires: server.Expiry(),
		ID:      server.GetResourceID(),
	})
	return trace.Wrap(err)
}

// DeleteAllNodes deletes all nodes in a namespace
func (s *PresenceService) DeleteAllNodes(ctx context.Context, namespace string) error {
	startKey := backend.Key(nodesPrefix, namespace)
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
	)
}

// GetNodes returns a list of registered servers
func (s *PresenceService) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing namespace value")
	}

	// Get all items in the bucket.
	startKey := backend.Key(nodesPrefix, namespace)
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
			services.AddOptions(opts,
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}

	return servers, nil
}

// ListNodes returns a paginated list of registered servers.
// StartKey is a resource name, which is the suffix of its key.
func (s *PresenceService) ListNodes(ctx context.Context, req proto.ListNodesRequest) (page []types.Server, nextKey string, err error) {
	// NOTE: changes to the outward behavior of this method may require updating cache.Cache.ListNodes, since that method
	// emulates this one but relies on a different implementation internally.
	if req.Namespace == "" {
		return nil, "", trace.BadParameter("missing namespace value")
	}
	limit := int(req.Limit)
	if limit <= 0 {
		return nil, "", trace.BadParameter("nonpositive limit value")
	}

	// Get all items in the bucket within the given range.
	rangeStart := backend.Key(nodesPrefix, req.Namespace, req.StartKey)
	keyPrefix := backend.Key(nodesPrefix, req.Namespace)
	rangeEnd := backend.RangeEnd(keyPrefix)

	var servers []types.Server
	err = backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, limit, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(servers) == limit {
				break
			}
			server, err := services.UnmarshalServer(
				item.Value,
				types.KindNode,
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires),
			)
			if err != nil {
				return false, trace.Wrap(err)
			}
			if server.MatchAgainst(req.Labels) {
				servers = append(servers, server)
			}
		}
		return len(servers) == limit, nil
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// If a full page was filled, set nextKey using the last node.
	if len(servers) == limit {
		nextKey = backend.NextPaginationKey(servers[len(servers)-1])
	}

	return servers, nextKey, nil
}

// UpsertNode registers node presence, permanently if TTL is 0 or for the
// specified duration with second resolution if it's >= 1 second.
func (s *PresenceService) UpsertNode(ctx context.Context, server types.Server) (*types.KeepAlive, error) {
	if server.GetNamespace() == "" {
		return nil, trace.BadParameter("missing node namespace")
	}
	value, err := services.MarshalServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := s.Put(ctx, backend.Item{
		Key:     backend.Key(nodesPrefix, server.GetNamespace(), server.GetName()),
		Value:   value,
		Expires: server.Expiry(),
		ID:      server.GetResourceID(),
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

// DELETE IN: 5.1.0.
//
// This logic has been moved to KeepAliveServer.
//
// KeepAliveNode updates node expiry
func (s *PresenceService) KeepAliveNode(ctx context.Context, h types.KeepAlive) error {
	if err := h.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	err := s.KeepAlive(ctx, backend.Lease{
		ID:  h.LeaseID,
		Key: backend.Key(nodesPrefix, h.Namespace, h.Name),
	}, h.Expires)
	return trace.Wrap(err)
}

// UpsertNodes is used for bulk insertion of nodes.
func (s *PresenceService) UpsertNodes(namespace string, servers []types.Server) error {
	batch, ok := s.Backend.(backend.Batch)
	if !ok {
		return trace.BadParameter("backend does not support batch interface")
	}
	if namespace == "" {
		return trace.BadParameter("missing node namespace")
	}

	start := time.Now()

	items := make([]backend.Item, len(servers))
	for i, server := range servers {
		value, err := services.MarshalServer(server)
		if err != nil {
			return trace.Wrap(err)
		}

		items[i] = backend.Item{
			Key:     backend.Key(nodesPrefix, server.GetNamespace(), server.GetName()),
			Value:   value,
			Expires: server.Expiry(),
			ID:      server.GetResourceID(),
		}
	}

	err := batch.PutRange(context.TODO(), items)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Debugf("UpsertNodes(%v) in %v", len(servers), time.Since(start))

	return nil
}

// GetAuthServers returns a list of registered servers
func (s *PresenceService) GetAuthServers() ([]types.Server, error) {
	return s.getServers(context.TODO(), types.KindAuthServer, authServersPrefix)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertAuthServer(server types.Server) error {
	return s.upsertServer(context.TODO(), authServersPrefix, server)
}

// DeleteAllAuthServers deletes all auth servers
func (s *PresenceService) DeleteAllAuthServers() error {
	startKey := backend.Key(authServersPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// DeleteAuthServer deletes auth server by name
func (s *PresenceService) DeleteAuthServer(name string) error {
	key := backend.Key(authServersPrefix, name)
	return s.Delete(context.TODO(), key)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertProxy(server types.Server) error {
	return s.upsertServer(context.TODO(), proxiesPrefix, server)
}

// GetProxies returns a list of registered proxies
func (s *PresenceService) GetProxies() ([]types.Server, error) {
	return s.getServers(context.TODO(), types.KindProxy, proxiesPrefix)
}

// DeleteAllProxies deletes all proxies
func (s *PresenceService) DeleteAllProxies() error {
	startKey := backend.Key(proxiesPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// DeleteProxy deletes proxy
func (s *PresenceService) DeleteProxy(name string) error {
	key := backend.Key(proxiesPrefix, name)
	return s.Delete(context.TODO(), key)
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (s *PresenceService) DeleteAllReverseTunnels() error {
	startKey := backend.Key(reverseTunnelsPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (s *PresenceService) UpsertReverseTunnel(tunnel types.ReverseTunnel) error {
	if err := services.ValidateReverseTunnel(tunnel); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalReverseTunnel(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
		Key:     backend.Key(reverseTunnelsPrefix, tunnel.GetName()),
		Value:   value,
		Expires: tunnel.Expiry(),
		ID:      tunnel.GetResourceID(),
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
		services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
}

// GetReverseTunnels returns a list of registered servers
func (s *PresenceService) GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	startKey := backend.Key(reverseTunnelsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]types.ReverseTunnel, len(result.Items))
	if len(result.Items) == 0 {
		return tunnels, nil
	}
	for i, item := range result.Items {
		tunnel, err := services.UnmarshalReverseTunnel(
			item.Value, services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
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
	value, err := services.MarshalTrustedCluster(trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = s.Put(ctx, backend.Item{
		Key:     backend.Key(trustedClustersPrefix, trustedCluster.GetName()),
		Value:   value,
		Expires: trustedCluster.Expiry(),
		ID:      trustedCluster.GetResourceID(),
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
	return services.UnmarshalTrustedCluster(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// GetTrustedClusters returns all TrustedClusters in the backend.
func (s *PresenceService) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	startKey := backend.Key(trustedClustersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.TrustedCluster, len(result.Items))
	for i, item := range result.Items {
		tc, err := services.UnmarshalTrustedCluster(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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
	if err := conn.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalTunnelConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
		Key:     backend.Key(tunnelConnectionsPrefix, conn.GetClusterName(), conn.GetName()),
		Value:   value,
		Expires: conn.Expiry(),
		ID:      conn.GetResourceID(),
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
		services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
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
	startKey := backend.Key(tunnelConnectionsPrefix, clusterName)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conns := make([]types.TunnelConnection, len(result.Items))
	for i, item := range result.Items {
		conn, err := services.UnmarshalTunnelConnection(item.Value,
			services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}

	return conns, nil
}

// GetAllTunnelConnections returns all tunnel connections
func (s *PresenceService) GetAllTunnelConnections(opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	startKey := backend.Key(tunnelConnectionsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conns := make([]types.TunnelConnection, len(result.Items))
	for i, item := range result.Items {
		conn, err := services.UnmarshalTunnelConnection(item.Value,
			services.AddOptions(opts,
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires))...)
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
	startKey := backend.Key(tunnelConnectionsPrefix, clusterName)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// DeleteAllTunnelConnections deletes all tunnel connections
func (s *PresenceService) DeleteAllTunnelConnections() error {
	startKey := backend.Key(tunnelConnectionsPrefix)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// CreateRemoteCluster creates remote cluster
func (s *PresenceService) CreateRemoteCluster(rc types.RemoteCluster) error {
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
func (s *PresenceService) UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) error {
	if err := rc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	existingItem, update, err := s.getRemoteCluster(rc.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	update.SetExpiry(rc.Expiry())
	update.SetLastHeartbeat(rc.GetLastHeartbeat())
	meta := rc.GetMetadata()
	meta.Labels = rc.GetMetadata().Labels
	update.SetMetadata(meta)

	updateValue, err := services.MarshalRemoteCluster(update)
	if err != nil {
		return trace.Wrap(err)
	}
	updateItem := backend.Item{
		Key:     backend.Key(remoteClustersPrefix, update.GetName()),
		Value:   updateValue,
		Expires: update.Expiry(),
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
func (s *PresenceService) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	startKey := backend.Key(remoteClustersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]types.RemoteCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalRemoteCluster(item.Value,
			services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters[i] = cluster
	}
	return clusters, nil
}

// getRemoteCluster returns a remote cluster in raw form and unmarshaled
func (s *PresenceService) getRemoteCluster(clusterName string) (*backend.Item, types.RemoteCluster, error) {
	if clusterName == "" {
		return nil, nil, trace.BadParameter("missing parameter cluster name")
	}
	item, err := s.Get(context.TODO(), backend.Key(remoteClustersPrefix, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, nil, trace.NotFound("remote cluster %q is not found", clusterName)
		}
		return nil, nil, trace.Wrap(err)
	}
	rc, err := services.UnmarshalRemoteCluster(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return item, rc, nil
}

// GetRemoteCluster returns a remote cluster by name
func (s *PresenceService) GetRemoteCluster(clusterName string) (types.RemoteCluster, error) {
	_, rc, err := s.getRemoteCluster(clusterName)
	return rc, trace.Wrap(err)
}

// DeleteRemoteCluster deletes remote cluster by name
func (s *PresenceService) DeleteRemoteCluster(clusterName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	return s.Delete(context.TODO(), backend.Key(remoteClustersPrefix, clusterName))
}

// DeleteAllRemoteClusters deletes all remote clusters
func (s *PresenceService) DeleteAllRemoteClusters() error {
	startKey := backend.Key(remoteClustersPrefix)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// AcquireSemaphore attempts to acquire the specified semaphore.  AcquireSemaphore will automatically handle
// retry on contention.  If the semaphore has already reached MaxLeases, or there is too much contention,
// a LimitExceeded error is returned (contention in this context means concurrent attempts to update the
// *same* semaphore, separate semaphores can be modified concurrently without issue).  Note that this function
// is the only semaphore method that handles retries internally.  This is because this method both blocks
// user-facing operations, and contains multiple different potential contention points.
func (s *PresenceService) AcquireSemaphore(ctx context.Context, req types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	// this combination of backoff parameters leads to worst-case total time spent
	// in backoff between 1200ms and 2400ms depending on jitter.  tests are in
	// place to verify that this is sufficient to resolve a 20-lease contention
	// event, which is worse than should ever occur in practice.
	const baseBackoff = time.Millisecond * 300
	const acquireAttempts int64 = 6

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
	for i := int64(0); i < acquireAttempts; i++ {
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
	value, err := services.MarshalSemaphore(sem)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     key,
		Value:   value,
		Expires: sem.Expiry(),
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

	newValue, err := services.MarshalSemaphore(sem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:     key,
		Value:   newValue,
		Expires: sem.Expiry(),
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

	newValue, err := services.MarshalSemaphore(sem)
	if err != nil {
		return trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:     key,
		Value:   newValue,
		Expires: sem.Expiry(),
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

	newValue, err := services.MarshalSemaphore(sem)
	if err != nil {
		return trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:     key,
		Value:   newValue,
		Expires: sem.Expiry(),
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
			startKey = backend.Key(semaphoresPrefix, filter.SemaphoreKind)
		} else {
			startKey = backend.Key(semaphoresPrefix)
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
		sem, err := services.UnmarshalSemaphore(item.Value)
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

// UpsertKubeService registers kubernetes service presence.
func (s *PresenceService) UpsertKubeService(ctx context.Context, server types.Server) error {
	// TODO(awly): verify that no other KubeService has the same kubernetes
	// cluster names with different labels to avoid RBAC check confusion.
	return s.upsertServer(ctx, kubeServicesPrefix, server)
}

// GetKubeServices returns a list of registered kubernetes services.
func (s *PresenceService) GetKubeServices(ctx context.Context) ([]types.Server, error) {
	return s.getServers(ctx, types.KindKubeService, kubeServicesPrefix)
}

// DeleteKubeService deletes a named kubernetes service.
func (s *PresenceService) DeleteKubeService(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("no name specified for kubernetes service deletion")
	}
	return trace.Wrap(s.Delete(ctx, backend.Key(kubeServicesPrefix, name)))
}

// DeleteAllKubeServices deletes all registered kubernetes services.
func (s *PresenceService) DeleteAllKubeServices(ctx context.Context) error {
	return trace.Wrap(s.DeleteRange(
		ctx,
		backend.Key(kubeServicesPrefix),
		backend.RangeEnd(backend.Key(kubeServicesPrefix)),
	))
}

// GetDatabaseServers returns all registered database proxy servers.
func (s *PresenceService) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing database server namespace")
	}
	startKey := backend.Key(dbServersPrefix, namespace)
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
				services.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}
	return servers, nil
}

// UpsertDatabaseServer registers new database proxy server.
func (s *PresenceService) UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error) {
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
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
		Value:   value,
		Expires: server.Expiry(),
		ID:      server.GetResourceID(),
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
	startKey := backend.Key(dbServersPrefix, namespace)
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
	legacyServers, err := s.getApplicationServersLegacy(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(servers, legacyServers...), nil
}

func (s *PresenceService) getApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	startKey := backend.Key(appServersPrefix, namespace)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]types.AppServer, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalAppServer(
			item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}
	return servers, nil
}

// getApplicationServersLegacy fetches legacy application servers that are
// represented by types.Server and adapts them to the types.AppServer type.
//
// DELETE IN 9.0.
func (s *PresenceService) getApplicationServersLegacy(ctx context.Context, namespace string) ([]types.AppServer, error) {
	legacyServers, err := s.GetAppServers(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var servers []types.AppServer
	for _, legacyServer := range legacyServers {
		appServers, err := types.NewAppServersV3FromServer(legacyServer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers = append(servers, appServers...)
	}
	return servers, nil
}

// UpsertApplicationServer registers an application server.
func (s *PresenceService) UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error) {
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
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
		Value:   value,
		Expires: server.Expiry(),
		ID:      server.GetResourceID(),
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
	startKey := backend.Key(appServersPrefix, namespace)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// GetAppServers gets all application servers.
//
// DELETE IN 9.0. Deprecated, use GetApplicationServers.
func (s *PresenceService) GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing namespace")
	}

	// Get all items in the bucket.
	startKey := backend.Key(appsPrefix, serversPrefix, namespace)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal values into a []services.Server slice.
	servers := make([]types.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := services.UnmarshalServer(
			item.Value,
			types.KindAppServer,
			services.AddOptions(opts,
				services.WithResourceID(item.ID),
				services.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}

	return servers, nil
}

// UpsertAppServer adds an application server.
//
// DELETE IN 9.0. Deprecated, use UpsertApplicationServer.
func (s *PresenceService) UpsertAppServer(ctx context.Context, server types.Server) (*types.KeepAlive, error) {
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := s.Put(ctx, backend.Item{
		Key:     backend.Key(appsPrefix, serversPrefix, server.GetNamespace(), server.GetName()),
		Value:   value,
		Expires: server.Expiry(),
		ID:      server.GetResourceID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server.Expiry().IsZero() {
		return &types.KeepAlive{}, nil
	}
	return &types.KeepAlive{
		Type:    types.KeepAlive_APP,
		LeaseID: lease.ID,
		Name:    server.GetName(),
	}, nil
}

// DeleteAppServer removes an application server.
//
// DELETE IN 9.0. Deprecated, use DeleteApplicationServer.
func (s *PresenceService) DeleteAppServer(ctx context.Context, namespace string, name string) error {
	key := backend.Key(appsPrefix, serversPrefix, namespace, name)
	return s.Delete(ctx, key)
}

// DeleteAllAppServers removes all application servers.
//
// DELETE IN 9.0. Deprecated, use DeleteAllApplicationServers.
func (s *PresenceService) DeleteAllAppServers(ctx context.Context, namespace string) error {
	startKey := backend.Key(appsPrefix, serversPrefix, namespace)
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
	startKey := backend.Key(windowsDesktopServicesPrefix, "")
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
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		srvs[i] = s
	}
	return srvs, nil
}

// UpsertWindowsDesktopService registers new Windows desktop service.
func (s *PresenceService) UpsertWindowsDesktopService(ctx context.Context, srv types.WindowsDesktopService) (*types.KeepAlive, error) {
	if err := srv.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalWindowsDesktopService(srv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := s.Put(ctx, backend.Item{
		Key:     backend.Key(windowsDesktopServicesPrefix, srv.GetName()),
		Value:   value,
		Expires: srv.Expiry(),
		ID:      srv.GetResourceID(),
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
	startKey := backend.Key(windowsDesktopServicesPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// ListResources returns a paginated list of resources.
// It implements label filtering for scenarios where the call comes directly
// here (without passing through the RBAC).
func (s *PresenceService) ListResources(ctx context.Context, req proto.ListResourcesRequest) ([]types.ResourceWithLabels, string, error) {
	limit := int(req.Limit)

	var keyPrefix []string
	var unmarshalItemFunc backendItemToResourceFunc

	switch req.ResourceType {
	case types.KindDatabaseServer:
		keyPrefix = []string{dbServersPrefix, req.Namespace}
		unmarshalItemFunc = backendItemToDatabaseServer
	case types.KindAppServer:
		keyPrefix = []string{appServersPrefix, req.Namespace}
		unmarshalItemFunc = backendItemToApplicationServer
	case types.KindNode:
		keyPrefix = []string{nodesPrefix, req.Namespace}
		unmarshalItemFunc = backendItemToServer(types.KindNode)
	case types.KindKubeService:
		keyPrefix = []string{kubeServicesPrefix}
		unmarshalItemFunc = backendItemToServer(types.KindKubeService)
	default:
		return nil, "", trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	}

	rangeStart := backend.Key(append(keyPrefix, req.StartKey)...)
	rangeEnd := backend.RangeEnd(backend.Key(keyPrefix...))

	var resources []types.ResourceWithLabels
	err := backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, limit, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(resources) == limit {
				break
			}

			resource, err := unmarshalItemFunc(item)
			if err != nil {
				return false, trace.Wrap(err)
			}

			if !types.MatchLabels(resource, req.Labels) {
				continue
			}

			resources = append(resources, resource)
		}

		return len(resources) == limit, nil
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	var nextKey string
	if len(resources) == limit {
		nextKey = backend.NextPaginationKey(resources[len(resources)-1])
	}

	return resources, nextKey, nil
}

// backendItemToDatabaseServer unmarshals `backend.Item` into a
// `types.DatabaseServer`, returning it as a `types.Resource`.
func backendItemToDatabaseServer(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalDatabaseServer(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
}

// backendItemToApplicationServer unmarshals `backend.Item` into a
// `types.AppServer`, returning it as a `types.Resource`.
func backendItemToApplicationServer(item backend.Item) (types.ResourceWithLabels, error) {
	return services.UnmarshalAppServer(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
}

// backendItemToServer returns `backendItemToResourceFunc` to unmarshal a
// `backend.Item` into a `types.ServerV2` with a specific `kind`, returning it
// as a `types.Resource`.
func backendItemToServer(kind string) backendItemToResourceFunc {
	return func(item backend.Item) (types.ResourceWithLabels, error) {
		return services.UnmarshalServer(
			item.Value, kind,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
		)
	}
}

const (
	localClusterPrefix           = "localCluster"
	reverseTunnelsPrefix         = "reverseTunnels"
	tunnelConnectionsPrefix      = "tunnelConnections"
	trustedClustersPrefix        = "trustedclusters"
	remoteClustersPrefix         = "remoteClusters"
	nodesPrefix                  = "nodes"
	appsPrefix                   = "apps"
	serversPrefix                = "servers"
	dbServersPrefix              = "databaseServers"
	appServersPrefix             = "appServers"
	namespacesPrefix             = "namespaces"
	authServersPrefix            = "authservers"
	proxiesPrefix                = "proxies"
	semaphoresPrefix             = "semaphores"
	kubeServicesPrefix           = "kubeServices"
	windowsDesktopServicesPrefix = "windowsDesktopServices"
)
