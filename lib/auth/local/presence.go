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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

// Presence manages Teleport components on the auth server
type Presence interface {
	auth.Presence

	// DeleteAuthServer deletes auth server by name
	DeleteAuthServer(name string) error

	// DeleteAllAuthServers deletes all auth servers
	DeleteAllAuthServers() error

	// DeleteAllNamespaces deletes all namespaces
	DeleteAllNamespaces() error

	// DeleteAllReverseTunnels deletes all reverse tunnels
	DeleteAllReverseTunnels() error

	// GetReverseTunnel returns reverse tunnel by name
	GetReverseTunnel(name string, opts ...auth.MarshalOption) (types.ReverseTunnel, error)

	// UpsertLocalClusterName upserts local domain
	UpsertLocalClusterName(name string) error
}

// PresenceService records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type PresenceService struct {
	log    *logrus.Entry
	jitter utils.Jitter
	backend.Backend
}

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
func (s *PresenceService) GetNamespaces() ([]services.Namespace, error) {
	result, err := s.GetRange(context.TODO(), backend.Key(namespacesPrefix), backend.RangeEnd(backend.Key(namespacesPrefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.Namespace, 0, len(result.Items))
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			continue
		}
		ns, err := resource.UnmarshalNamespace(
			item.Value, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, *ns)
	}
	sort.Sort(services.SortedNamespaces(out))
	return out, nil
}

// UpsertNamespace upserts namespace
func (s *PresenceService) UpsertNamespace(n services.Namespace) error {
	if err := n.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := resource.MarshalNamespace(n)
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
func (s *PresenceService) GetNamespace(name string) (*services.Namespace, error) {
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
	return resource.UnmarshalNamespace(
		item.Value, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
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

func (s *PresenceService) getServers(ctx context.Context, kind, prefix string) ([]services.Server, error) {
	result, err := s.GetRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]services.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := resource.UnmarshalServer(
			item.Value, kind,
			resource.SkipValidation(),
			resource.WithResourceID(item.ID),
			resource.WithExpires(item.Expires),
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

func (s *PresenceService) upsertServer(ctx context.Context, prefix string, server services.Server) error {
	value, err := resource.MarshalServer(server)
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
func (s *PresenceService) DeleteAllNodes(namespace string) error {
	startKey := backend.Key(nodesPrefix, namespace)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// DeleteNode deletes node
func (s *PresenceService) DeleteNode(namespace string, name string) error {
	key := backend.Key(nodesPrefix, namespace, name)
	return s.Delete(context.TODO(), key)
}

// GetNodes returns a list of registered servers
func (s *PresenceService) GetNodes(namespace string, opts ...auth.MarshalOption) ([]services.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter("missing namespace value")
	}

	// Get all items in the bucket.
	startKey := backend.Key(nodesPrefix, namespace)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Marshal values into a []services.Server slice.
	servers := make([]services.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := resource.UnmarshalServer(
			item.Value,
			services.KindNode,
			resource.AddOptions(opts,
				resource.WithResourceID(item.ID),
				resource.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}

	return servers, nil
}

// UpsertNode registers node presence, permanently if TTL is 0 or for the
// specified duration with second resolution if it's >= 1 second.
func (s *PresenceService) UpsertNode(server services.Server) (*services.KeepAlive, error) {
	if server.GetNamespace() == "" {
		return nil, trace.BadParameter("missing node namespace")
	}
	value, err := resource.MarshalServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := s.Put(context.TODO(), backend.Item{
		Key:     backend.Key(nodesPrefix, server.GetNamespace(), server.GetName()),
		Value:   value,
		Expires: server.Expiry(),
		ID:      server.GetResourceID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if server.Expiry().IsZero() {
		return &services.KeepAlive{}, nil
	}
	return &services.KeepAlive{
		Type:    services.KeepAlive_NODE,
		LeaseID: lease.ID,
		Name:    server.GetName(),
	}, nil
}

// DELETE IN: 5.1.0.
//
// This logic has been moved to KeepAliveServer.
//
// KeepAliveNode updates node expiry
func (s *PresenceService) KeepAliveNode(ctx context.Context, h services.KeepAlive) error {
	if err := h.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	err := s.KeepAlive(ctx, backend.Lease{
		ID:  h.LeaseID,
		Key: backend.Key(nodesPrefix, h.Namespace, h.Name),
	}, h.Expires)
	return trace.Wrap(err)
}

// UpsertNodes is used for bulk insertion of nodes. Schema validation is
// always skipped during bulk insertion.
func (s *PresenceService) UpsertNodes(namespace string, servers []services.Server) error {
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
		value, err := resource.MarshalServer(server)
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
func (s *PresenceService) GetAuthServers() ([]services.Server, error) {
	return s.getServers(context.TODO(), services.KindAuthServer, authServersPrefix)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertAuthServer(server services.Server) error {
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
func (s *PresenceService) UpsertProxy(server services.Server) error {
	return s.upsertServer(context.TODO(), proxiesPrefix, server)
}

// GetProxies returns a list of registered proxies
func (s *PresenceService) GetProxies() ([]services.Server, error) {
	return s.getServers(context.TODO(), services.KindProxy, proxiesPrefix)
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
func (s *PresenceService) UpsertReverseTunnel(tunnel services.ReverseTunnel) error {
	if err := auth.ValidateReverseTunnel(tunnel); err != nil {
		return trace.Wrap(err)
	}
	value, err := resource.MarshalReverseTunnel(tunnel)
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
func (s *PresenceService) GetReverseTunnel(name string, opts ...auth.MarshalOption) (services.ReverseTunnel, error) {
	item, err := s.Get(context.TODO(), backend.Key(reverseTunnelsPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resource.UnmarshalReverseTunnel(item.Value,
		resource.AddOptions(opts, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
}

// GetReverseTunnels returns a list of registered servers
func (s *PresenceService) GetReverseTunnels(opts ...auth.MarshalOption) ([]services.ReverseTunnel, error) {
	startKey := backend.Key(reverseTunnelsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]services.ReverseTunnel, len(result.Items))
	for i, item := range result.Items {
		tunnel, err := resource.UnmarshalReverseTunnel(
			item.Value, resource.AddOptions(opts, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
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
func (s *PresenceService) UpsertTrustedCluster(ctx context.Context, trustedCluster services.TrustedCluster) (services.TrustedCluster, error) {
	if err := auth.ValidateTrustedCluster(trustedCluster); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := resource.MarshalTrustedCluster(trustedCluster)
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
func (s *PresenceService) GetTrustedCluster(ctx context.Context, name string) (services.TrustedCluster, error) {
	if name == "" {
		return nil, trace.BadParameter("missing trusted cluster name")
	}
	item, err := s.Get(ctx, backend.Key(trustedClustersPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resource.UnmarshalTrustedCluster(item.Value, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
}

// GetTrustedClusters returns all TrustedClusters in the backend.
func (s *PresenceService) GetTrustedClusters(ctx context.Context) ([]services.TrustedCluster, error) {
	startKey := backend.Key(trustedClustersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.TrustedCluster, len(result.Items))
	for i, item := range result.Items {
		tc, err := resource.UnmarshalTrustedCluster(item.Value,
			resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = tc
	}

	sort.Sort(services.SortedTrustedCluster(out))
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
func (s *PresenceService) UpsertTunnelConnection(conn services.TunnelConnection) error {
	if err := conn.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := resource.MarshalTunnelConnection(conn)
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
func (s *PresenceService) GetTunnelConnection(clusterName, connectionName string, opts ...auth.MarshalOption) (services.TunnelConnection, error) {
	item, err := s.Get(context.TODO(), backend.Key(tunnelConnectionsPrefix, clusterName, connectionName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("trusted cluster connection %q is not found", connectionName)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := resource.UnmarshalTunnelConnection(item.Value,
		resource.AddOptions(opts, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// GetTunnelConnections returns connections for a trusted cluster
func (s *PresenceService) GetTunnelConnections(clusterName string, opts ...auth.MarshalOption) ([]services.TunnelConnection, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing cluster name")
	}
	startKey := backend.Key(tunnelConnectionsPrefix, clusterName)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conns := make([]services.TunnelConnection, len(result.Items))
	for i, item := range result.Items {
		conn, err := resource.UnmarshalTunnelConnection(item.Value,
			resource.AddOptions(opts, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}

	return conns, nil
}

// GetAllTunnelConnections returns all tunnel connections
func (s *PresenceService) GetAllTunnelConnections(opts ...auth.MarshalOption) ([]services.TunnelConnection, error) {
	startKey := backend.Key(tunnelConnectionsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conns := make([]services.TunnelConnection, len(result.Items))
	for i, item := range result.Items {
		conn, err := resource.UnmarshalTunnelConnection(item.Value,
			resource.AddOptions(opts,
				resource.WithResourceID(item.ID),
				resource.WithExpires(item.Expires))...)
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
func (s *PresenceService) CreateRemoteCluster(rc services.RemoteCluster) error {
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
func (s *PresenceService) UpdateRemoteCluster(ctx context.Context, rc services.RemoteCluster) error {
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

	updateValue, err := resource.MarshalRemoteCluster(update)
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
func (s *PresenceService) GetRemoteClusters(opts ...auth.MarshalOption) ([]services.RemoteCluster, error) {
	startKey := backend.Key(remoteClustersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]services.RemoteCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := resource.UnmarshalRemoteCluster(item.Value,
			resource.AddOptions(opts, resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters[i] = cluster
	}
	return clusters, nil
}

// getRemoteCluster returns a remote cluster in raw form and unmarshaled
func (s *PresenceService) getRemoteCluster(clusterName string) (*backend.Item, services.RemoteCluster, error) {
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
	rc, err := resource.UnmarshalRemoteCluster(item.Value,
		resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return item, rc, nil
}

// GetRemoteCluster returns a remote cluster by name
func (s *PresenceService) GetRemoteCluster(clusterName string) (services.RemoteCluster, error) {
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
func (s *PresenceService) AcquireSemaphore(ctx context.Context, req services.AcquireSemaphoreRequest) (*services.SemaphoreLease, error) {
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

	leaseID := uuid.New()

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
func (s *PresenceService) initSemaphore(ctx context.Context, key []byte, leaseID string, req services.AcquireSemaphoreRequest) (*services.SemaphoreLease, error) {
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
	value, err := resource.MarshalSemaphore(sem)
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
func (s *PresenceService) acquireSemaphore(ctx context.Context, key []byte, leaseID string, req services.AcquireSemaphoreRequest) (*services.SemaphoreLease, error) {
	item, err := s.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sem, err := resource.UnmarshalSemaphore(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sem.RemoveExpiredLeases(s.Clock().Now().UTC())

	lease, err := sem.Acquire(leaseID, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newValue, err := resource.MarshalSemaphore(sem)
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
func (s *PresenceService) KeepAliveSemaphoreLease(ctx context.Context, lease services.SemaphoreLease) error {
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

	sem, err := resource.UnmarshalSemaphore(item.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	sem.RemoveExpiredLeases(s.Clock().Now().UTC())

	if err := sem.KeepAlive(lease); err != nil {
		return trace.Wrap(err)
	}

	newValue, err := resource.MarshalSemaphore(sem)
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
func (s *PresenceService) CancelSemaphoreLease(ctx context.Context, lease services.SemaphoreLease) error {
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

	sem, err := resource.UnmarshalSemaphore(item.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := sem.Cancel(lease); err != nil {
		return trace.Wrap(err)
	}

	newValue, err := resource.MarshalSemaphore(sem)
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
func (s *PresenceService) GetSemaphores(ctx context.Context, filter services.SemaphoreFilter) ([]services.Semaphore, error) {
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

	sems := make([]services.Semaphore, 0, len(items))

	for _, item := range items {
		sem, err := resource.UnmarshalSemaphore(item.Value)
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
func (s *PresenceService) DeleteSemaphore(ctx context.Context, filter services.SemaphoreFilter) error {
	if filter.SemaphoreKind == "" || filter.SemaphoreName == "" {
		return trace.BadParameter("semaphore kind and name must be specified for deletion")
	}
	return trace.Wrap(s.Delete(ctx, backend.Key(semaphoresPrefix, filter.SemaphoreKind, filter.SemaphoreName)))
}

// UpsertKubeService registers kubernetes service presence.
func (s *PresenceService) UpsertKubeService(ctx context.Context, server services.Server) error {
	// TODO(awly): verify that no other KubeService has the same kubernetes
	// cluster names with different labels to avoid RBAC check confusion.
	return s.upsertServer(ctx, kubeServicesPrefix, server)
}

// GetKubeServices returns a list of registered kubernetes services.
func (s *PresenceService) GetKubeServices(ctx context.Context) ([]services.Server, error) {
	return s.getServers(ctx, services.KindKubeService, kubeServicesPrefix)
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
func (s *PresenceService) GetDatabaseServers(ctx context.Context, namespace string, opts ...auth.MarshalOption) ([]types.DatabaseServer, error) {
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
		server, err := resource.UnmarshalDatabaseServer(
			item.Value,
			resource.AddOptions(opts,
				resource.WithResourceID(item.ID),
				resource.WithExpires(item.Expires))...)
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
	value, err := resource.MarshalDatabaseServer(server)
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

// GetAppServers gets all application servers.
func (s *PresenceService) GetAppServers(ctx context.Context, namespace string, opts ...auth.MarshalOption) ([]services.Server, error) {
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
	servers := make([]services.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := resource.UnmarshalServer(
			item.Value,
			types.KindAppServer,
			resource.AddOptions(opts,
				resource.WithResourceID(item.ID),
				resource.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers[i] = server
	}

	return servers, nil
}

// UpsertAppServer adds an application server.
func (s *PresenceService) UpsertAppServer(ctx context.Context, server services.Server) (*services.KeepAlive, error) {
	if err := server.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := resource.MarshalServer(server)
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
		return &services.KeepAlive{}, nil
	}
	return &services.KeepAlive{
		Type:    services.KeepAlive_APP,
		LeaseID: lease.ID,
		Name:    server.GetName(),
	}, nil
}

// DeleteAppServer removes an application server.
func (s *PresenceService) DeleteAppServer(ctx context.Context, namespace string, name string) error {
	key := backend.Key(appsPrefix, serversPrefix, namespace, name)
	return s.Delete(ctx, key)
}

// DeleteAllAppServers removes all application servers.
func (s *PresenceService) DeleteAllAppServers(ctx context.Context, namespace string) error {
	startKey := backend.Key(appsPrefix, serversPrefix, namespace)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// KeepAliveServer updates expiry time of a server resource.
func (s *PresenceService) KeepAliveServer(ctx context.Context, h services.KeepAlive) error {
	if err := h.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Update the prefix off the type information in the keep alive.
	var key []byte
	switch h.GetType() {
	case teleport.KeepAliveNode:
		key = backend.Key(nodesPrefix, h.Namespace, h.Name)
	case teleport.KeepAliveApp:
		key = backend.Key(appsPrefix, serversPrefix, h.Namespace, h.Name)
	case teleport.KeepAliveDatabase:
		key = backend.Key(dbServersPrefix, h.Namespace, h.HostID, h.Name)
	default:
		return trace.BadParameter("unknown keep-alive type %q", h.GetType())
	}

	err := s.KeepAlive(ctx, backend.Lease{
		ID:  h.LeaseID,
		Key: key,
	}, h.Expires)
	return trace.Wrap(err)
}

const (
	localClusterPrefix      = "localCluster"
	reverseTunnelsPrefix    = "reverseTunnels"
	tunnelConnectionsPrefix = "tunnelConnections"
	trustedClustersPrefix   = "trustedclusters"
	remoteClustersPrefix    = "remoteClusters"
	nodesPrefix             = "nodes"
	appsPrefix              = "apps"
	serversPrefix           = "servers"
	dbServersPrefix         = "databaseServers"
	namespacesPrefix        = "namespaces"
	authServersPrefix       = "authservers"
	proxiesPrefix           = "proxies"
	semaphoresPrefix        = "semaphores"
	kubeServicesPrefix      = "kubeServices"
)
