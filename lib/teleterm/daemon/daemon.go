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

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

// New creates an instance of Daemon service
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(context.Background())

	return &Service{
		cfg:          &cfg,
		closeContext: closeContext,
		cancel:       cancel,
		gateways:     make(map[string]*gateway.Gateway),
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

	if err := s.cfg.Storage.Remove(ctx, cluster.ProfileName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ResolveCluster resolves a cluster by URI and returns
// information stored in the profile along with a TeleportClient.
// It will not include detailed information returned from the web/auth servers
func (s *Service) ResolveCluster(uri string) (*clusters.Cluster, error) {
	cluster, err := s.cfg.Storage.GetByResourceURI(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// GetCluster returns cluster information
func (s *Service) GetCluster(ctx context.Context, uri string) (*clusters.Cluster, error) {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	features, err := cluster.GetClusterFeatures(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster.Features = features

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
		TCPPortAllocator:      s.cfg.TCPPortAllocator,
	}

	gateway, err := s.cfg.GatewayCreator.CreateGateway(ctx, clusterCreateGatewayParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		if err := gateway.Serve(); err != nil {
			gateway.Log().WithError(err).Warn("Failed to handle a gateway connection.")
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
	// If gateway.Close() fails it most likely means it was called on a gateway that was already
	// closed and that we have a race condition. Let's return an error in that case.
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

// SetGatewayLocalPort creates a new gateway with the given port, swaps it with the old gateway
// under the same URI in s.gateways and then closes the old gateway. It doesn't fetch a fresh db
// cert.
//
// If gateway.NewWithLocalPort fails it's imperative that the current gateway is kept intact. This
// way if the user attempts to change the port to one that cannot be obtained, they're able to
// correct that mistake and choose a different port.
//
// SetGatewayLocalPort is a noop if port is equal to the existing port.
func (s *Service) SetGatewayLocalPort(gatewayURI, localPort string) (*gateway.Gateway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldGateway, err := s.findGateway(gatewayURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if localPort == oldGateway.LocalPort() {
		return oldGateway, nil
	}

	newGateway, err := gateway.NewWithLocalPort(oldGateway, localPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.removeGateway(oldGateway); err != nil {
		// s.removeGateway() fails only if it was called on a gateway that was already close. This
		// shouldn't happen and would mean that we have a race condition.
		//
		// Rather than continuing in presence of the race condition, let's attempt to close the new
		// gateway (since it shouldn't be used anyway) and return the error.
		if newGatewayCloseErr := newGateway.Close(); newGatewayCloseErr != nil {
			newGateway.Log().Warnf(
				"Failed to close the new gateway after failing to close the old gateway: %v",
				newGatewayCloseErr,
			)
		}
		return nil, trace.Wrap(err)
	}

	s.gateways[gatewayURI] = newGateway

	go func() {
		if err := newGateway.Serve(); err != nil {
			newGateway.Log().WithError(err).Warn("Failed to handle a gateway connection.")
		}
	}()

	return newGateway, nil
}

// GetAllServers returns a full list of nodes without pagination or sorting.
func (s *Service) GetAllServers(ctx context.Context, clusterURI string) ([]clusters.Server, error) {
	cluster, err := s.ResolveCluster(clusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := cluster.GetAllServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return servers, nil
}

// GetServers accepts parameterized input to enable searching, sorting, and pagination.
func (s *Service) GetServers(ctx context.Context, req *api.GetServersRequest) (*clusters.GetServersResponse, error) {
	cluster, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetServers(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

func (s *Service) GetRequestableRoles(ctx context.Context, req *api.GetRequestableRolesRequest) (*api.GetRequestableRolesResponse, error) {
	cluster, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetRequestableRoles(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetRequestableRolesResponse{
		Roles:           response.RequestableRoles,
		ApplicableRoles: response.ApplicableRolesForResources,
	}, nil
}

// GetAccessRequests returns all access requests with filtered input
func (s *Service) GetAccessRequests(ctx context.Context, req *api.GetAccessRequestsRequest) ([]clusters.AccessRequest, error) {
	cluster, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := cluster.GetAccessRequests(ctx, types.AccessRequestFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// GetAccessRequest returns AccessRequests filtered by ID
func (s *Service) GetAccessRequest(ctx context.Context, req *api.GetAccessRequestRequest) ([]clusters.AccessRequest, error) {
	if req.AccessRequestId == "" {
		return nil, trace.BadParameter("missing request id")
	}

	cluster, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetAccessRequests(ctx, types.AccessRequestFilter{
		ID: req.AccessRequestId,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// CreateAccessRequest creates an access request
func (s *Service) CreateAccessRequest(ctx context.Context, req *api.CreateAccessRequestRequest) (*clusters.AccessRequest, error) {
	cluster, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request, err := cluster.CreateAccessRequest(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return request, nil
}

func (s *Service) ReviewAccessRequest(ctx context.Context, req *api.ReviewAccessRequestRequest) (*clusters.AccessRequest, error) {
	cluster, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := cluster.ReviewAccessRequest(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

func (s *Service) DeleteAccessRequest(ctx context.Context, req *api.DeleteAccessRequestRequest) error {
	if req.AccessRequestId == "" {
		return trace.BadParameter("missing request id")
	}

	cluster, err := s.ResolveCluster((req.RootClusterUri))
	if err != nil {
		return trace.Wrap(err)
	}

	err = cluster.DeleteAccessRequest(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) AssumeRole(ctx context.Context, req *api.AssumeRoleRequest) error {
	cluster, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return trace.Wrap(err)
	}

	err = cluster.AssumeRole(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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

// GetAllKubes lists kubernetes clusters
func (s *Service) GetAllKubes(ctx context.Context, uri string) ([]clusters.Kube, error) {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubes, err := cluster.GetAllKubes(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return kubes, nil
}

// GetKubes accepts parameterized input to enable searching, sorting, and pagination.
func (s *Service) GetKubes(ctx context.Context, req *api.GetKubesRequest) (*clusters.GetKubesResponse, error) {
	cluster, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetKubes(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// Stop terminates all cluster open connections
func (s *Service) Stop() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.cfg.Log.Info("Stopping")

	for _, gateway := range s.gateways {
		gateway.Close()
	}

	// s.closeContext is used for the tshd events client which might make requests as long as any of
	// the resources managed by daemon.Service are up and running. So let's cancel the context only
	// after closing those resources.
	s.cancel()
}

// UpdateAndDialTshdEventsServerAddress allows the Electron app to provide the tshd events server
// address.
//
// The startup of the app is orchestrated so that this method is called before any other method on
// daemon.Service. This way all the other code in daemon.Service can assume that the tshd events
// client is available right from the beginning, without the need for nil checks.
func (s *Service) UpdateAndDialTshdEventsServerAddress(serverAddress string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	withCreds, err := s.cfg.CreateTshdEventsClientCredsFunc()
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := grpc.Dial(serverAddress, withCreds)
	if err != nil {
		return trace.Wrap(err)
	}

	client := api.NewTshdEventsServiceClient(conn)
	s.tshdEventsClient = client

	return nil
}

func (s *Service) TransferFile(ctx context.Context, request *api.FileTransferRequest, sendProgress clusters.FileTransferProgressSender) error {
	cluster, err := s.ResolveCluster(request.GetClusterUri())
	if err != nil {
		return trace.Wrap(err)
	}

	return cluster.TransferFile(ctx, request, sendProgress)
}

// Service is the daemon service
type Service struct {
	cfg *Config
	mu  sync.RWMutex
	// closeContext is canceled when Service is getting stopped. It is used as a context for the calls
	// to the tshd events gRPC client.
	closeContext context.Context
	cancel       context.CancelFunc
	// gateways holds the long-running gateways for resources on different clusters. So far it's been
	// used mostly for database gateways but it has potential to be used for app access as well.
	gateways map[string]*gateway.Gateway
	// tshdEventsClient is created after UpdateAndDialTshdEventsServerAddress gets called. The startup
	// of the whole app is orchestrated in a way which ensures that is the first Service method that
	// gets called. This lets other methods in Service assume that tshdEventsClient is available from
	// the start, without having to perform nil checks.
	tshdEventsClient api.TshdEventsServiceClient
}

type CreateGatewayParams struct {
	TargetURI             string
	TargetUser            string
	TargetSubresourceName string
	LocalPort             string
}
