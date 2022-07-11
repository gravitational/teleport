// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"

	"github.com/gravitational/trace"
)

// New creates an instance of Daemon service
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg:      &cfg,
		gateways: make(map[string]*gateway.Gateway),
	}, nil
}

// ListRootClusters returns a list of root clusters
func (s *Service) ListRootClusters(ctx context.Context) ([]*clusters.Cluster, error) {
	clusters, err := s.cfg.Storage.ReadAll()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clusters, nil
}

// ListLeafClusters returns a list of leaf clusters
func (s *Service) ListLeafClusters(ctx context.Context, uri string) ([]clusters.LeafCluster, error) {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// leaf cluster cannot have own leaves
	if cluster.URI.GetLeafClusterName() != "" {
		return nil, nil
	}

	leaves, err := cluster.GetLeafClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return leaves, nil
}

// AddCluster adds a cluster
func (s *Service) AddCluster(ctx context.Context, webProxyAddress string) (*clusters.Cluster, error) {
	cluster, err := s.cfg.Storage.Add(ctx, webProxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// RemoveCluster removes cluster
func (s *Service) RemoveCluster(ctx context.Context, uri string) error {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.Connected() {
		if err := cluster.Logout(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := s.cfg.Storage.Remove(ctx, cluster.Name); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ResolveCluster resolves a cluster by URI
func (s *Service) ResolveCluster(uri string) (*clusters.Cluster, error) {
	cluster, err := s.cfg.Storage.GetByResourceURI(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// ClusterLogout logs a user out from the cluster
func (s *Service) ClusterLogout(ctx context.Context, uri string) error {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := cluster.Logout(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CreateGateway creates a gateway to given targetURI
func (s *Service) CreateGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gateway, err := s.createGateway(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return gateway, nil
}

type GatewayCreator interface {
	CreateGateway(context.Context, clusters.CreateGatewayParams) (*gateway.Gateway, error)
}

// createGateway assumes that mu is already held by a public method.
func (s *Service) createGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	cliCommandProvider := clusters.NewDbcmdCLICommandProvider(s.cfg.Storage, dbcmd.SystemExecer{})
	clusterCreateGatewayParams := clusters.CreateGatewayParams{
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetSubresourceName: params.TargetSubresourceName,
		LocalPort:             params.LocalPort,
		CLICommandProvider:    cliCommandProvider,
	}

	gateway, err := s.cfg.GatewayCreator.CreateGateway(ctx, clusterCreateGatewayParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		if err := gateway.Serve(); err != nil {
			gateway.Log().WithError(err).Warn("Failed to open a connection.")
		}
	}()

	s.gateways[gateway.URI().String()] = gateway

	return gateway, nil
}

// RemoveGateway removes cluster gateway
func (s *Service) RemoveGateway(gatewayURI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	gateway, err := s.findGateway(gatewayURI)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.removeGateway(gateway); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// removeGateway assumes that mu is already held by a public method.
func (s *Service) removeGateway(gateway *gateway.Gateway) error {
	// In case closing the gateway fails, we want to return early and leave the gateway in the map.
	//
	// This way the gateway will remain shown as available in the app. The user will see the error
	// related to closing the gateway and will be able to attempt to close it again.
	//
	// This is preferable to removing the gateway from the hash map. Since Connect remembers the port
	// used for a particular db server + username pair, the user would be able to reopen the same
	// gateway on the occupied port only to see an error.
	if err := gateway.Close(); err != nil {
		return trace.Wrap(err)
	}

	delete(s.gateways, gateway.URI().String())

	return nil
}

// RestartGateway stops a gateway and starts a new one with identical parameters.
// It also keeps the original URI so that from the perspective of Connect it's still the same
// gateway but with fresh certs.
func (s *Service) RestartGateway(ctx context.Context, gatewayURI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldGateway, err := s.findGateway(gatewayURI)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.removeGateway(oldGateway); err != nil {
		return trace.Wrap(err)
	}

	newGateway, err := s.createGateway(ctx, CreateGatewayParams{
		TargetURI:             oldGateway.TargetURI(),
		TargetUser:            oldGateway.TargetUser(),
		TargetSubresourceName: oldGateway.TargetSubresourceName(),
		LocalPort:             oldGateway.LocalPort(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// s.createGateway adds a gateway under a random URI, so we need to place the new gateway under
	// the URI of the old gateway.
	delete(s.gateways, newGateway.URI().String())
	newGateway.SetURI(oldGateway.URI())
	s.gateways[oldGateway.URI().String()] = newGateway

	return nil
}

// findGateway assumes that mu is already held by a public method.
func (s *Service) findGateway(gatewayURI string) (*gateway.Gateway, error) {
	if gateway, ok := s.gateways[gatewayURI]; ok {
		return gateway, nil
	}

	return nil, trace.NotFound("gateway is not found: %v", gatewayURI)
}

// ListGateways lists gateways
func (s *Service) ListGateways() []gateway.Gateway {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gws := make([]gateway.Gateway, 0, len(s.gateways))
	for _, gateway := range s.gateways {
		gws = append(gws, *gateway)
	}

	return gws
}

// SetGatewayTargetSubresourceName updates the TargetSubresourceName field of a gateway stored in
// s.gateways.
func (s *Service) SetGatewayTargetSubresourceName(gatewayURI, targetSubresourceName string) (*gateway.Gateway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gateway, err := s.findGateway(gatewayURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway.SetTargetSubresourceName(targetSubresourceName)

	return gateway, nil
}

// ListServers returns cluster servers
func (s *Service) ListServers(ctx context.Context, clusterURI string) ([]clusters.Server, error) {
	cluster, err := s.ResolveCluster(clusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := cluster.GetServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return servers, nil
}

// ListServers returns cluster servers
func (s *Service) ListApps(ctx context.Context, clusterURI string) ([]clusters.App, error) {
	cluster, err := s.ResolveCluster(clusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apps, err := cluster.GetApps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return apps, nil
}

// ListKubes lists kubernetes clusters
func (s *Service) ListKubes(ctx context.Context, uri string) ([]clusters.Kube, error) {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubes, err := cluster.GetKubes(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return kubes, nil
}

// Stop terminates all cluster open connections
func (s *Service) Stop() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, gateway := range s.gateways {
		gateway.Close()
	}
}

// Service is the daemon service
type Service struct {
	cfg *Config
	mu  sync.RWMutex
	// gateways holds the long-running gateways for resources on different clusters. So far it's been
	// used mostly for database gateways but it has potential to be used for app access as well.
	gateways map[string]*gateway.Gateway
}

type CreateGatewayParams struct {
	TargetURI             string
	TargetUser            string
	TargetSubresourceName string
	LocalPort             string
}
