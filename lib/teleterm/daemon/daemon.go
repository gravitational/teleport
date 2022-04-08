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

	apiuri "github.com/gravitational/teleport/lib/teleterm/api/uri"
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
		Config: cfg,
	}, nil
}

// ListRootClusters returns a list of root clusters
func (s *Service) ListRootClusters(ctx context.Context) ([]*clusters.Cluster, error) {
	clusters, err := s.Storage.ReadAll()
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
	cluster, err := s.Storage.Add(ctx, webProxyAddress)
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

	if err := s.Storage.Remove(ctx, cluster.Name); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ResolveCluster resolves a cluster by URI
func (s *Service) ResolveCluster(uri string) (*clusters.Cluster, error) {
	clusterURI, err := apiuri.ParseClusterURI(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := s.Storage.GetByURI(clusterURI.String())
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
func (s *Service) CreateGateway(ctx context.Context, params clusters.CreateGatewayParams) (*gateway.Gateway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, err := s.ResolveCluster(params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway, err := cluster.CreateGateway(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway.Open()

	s.gateways = append(s.gateways, gateway)

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

// RemoveGateway removes cluster gateway
func (s *Service) RemoveGateway(ctx context.Context, gatewayURI string) error {
	gateway, err := s.FindGateway(gatewayURI)
	if err != nil {
		return trace.Wrap(err)
	}

	gateway.Close()

	s.mu.Lock()
	defer s.mu.Unlock()
	// remove closed gateway from list
	for index := range s.gateways {
		if s.gateways[index] == gateway {
			s.gateways = append(s.gateways[:index], s.gateways[index+1:]...)
			return nil
		}
	}

	return nil
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

// FindGateway finds a gateway by URI
func (s *Service) FindGateway(gatewayURI string) (*gateway.Gateway, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, gateway := range s.gateways {
		if gateway.URI.String() == gatewayURI {
			return gateway, nil
		}
	}

	return nil, trace.NotFound("gateway is not found: %v", gatewayURI)
}

// ListGateways lists gateways
func (s *Service) ListGateways(ctx context.Context) ([]*gateway.Gateway, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// copy this slice to avoid race conditions when original slice gets modified
	gateways := make([]*gateway.Gateway, len(s.gateways))
	copy(gateways, s.gateways)
	return gateways, nil
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
	Config

	mu sync.RWMutex
	// gateways is the cluster gateways
	gateways []*gateway.Gateway
}
