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

package daemon

import (
	"context"
	"crypto/tls"
	"os/exec"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/client/proto"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/cmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/teleterm/services/unifiedresources"
	"github.com/gravitational/teleport/lib/teleterm/services/userpreferences"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/daemon"
)

const (
	// tshdEventsTimeout is the maximum amount of time the gRPC client managed by the tshd daemon will
	// wait for a response from the tshd events server managed by the Electron app. This timeout
	// should be used for quick one-off calls where the client doesn't need the server or the user to
	// perform any additional work, such as the SendNotification RPC.
	tshdEventsTimeout = time.Second

	// imporantModalWaitDuraiton is the amount of time to wait between sending tshd events that
	// display important modals in the Electron App. This ensures a clear transition between modals.
	imporantModalWaitDuraiton = time.Second / 2

	// The Electron App can only display one important modal at a time.
	maxConcurrentImportantModals = 1
)

// New creates an instance of Daemon service
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(context.Background())

	connectUsageReporter, err := usagereporter.NewConnectUsageReporter(closeContext, cfg.PrehogAddr)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	go connectUsageReporter.Run(closeContext)

	service := &Service{
		cfg:                    &cfg,
		closeContext:           closeContext,
		cancel:                 cancel,
		gateways:               make(map[string]gateway.Gateway),
		usageReporter:          connectUsageReporter,
		headlessWatcherClosers: make(map[string]context.CancelFunc),
	}

	// TODO(gzdunek): The client cache should be created outside of daemon.New.
	// Unfortunately, we have to do it here, because we need to pass
	// Daemon.ResolveClusterURI as a cluster resolver.
	// Why can't we pass Storage.GetByResourceURI?
	// That's because Daemon.ResolveClusterURI sets a custom MFAPromptConstructor that
	// shows an MFA prompt in Connect.
	// At the level of Storage.ResolveClusterFunc we don't have access to it.
	service.clientCache = cfg.CreateClientCacheFunc(service.ResolveClusterURI)
	return service, nil
}

// relogin makes the Electron app display a login modal to trigger re-login.
func (s *Service) relogin(ctx context.Context, req *api.ReloginRequest) error {
	// Relogin may be triggered by multiple gateways simultaneously. To prevent
	// redundant relogin requests, cut short additional relogin requests.
	if !s.reloginMu.TryLock() {
		return trace.AlreadyExists("another relogin request is in progress")
	}
	defer s.reloginMu.Unlock()

	if err := s.importantModalSemaphore.Acquire(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer s.importantModalSemaphore.Release()

	const reloginUserTimeout = time.Minute
	timeoutCtx, cancelTshdEventsCtx := context.WithTimeout(ctx, reloginUserTimeout)
	defer cancelTshdEventsCtx()

	if _, err := s.tshdEventsClient.Relogin(timeoutCtx, req); err != nil {
		if status.Code(err) == codes.DeadlineExceeded {
			return trace.Wrap(err, "the user did not refresh the session within %s", reloginUserTimeout.String())
		}

		return trace.Wrap(err, "could not refresh the session")
	}

	return nil
}

// retryWithRelogin tries the given function. If the function returns an error that appears to be
// resolvable with relogin, then it requests relogin and tries the function a second time.
//
// retryWithRelogin is reserved for cases where the retryable request does not originate from the
// Electron app, for example when the request is made a long-running goroutine such as a gateway.
// When the request originates from the Electron app and daemon.Service is merely an intermediary,
// the retry flow is handled by clusters.addMetadataToRetryableError and the JavaScript version of
// client.RetryWithRelogin with the same name.
func (s *Service) retryWithRelogin(ctx context.Context, reloginReq *api.ReloginRequest, fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}

	// Do not ask for relogin if the error cannot be resolved with relogin.
	if !client.IsErrorResolvableWithRelogin(err) {
		return trace.Wrap(err)
	}

	err = s.relogin(ctx, reloginReq)
	if err != nil {
		return trace.Wrap(err)
	}

	err = fn()
	return trace.Wrap(err)
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
	cluster, _, err := s.ResolveCluster(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// leaf cluster cannot have own leaves
	if cluster.URI.GetLeafClusterName() != "" {
		return nil, nil
	}

	leaves, err := cluster.GetLeafClusters(ctx, proxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return leaves, nil
}

// AddCluster adds a cluster
func (s *Service) AddCluster(ctx context.Context, webProxyAddress string) (*clusters.Cluster, error) {
	cluster, _, err := s.cfg.Storage.Add(ctx, webProxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// RemoveCluster removes cluster
func (s *Service) RemoveCluster(ctx context.Context, uri string) error {
	cluster, _, err := s.ResolveCluster(uri)
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

// ResolveCluster resolves a cluster by URI by reading data stored on disk in the profile.
//
// It doesn't make network requests so the returned clusters.Cluster will not include full
// information returned from the web/auth servers.
func (s *Service) ResolveCluster(path string) (*clusters.Cluster, *client.TeleportClient, error) {
	resourceURI, err := uri.Parse(path)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cluster, clusterClient, err := s.ResolveClusterURI(resourceURI)
	return cluster, clusterClient, trace.Wrap(err)
}

// ResolveClusterURI is like ResolveCluster, but it accepts an already parsed URI instead of a
// string.
//
// In the future, we should migrate towards ResolveClusterURI. Transforming strings into URIs should
// be done on the outermost layer, that is the gRPC handlers, so that the inner core doesn't have to
// worry about parsing URIs and can assume they are correct.
func (s *Service) ResolveClusterURI(uri uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error) {
	cluster, clusterClient, err := s.cfg.Storage.GetByResourceURI(uri)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Custom MFAPromptConstructor gets removed during the calls to Login and LoginPasswordless RPCs.
	// Those RPCs assume that the default CLI prompt is in use.
	clusterClient.MFAPromptConstructor = s.NewMFAPromptConstructor(cluster.URI.String())
	return cluster, clusterClient, nil
}

// ResolveClusterWithDetails returns fully detailed cluster information. It makes requests to the auth server and includes
// details about the cluster and logged in user.
func (s *Service) ResolveClusterWithDetails(ctx context.Context, uri string) (*clusters.ClusterWithDetails, *client.TeleportClient, error) {
	cluster, clusterClient, err := s.ResolveCluster(uri)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	withDetails, err := cluster.GetWithDetails(ctx, proxyClient.CurrentCluster())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return withDetails, clusterClient, nil
}

// ClusterLogout logs a user out from the cluster
func (s *Service) ClusterLogout(ctx context.Context, uri string) error {
	cluster, _, err := s.ResolveCluster(uri)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := cluster.Logout(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := s.StopHeadlessWatcher(uri); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.ClearCachedClientsForRoot(cluster.URI))
}

// CreateGateway creates a gateway to given targetURI
func (s *Service) CreateGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gateway, err := s.createGateway(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return gateway, nil
}

type GatewayCreator interface {
	CreateGateway(context.Context, clusters.CreateGatewayParams) (gateway.Gateway, error)
}

// createGateway assumes that mu is already held by a public method.
func (s *Service) createGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	targetURI, err := uri.ParseGatewayTargetURI(params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if gateway, ok := s.shouldReuseGateway(targetURI); ok {
		return gateway, nil
	}

	proxyClient, err := s.GetCachedClient(ctx, targetURI.GetClusterURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterCreateGatewayParams := clusters.CreateGatewayParams{
		TargetURI:             targetURI,
		TargetUser:            params.TargetUser,
		TargetSubresourceName: params.TargetSubresourceName,
		LocalPort:             params.LocalPort,
		OnExpiredCert:         s.reissueGatewayCerts,
		KubeconfigsDir:        s.cfg.KubeconfigsDir,
		MFAPromptConstructor:  s.NewMFAPromptConstructor(targetURI.String()),
		ProxyClient:           proxyClient,
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

// reissueGatewayCerts tries to reissue gateway certs. It handles asking the user to relogin and
// per-session MFA checks.
func (s *Service) reissueGatewayCerts(ctx context.Context, g gateway.Gateway) (tls.Certificate, error) {
	reloginReq := &api.ReloginRequest{
		RootClusterUri: g.TargetURI().GetClusterURI().String(),
		Reason: &api.ReloginRequest_GatewayCertExpired{
			GatewayCertExpired: &api.GatewayCertExpired{
				GatewayUri: g.URI().String(),
				TargetUri:  g.TargetURI().String(),
			},
		},
	}

	var cert tls.Certificate

	reissueGatewayCerts := func() error {
		cluster, _, err := s.ResolveClusterURI(g.TargetURI())
		if err != nil {
			return trace.Wrap(err)
		}

		proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
		if err != nil {
			return trace.Wrap(err)
		}

		cert, err = cluster.ReissueGatewayCerts(ctx, proxyClient, g)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// If the gateway certs have expired but the user cert is active,
	// new certs can be obtained without having to relogin first.
	//
	// This can happen if the user cert was refreshed by anything other than the gateway itself. For
	// example, if you execute `tsh ssh` within Connect after your user cert expires or there are two
	// gateways that subsequently go through this flow.
	if err := s.retryWithRelogin(ctx, reloginReq, reissueGatewayCerts); err != nil {
		notifyErr := s.notifyApp(ctx, &api.SendNotificationRequest{
			Subject: &api.SendNotificationRequest_CannotProxyGatewayConnection{
				CannotProxyGatewayConnection: &api.CannotProxyGatewayConnection{
					GatewayUri: g.URI().String(),
					TargetUri:  g.TargetURI().String(),
					Error:      err.Error(),
				},
			},
		})
		if notifyErr != nil {
			s.cfg.Log.WithError(notifyErr).Error("Failed to send a notification for an error encountered during gateway cert reissue")
		}

		// Return the error to the alpn.LocalProxy's middleware.
		return tls.Certificate{}, trace.Wrap(err)
	}

	return cert, nil
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
func (s *Service) removeGateway(gateway gateway.Gateway) error {
	// If gateway.Close() fails it most likely means it was called on a gateway that was already
	// closed and that we have a race condition. Let's return an error in that case.
	if err := gateway.Close(); err != nil {
		return trace.Wrap(err)
	}

	delete(s.gateways, gateway.URI().String())

	return nil
}

// findGateway assumes that mu is already held by a public method.
func (s *Service) findGateway(gatewayURI string) (gateway.Gateway, error) {
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
		gws = append(gws, gateway)
	}

	return gws
}

// GetGatewayCLICommand creates the CLI command used for the provided gateway.
func (s *Service) GetGatewayCLICommand(gateway gateway.Gateway) (cmd.Cmds, error) {
	targetURI := gateway.TargetURI()
	switch {
	case targetURI.IsDB():
		cluster, _, err := s.cfg.Storage.GetByResourceURI(targetURI)
		if err != nil {
			return cmd.Cmds{}, trace.Wrap(err)
		}

		cmds, err := cmd.NewDBCLICommand(cluster, gateway)
		return cmds, trace.Wrap(err)

	case targetURI.IsKube():
		cmds, err := cmd.NewKubeCLICommand(gateway)
		return cmds, trace.Wrap(err)

	case targetURI.IsApp():
		blankCmd := exec.Command("")
		return cmd.Cmds{Exec: blankCmd, Preview: blankCmd}, nil

	default:
		return cmd.Cmds{}, trace.NotImplemented("gateway not supported for %v", targetURI)
	}
}

// SetGatewayTargetSubresourceName updates the TargetSubresourceName field of a gateway stored in
// s.gateways.
func (s *Service) SetGatewayTargetSubresourceName(gatewayURI, targetSubresourceName string) (gateway.Gateway, error) {
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
func (s *Service) SetGatewayLocalPort(gatewayURI, localPort string) (gateway.Gateway, error) {
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

// GetServers accepts parameterized input to enable searching, sorting, and pagination.
func (s *Service) GetServers(ctx context.Context, req *api.GetServersRequest) (*clusters.GetServersResponse, error) {
	cluster, _, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetServers(ctx, req, proxyClient.CurrentCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

func (s *Service) GetRequestableRoles(ctx context.Context, req *api.GetRequestableRolesRequest) (*api.GetRequestableRolesResponse, error) {
	cluster, _, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetRequestableRoles(ctx, req, proxyClient.CurrentCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetRequestableRolesResponse{
		Roles:           response.RequestableRoles,
		ApplicableRoles: response.ApplicableRolesForResources,
	}, nil
}

// PromoteAccessRequest promotes an access request to an access list.
func (s *Service) PromoteAccessRequest(ctx context.Context, rootClusterURI uri.ResourceURI, req *accesslistv1.AccessRequestPromoteRequest) (*clusters.AccessRequest, error) {
	cluster, _, err := s.ResolveClusterURI(rootClusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var response *clusters.AccessRequest
	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		promoteResponse, err := proxyClient.CurrentCluster().AccessListClient().AccessRequestPromote(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}
		accessRequest := promoteResponse.AccessRequest
		response = &clusters.AccessRequest{
			URI:           cluster.URI.AppendAccessRequest(accessRequest.GetName()),
			AccessRequest: accessRequest,
		}
		return nil
	})

	return response, trace.Wrap(err)
}

// GetSuggestedAccessLists returns suggested access lists for an access request.
func (s *Service) GetSuggestedAccessLists(ctx context.Context, rootClusterURI uri.ResourceURI, accessRequestID string) ([]*accesslist.AccessList, error) {
	proxyClient, err := s.GetCachedClient(ctx, rootClusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var response []*accesslist.AccessList
	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		authClient := proxyClient.CurrentCluster()

		accessLists, err := authClient.AccessListClient().GetSuggestedAccessLists(ctx, accessRequestID)
		if err != nil {
			return trace.Wrap(err)
		}
		response = accessLists
		return nil
	})

	return response, trace.Wrap(err)
}

// GetAccessRequests returns all access requests with filtered input
func (s *Service) GetAccessRequests(ctx context.Context, req *api.GetAccessRequestsRequest) ([]clusters.AccessRequest, error) {
	cluster, _, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetAccessRequests(ctx, proxyClient.CurrentCluster(), types.AccessRequestFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// GetAccessRequest returns AccessRequests filtered by ID
func (s *Service) GetAccessRequest(ctx context.Context, req *api.GetAccessRequestRequest) (*clusters.AccessRequest, error) {
	if req.AccessRequestId == "" {
		return nil, trace.BadParameter("missing request id")
	}

	cluster, _, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetAccessRequest(ctx, proxyClient.CurrentCluster(), types.AccessRequestFilter{
		ID: req.AccessRequestId,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// CreateAccessRequest creates an access request
func (s *Service) CreateAccessRequest(ctx context.Context, req *api.CreateAccessRequestRequest) (*clusters.AccessRequest, error) {
	cluster, _, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request, err := cluster.CreateAccessRequest(ctx, proxyClient.CurrentCluster(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return request, nil
}

func (s *Service) ReviewAccessRequest(ctx context.Context, req *api.ReviewAccessRequestRequest) (*clusters.AccessRequest, error) {
	cluster, _, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.ReviewAccessRequest(ctx, proxyClient.CurrentCluster(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

func (s *Service) DeleteAccessRequest(ctx context.Context, req *api.DeleteAccessRequestRequest) error {
	if req.AccessRequestId == "" {
		return trace.BadParameter("missing request id")
	}

	cluster, _, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(cluster.DeleteAccessRequest(ctx, proxyClient.CurrentCluster(), req))
}

func (s *Service) AssumeRole(ctx context.Context, req *api.AssumeRoleRequest) error {
	cluster, _, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := cluster.AssumeRole(ctx, proxyClient, req); err != nil {
		return trace.Wrap(err)
	}

	// We have to reconnect using the updated cert.
	return trace.Wrap(s.ClearCachedClientsForRoot(cluster.URI))
}

// GetKubes accepts parameterized input to enable searching, sorting, and pagination.
func (s *Service) GetKubes(ctx context.Context, req *api.GetKubesRequest) (*clusters.GetKubesResponse, error) {
	cluster, _, err := s.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := cluster.GetKubes(ctx, proxyClient.CurrentCluster(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

func (s *Service) ReportUsageEvent(req *api.ReportUsageEventRequest) error {
	prehogEvent, err := usagereporter.GetAnonymizedPrehogEvent(req)
	if err != nil {
		return trace.Wrap(err)
	}
	s.usageReporter.AddEventsToQueue(prehogEvent)
	return nil
}

// Stop terminates all cluster open connections
func (s *Service) Stop() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.cfg.Log.Info("Stopping")

	for _, gateway := range s.gateways {
		gateway.Close()
	}

	s.StopHeadlessWatchers()

	if err := s.clientCache.Clear(); err != nil {
		s.cfg.Log.WithError(err).Error("Failed to close remote clients")
	}

	timeoutCtx, cancel := context.WithTimeout(s.closeContext, time.Second*10)
	defer cancel()

	if err := s.usageReporter.GracefulStop(timeoutCtx); err != nil {
		s.cfg.Log.WithError(err).Error("Gracefully stopping usage reporter failed")
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
	s.importantModalSemaphore = newWaitSemaphore(maxConcurrentImportantModals, imporantModalWaitDuraiton)

	// Resume headless watchers for any active login sessions.
	if err := s.StartHeadlessWatchers(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// notifyApp sends a notification (usually an error) to the Electron App.
func (s *Service) notifyApp(ctx context.Context, notification *api.SendNotificationRequest) error {
	tshdEventsCtx, cancelTshdEventsCtx := context.WithTimeout(ctx, tshdEventsTimeout)
	defer cancelTshdEventsCtx()

	_, err := s.tshdEventsClient.SendNotification(tshdEventsCtx, notification)
	return trace.Wrap(err)
}

func (s *Service) TransferFile(ctx context.Context, request *api.FileTransferRequest, sendProgress clusters.FileTransferProgressSender) error {
	cluster, _, err := s.ResolveCluster(request.GetServerUri())
	if err != nil {
		return trace.Wrap(err)
	}

	return cluster.TransferFile(ctx, request, sendProgress)
}

// CreateConnectMyComputerRole creates a role which allows access to nodes with the label
// teleport.dev/connect-my-computer/owner: <cluster user> and allows logging in to those nodes as
// the current system user.
func (s *Service) CreateConnectMyComputerRole(ctx context.Context, req *api.CreateConnectMyComputerRoleRequest) (*api.CreateConnectMyComputerRoleResponse, error) {
	cluster, _, err := s.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.CreateConnectMyComputerRoleResponse{}
	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		result, err := s.cfg.ConnectMyComputerRoleSetup.Run(ctx, proxyClient.CurrentCluster(), proxyClient, cluster)
		if err != nil {
			return trace.Wrap(err)
		}
		response.CertsReloaded = result.CertsReloaded
		return nil
	})

	return response, trace.Wrap(err)
}

// CreateConnectMyComputerNodeToken creates a node join token that is valid for 5 minutes.
func (s *Service) CreateConnectMyComputerNodeToken(ctx context.Context, rootClusterUri string) (string, error) {
	cluster, _, err := s.ResolveCluster(rootClusterUri)
	if err != nil {
		return "", trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var nodeToken string
	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		nodeToken, err = s.cfg.ConnectMyComputerTokenProvisioner.CreateNodeToken(ctx, proxyClient.CurrentCluster(), cluster)
		return trace.Wrap(err)
	})

	return nodeToken, trace.Wrap(err)
}

// DeleteConnectMyComputerNode deletes the Connect My Computer node.
func (s *Service) DeleteConnectMyComputerNode(ctx context.Context, req *api.DeleteConnectMyComputerNodeRequest) (*api.DeleteConnectMyComputerNodeResponse, error) {
	cluster, _, err := s.ResolveCluster(req.GetRootClusterUri())
	if err != nil {
		return &api.DeleteConnectMyComputerNodeResponse{}, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		err = s.cfg.ConnectMyComputerNodeDelete.Run(ctx, proxyClient.CurrentCluster(), cluster)
		return trace.Wrap(err)
	})

	return &api.DeleteConnectMyComputerNodeResponse{}, trace.Wrap(err)
}

// GetConnectMyComputerNodeName reads the Connect My Computer node name (UUID) from a disk.
func (s *Service) GetConnectMyComputerNodeName(req *api.GetConnectMyComputerNodeNameRequest) (*api.GetConnectMyComputerNodeNameResponse, error) {
	cluster, _, err := s.ResolveCluster(req.GetRootClusterUri())
	if err != nil {
		return &api.GetConnectMyComputerNodeNameResponse{}, trace.Wrap(err)
	}

	uuid, err := s.cfg.ConnectMyComputerNodeName.Get(cluster)
	return &api.GetConnectMyComputerNodeNameResponse{Name: uuid}, trace.Wrap(err)
}

// WaitForConnectMyComputerNodeJoin returns a response only after detecting that a Connect My
// Computer node for the given cluster has joined the cluster.
func (s *Service) WaitForConnectMyComputerNodeJoin(ctx context.Context, rootClusterURI uri.ResourceURI) (clusters.Server, error) {
	cluster, _, err := s.ResolveClusterURI(rootClusterURI)
	if err != nil {
		return clusters.Server{}, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return clusters.Server{}, trace.Wrap(err)
	}

	var server clusters.Server
	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		server, err = s.cfg.ConnectMyComputerNodeJoinWait.Run(ctx, proxyClient.CurrentCluster(), cluster)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return server, trace.Wrap(err)
}

// ListUnifiedResources returns resources for the given cluster and search params.
func (s *Service) ListUnifiedResources(ctx context.Context, clusterURI uri.ResourceURI, req *proto.ListUnifiedResourcesRequest) (*unifiedresources.ListResponse, error) {
	cluster, _, err := s.ResolveClusterURI(clusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, clusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resources *unifiedresources.ListResponse

	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		resources, err = unifiedresources.List(ctx, cluster, proxyClient.CurrentCluster(), req)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return resources, trace.Wrap(err)
}

// GetUserPreferences returns the preferences for a given user.
func (s *Service) GetUserPreferences(ctx context.Context, clusterURI uri.ResourceURI) (*api.UserPreferences, error) {
	rootProxyClient, err := s.GetCachedClient(ctx, clusterURI.GetRootClusterURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var preferences *api.UserPreferences

	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		rootAuthClient := rootProxyClient.CurrentCluster()

		var leafAuthClient auth.ClientI
		if clusterURI.IsLeaf() {
			leafProxyClient, err := s.GetCachedClient(ctx, clusterURI.GetClusterURI())
			if err != nil {
				return trace.Wrap(err)
			}

			leafAuthClient = leafProxyClient.CurrentCluster()
		}

		preferences, err = userpreferences.Get(ctx, rootAuthClient, leafAuthClient)
		return trace.Wrap(err)
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return preferences, nil
}

// UpdateUserPreferences updates the preferences for a given user.
func (s *Service) UpdateUserPreferences(ctx context.Context, clusterURI uri.ResourceURI, newPreferences *api.UserPreferences) (*api.UserPreferences, error) {
	rootProxyClient, err := s.GetCachedClient(ctx, clusterURI.GetRootClusterURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var preferences *api.UserPreferences

	err = clusters.AddMetadataToRetryableError(ctx, func() error {
		rootAuthClient := rootProxyClient.CurrentCluster()

		var leafAuthClient auth.ClientI
		if clusterURI.IsLeaf() {
			leafProxyClient, err := s.GetCachedClient(ctx, clusterURI.GetClusterURI())
			if err != nil {
				return trace.Wrap(err)
			}

			leafAuthClient = leafProxyClient.CurrentCluster()
		}

		preferences, err = userpreferences.Update(ctx, rootAuthClient, leafAuthClient, newPreferences)
		return trace.Wrap(err)
	})

	return preferences, trace.Wrap(err)
}

func (s *Service) shouldReuseGateway(targetURI uri.ResourceURI) (gateway.Gateway, bool) {
	// A single gateway can be shared for all terminals of the same kube
	// cluster.
	if targetURI.IsKube() {
		return s.findGatewayByTargetURI(targetURI)
	}
	return nil, false
}

func (s *Service) findGatewayByTargetURI(targetURI uri.ResourceURI) (gateway.Gateway, bool) {
	for _, gateway := range s.gateways {
		if gateway.TargetURI() == targetURI {
			return gateway, true
		}
	}
	return nil, false
}

// GetCachedClient returns a client from the cache if it exists,
// otherwise it dials the remote server.
func (s *Service) GetCachedClient(ctx context.Context, clusterURI uri.ResourceURI) (*client.ProxyClient, error) {
	clt, err := s.clientCache.Get(ctx, clusterURI)
	return clt, trace.Wrap(err)
}

// ClearCachedClientsForRoot closes and removes clients from the cache
// for the root cluster and its leaf clusters.
func (s *Service) ClearCachedClientsForRoot(clusterURI uri.ResourceURI) error {
	return trace.Wrap(s.clientCache.ClearForRoot(clusterURI))
}

// Service is the daemon service
type Service struct {
	cfg *Config
	// mu guards gateways and the creation of tshdEventsClient.
	mu sync.RWMutex

	// closeContext is canceled when Service is getting stopped. It is used as a context for the calls
	// to the tshd events gRPC client.
	closeContext context.Context
	cancel       context.CancelFunc
	// gateways holds the long-running gateways for resources on different clusters. So far it's been
	// used mostly for database gateways but it has potential to be used for app access as well.
	gateways map[string]gateway.Gateway
	// tshdEventsClient is a client to send events to the Electron App.
	tshdEventsClient api.TshdEventsServiceClient
	// The Electron App can only display one important Modal at a time. tshd events
	// that trigger an important modal (relogin, headless login) should use this
	// lock to ensure it doesn't overwrite existing tshd-initiated important modals.
	//
	// We use a semaphore instead of a mutex in order to cancel important modals that
	// are no longer relevant before acquisition.
	//
	// We use a waitSemaphore in order to make sure there is a clear transition between modals.
	importantModalSemaphore *waitSemaphore
	// usageReporter batches the events and sends them to prehog
	usageReporter *usagereporter.UsageReporter
	// reloginMu is used when a goroutine needs to request a relogin from the Electron app. Since the
	// app can show only one login modal at a time, we need to submit only one request at a time.
	reloginMu sync.Mutex
	// headlessWatcherClosers holds a map of root cluster URIs to headless watchers.
	headlessWatcherClosers   map[string]context.CancelFunc
	headlessWatcherClosersMu sync.Mutex
	clientCache              ClientCache
}

type CreateGatewayParams struct {
	TargetURI             string
	TargetUser            string
	TargetSubresourceName string
	LocalPort             string
}

// waitSemaphore is a semaphore that waits for a specified duration between acquisitions.
type waitSemaphore struct {
	semC         chan struct{}
	lastRelease  time.Time
	waitDuration time.Duration
}

func newWaitSemaphore(maxConcurrency int, waitDuration time.Duration) *waitSemaphore {
	return &waitSemaphore{
		semC:         make(chan struct{}, maxConcurrency),
		waitDuration: waitDuration,
	}
}

func (s *waitSemaphore) Acquire(ctx context.Context) error {
	select {
	case s.semC <- struct{}{}:
		// wait up to the specified wait duration before returning.
		time.Sleep(s.waitDuration - time.Since(s.lastRelease))
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func (s *waitSemaphore) Release() {
	s.lastRelease = time.Now()
	<-s.semC
}
