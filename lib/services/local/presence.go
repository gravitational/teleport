/*
Copyright 2015 Gravitational, Inc.

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

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// PresenceService records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type PresenceService struct {
	log *logrus.Entry
	backend.Backend
}

// NewPresenceService returns new presence service instance
func NewPresenceService(b backend.Backend) *PresenceService {
	return &PresenceService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "Presence"}),
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
		ns, err := services.UnmarshalNamespace(
			item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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

func (s *PresenceService) getServers(kind, prefix string) ([]services.Server, error) {
	result, err := s.GetRange(context.TODO(), backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]services.Server, len(result.Items))
	for i, item := range result.Items {
		server, err := services.GetServerMarshaler().UnmarshalServer(
			item.Value, kind,
			services.SkipValidation(),
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

func (s *PresenceService) upsertServer(prefix string, server services.Server) error {
	value, err := services.GetServerMarshaler().MarshalServer(server)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), backend.Item{
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
func (s *PresenceService) GetNodes(namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
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
		server, err := services.GetServerMarshaler().UnmarshalServer(
			item.Value,
			services.KindNode,
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

// UpsertNode registers node presence, permanently if TTL is 0 or for the
// specified duration with second resolution if it's >= 1 second.
func (s *PresenceService) UpsertNode(server services.Server) (*services.KeepAlive, error) {
	if server.GetNamespace() == "" {
		return nil, trace.BadParameter("missing node namespace")
	}
	value, err := services.GetServerMarshaler().MarshalServer(server)
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
	return &services.KeepAlive{LeaseID: lease.ID, ServerName: server.GetName()}, nil
}

// KeepAliveNode updates node expiry
func (s *PresenceService) KeepAliveNode(ctx context.Context, h services.KeepAlive) error {
	if err := h.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	err := s.KeepAlive(ctx, backend.Lease{
		ID:  h.LeaseID,
		Key: backend.Key(nodesPrefix, h.Namespace, h.ServerName),
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
		value, err := services.GetServerMarshaler().MarshalServer(server)
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
	return s.getServers(services.KindAuthServer, authServersPrefix)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (s *PresenceService) UpsertAuthServer(server services.Server) error {
	return s.upsertServer(authServersPrefix, server)
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
	return s.upsertServer(proxiesPrefix, server)
}

// GetProxies returns a list of registered proxies
func (s *PresenceService) GetProxies() ([]services.Server, error) {
	return s.getServers(services.KindProxy, proxiesPrefix)
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
	if err := tunnel.Check(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.GetReverseTunnelMarshaler().MarshalReverseTunnel(tunnel)
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
func (s *PresenceService) GetReverseTunnel(name string, opts ...services.MarshalOption) (services.ReverseTunnel, error) {
	item, err := s.Get(context.TODO(), backend.Key(reverseTunnelsPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetReverseTunnelMarshaler().UnmarshalReverseTunnel(item.Value,
		services.AddOptions(opts, services.WithResourceID(item.ID), services.WithExpires(item.Expires))...)
}

// GetReverseTunnels returns a list of registered servers
func (s *PresenceService) GetReverseTunnels(opts ...services.MarshalOption) ([]services.ReverseTunnel, error) {
	startKey := backend.Key(reverseTunnelsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]services.ReverseTunnel, len(result.Items))
	for i, item := range result.Items {
		tunnel, err := services.GetReverseTunnelMarshaler().UnmarshalReverseTunnel(
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
func (s *PresenceService) UpsertTrustedCluster(ctx context.Context, trustedCluster services.TrustedCluster) (services.TrustedCluster, error) {
	if err := trustedCluster.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.GetTrustedClusterMarshaler().Marshal(trustedCluster)
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
func (s *PresenceService) GetTrustedCluster(name string) (services.TrustedCluster, error) {
	if name == "" {
		return nil, trace.BadParameter("missing trusted cluster name")
	}
	item, err := s.Get(context.TODO(), backend.Key(trustedClustersPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetTrustedClusterMarshaler().Unmarshal(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// GetTrustedClusters returns all TrustedClusters in the backend.
func (s *PresenceService) GetTrustedClusters() ([]services.TrustedCluster, error) {
	startKey := backend.Key(trustedClustersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.TrustedCluster, len(result.Items))
	for i, item := range result.Items {
		tc, err := services.GetTrustedClusterMarshaler().Unmarshal(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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
func (s *PresenceService) GetTunnelConnection(clusterName, connectionName string, opts ...services.MarshalOption) (services.TunnelConnection, error) {
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
func (s *PresenceService) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
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
func (s *PresenceService) GetAllTunnelConnections(opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	startKey := backend.Key(tunnelConnectionsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conns := make([]services.TunnelConnection, len(result.Items))
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

// GetRemoteClusters returns a list of remote clusters
func (s *PresenceService) GetRemoteClusters(opts ...services.MarshalOption) ([]services.RemoteCluster, error) {
	startKey := backend.Key(remoteClustersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]services.RemoteCluster, len(result.Items))
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

// GetRemoteCluster returns a remote cluster by name
func (s *PresenceService) GetRemoteCluster(clusterName string) (services.RemoteCluster, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing parameter cluster name")
	}
	item, err := s.Get(context.TODO(), backend.Key(remoteClustersPrefix, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("remote cluster %q is not found", clusterName)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalRemoteCluster(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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

const (
	localClusterPrefix      = "localCluster"
	reverseTunnelsPrefix    = "reverseTunnels"
	tunnelConnectionsPrefix = "tunnelConnections"
	trustedClustersPrefix   = "trustedclusters"
	remoteClustersPrefix    = "remoteClusters"
	nodesPrefix             = "nodes"
	namespacesPrefix        = "namespaces"
	authServersPrefix       = "authservers"
	proxiesPrefix           = "proxies"
)
