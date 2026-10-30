/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package presencev1

import (
	"context"
	"iter"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	presencepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type cache interface {
	GetApplication(ctx context.Context, req *presencepb.GetApplicationRequest) (types.Application, error)
	GetKubeCluster(ctx context.Context, req *presencepb.GetKubeClusterRequest) (types.KubeCluster, error)
	GetRelayServer(ctx context.Context, name string) (*presencepb.RelayServer, error)
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
	GetSSHServer(ctx context.Context, req *presencepb.GetSSHServerRequest) (types.Server, error)
	ListAuthServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	ListRelayServers(ctx context.Context, pageSize int, pageToken string) (_ []*presencepb.RelayServer, nextPageToken string, _ error)
	ListRemoteClusters(ctx context.Context, pageSize int, nextToken string) ([]types.RemoteCluster, string, error)
	ListReverseTunnels(ctx context.Context, pageSize int, nextToken string) ([]types.ReverseTunnel, string, error)
	RangeApplications(ctx context.Context, req *presencepb.ListApplicationsRequest) iter.Seq2[types.Application, error]
	RangeKubeClusters(ctx context.Context, req *presencepb.ListKubeClustersRequest) iter.Seq2[types.KubeCluster, error]
	RangeSSHServers(ctx context.Context, req *presencepb.ListSSHServersRequest) iter.Seq2[types.Server, error]
}

// ensure that the auth cache implements the read-only methods we want; it's
// still possible for auth.Server to override one of these, but in such a case
// we should probably use the overridden method
var _ cache = struct{ authclient.Cache }{}

type AuthServer interface {
	cache

	DeleteApplication(ctx context.Context, req *presencepb.DeleteApplicationRequest) error
	DeleteAppServer(ctx context.Context, req *presencepb.DeleteAppServerRequest) error
	DeleteKubeCluster(ctx context.Context, req *presencepb.DeleteKubeClusterRequest) error
	DeleteKubeServer(ctx context.Context, req *presencepb.DeleteKubeServerRequest) error
	DeleteRelayServer(ctx context.Context, name string) error
	DeleteRemoteCluster(ctx context.Context, clusterName string) error
	DeleteReverseTunnel(ctx context.Context, tunnelName string) error
	DeleteSSHServer(ctx context.Context, req *presencepb.DeleteSSHServerRequest) error
	PatchRemoteCluster(ctx context.Context, name string, updateFn func(rc types.RemoteCluster) (types.RemoteCluster, error)) (types.RemoteCluster, error)
	UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error)
	UpsertReverseTunnel(ctx context.Context, tunnel types.ReverseTunnel) (types.ReverseTunnel, error)
}

// ServiceConfig holds configuration options for
// the presence gRPC service.
type ServiceConfig struct {
	Authorizer       authz.Authorizer
	ScopedAuthorizer authz.ScopedAuthorizer
	AuthServer       AuthServer
	Logger           *slog.Logger
	Emitter          apievents.Emitter
	Reporter         usagereporter.UsageReporter
	Clock            clockwork.Clock
}

// Service implements the teleport.presence.v1.PresenceService RPC service.
type Service struct {
	presencepb.UnimplementedPresenceServiceServer

	authorizer       authz.Authorizer
	scopedAuthorizer authz.ScopedAuthorizer
	authServer       AuthServer
	logger           *slog.Logger
	emitter          apievents.Emitter
	reporter         usagereporter.UsageReporter
	clock            clockwork.Clock
}

var _ presencepb.PresenceServiceServer = (*Service)(nil)

// NewService returns a new presence gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("auth server is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "presence.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Service{
		logger:           cfg.Logger,
		authorizer:       cfg.Authorizer,
		scopedAuthorizer: cfg.ScopedAuthorizer,
		authServer:       cfg.AuthServer,

		emitter:  cfg.Emitter,
		reporter: cfg.Reporter,
		clock:    cfg.Clock,
	}, nil
}

// GetRemoteCluster returns a remote cluster by name.
func (s *Service) GetRemoteCluster(
	ctx context.Context, req *presencepb.GetRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := s.authServer.GetRemoteCluster(ctx, req.GetName())
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	if err := authCtx.Checker.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	v3, ok := rc.(*types.RemoteClusterV3)
	if !ok {
		s.logger.WarnContext(ctx, "unexpected remote cluster type",
			"got_type", logutils.TypeAttr(rc),
			"expected_type", "RemoteClusterV3",
			"remote_cluster", rc.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected remote cluster type")
	}

	return v3, nil
}

// ListRemoteClusters returns a list of remote clusters.
func (s *Service) ListRemoteClusters(
	ctx context.Context, req *presencepb.ListRemoteClustersRequest,
) (*presencepb.ListRemoteClustersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	page, nextToken, err := s.authServer.ListRemoteClusters(
		ctx, int(req.GetPageSize()), req.GetPageToken(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert the remote clusters to the V3 type
	concretePage := make([]*types.RemoteClusterV3, 0, len(page))
	for _, rc := range page {
		v3, ok := rc.(*types.RemoteClusterV3)
		if !ok {
			s.logger.WarnContext(ctx, "unexpected remote cluster type",
				"got_type", logutils.TypeAttr(rc),
				"expected_type", "RemoteClusterV3",
				"remote_cluster", rc.GetName(),
			)
			continue
		}
		concretePage = append(concretePage, v3)
	}

	// Filter out remote clusters that the user doesn't have access to.
	filteredPage := make([]*types.RemoteClusterV3, 0, len(concretePage))
	for _, rc := range concretePage {
		if err := authCtx.Checker.CheckAccessToRemoteCluster(rc); err != nil {
			if trace.IsAccessDenied(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		filteredPage = append(filteredPage, rc)
	}

	return &presencepb.ListRemoteClustersResponse{
		RemoteClusters: filteredPage,
		NextPageToken:  nextToken,
	}, nil
}

// UpdateRemoteCluster updates a remote cluster.
func (s *Service) UpdateRemoteCluster(
	ctx context.Context, req *presencepb.UpdateRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	switch {
	case req.RemoteCluster == nil:
		return nil, trace.BadParameter("remote_cluster: must not be nil")
	case req.RemoteCluster.GetName() == "":
		return nil, trace.BadParameter("remote_cluster.Metadata.Name: must be non-empty")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	// If the update mask is empty, update the entire remote cluster.
	if len(req.GetUpdateMask().GetPaths()) == 0 {
		rc, err := s.authServer.UpdateRemoteCluster(ctx, req.GetRemoteCluster())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		v3, ok := rc.(*types.RemoteClusterV3)
		if !ok {
			s.logger.WarnContext(ctx, "unexpected remote cluster type",
				"got_type", logutils.TypeAttr(rc),
				"expected_type", "RemoteClusterV3",
				"remote_cluster", rc.GetName(),
			)
			return nil, trace.BadParameter("encountered unexpected remote cluster type")
		}
		return v3, nil
	}

	// Otherwise, we apply the update mask to the current remote cluster using
	// a patch operation.
	req.GetUpdateMask().Normalize()
	rc, err := s.authServer.PatchRemoteCluster(ctx, req.GetRemoteCluster().GetName(), func(rc types.RemoteCluster) (types.RemoteCluster, error) {
		for _, path := range req.GetUpdateMask().GetPaths() {
			switch path {
			case "Metadata.Labels":
				md := rc.GetMetadata()
				md.Labels = req.RemoteCluster.GetMetadata().Labels
				rc.SetMetadata(md)
			case "Metadata.Description":
				md := rc.GetMetadata()
				md.Description = req.RemoteCluster.GetMetadata().Description
				rc.SetMetadata(md)
			case "Metadata.Expires":
				rc.SetExpiry(req.RemoteCluster.Expiry())
			case "Status.Connection":
				rc.SetConnectionStatus(req.RemoteCluster.GetConnectionStatus())
			case "Status.LastHeartbeat":
				rc.SetLastHeartbeat(req.RemoteCluster.GetLastHeartbeat())
			default:
				return nil, trace.BadParameter("unsupported field: %q", path)
			}
		}
		return rc, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v3, ok := rc.(*types.RemoteClusterV3)
	if !ok {
		s.logger.WarnContext(ctx, "unexpected remote cluster type",
			"got_type", logutils.TypeAttr(rc),
			"expected_type", "RemoteClusterV3",
			"remote_cluster", rc.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected remote cluster type")
	}

	return v3, nil
}

// DeleteRemoteCluster deletes a remote cluster.
func (s *Service) DeleteRemoteCluster(
	ctx context.Context, req *presencepb.DeleteRemoteClusterRequest,
) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(
		types.KindRemoteCluster, types.VerbDelete,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteRemoteCluster(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteApplicationServer deletes a scoped or unscoped application server.
func (s *Service) DeleteAppServer(
	ctx context.Context, req *presencepb.DeleteAppServerRequest,
) (*presencepb.DeleteAppServerResponse, error) {
	authzCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case req.GetHostId() == "":
		return nil, trace.BadParameter("host_id: must be specified")
	case req.GetName() == "":
		return nil, trace.BadParameter("name: must be specified")
	}

	// App agents can delete their own app servers
	// (builtin RoleApp only grants read on app_server rules); anyone else
	// needs an app_server delete rule granted in the target scope.
	ruleCtx := authzCtx.RuleContext()
	if _, err := authzCtx.CheckerContext.Decision(ctx, req.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := authzCtx.AgentOwnedResourceAction(req.GetScope(), req.GetHostId(), types.RoleApp); err != nil {
			return checker.CheckAccessToRules(&ruleCtx, types.KindAppServer, scopedaccess.Delete)
		}
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteAppServer(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return presencepb.DeleteAppServerResponse_builder{}.Build(), nil
}

// ListAuthServers returns a page of auth servers.
func (s *Service) ListAuthServers(
	ctx context.Context, req *presencepb.ListAuthServersRequest,
) (*presencepb.ListAuthServersResponse, error) {
	authzCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ruleCtx := authzCtx.RuleContext()
	if err := authzCtx.CheckerContext.RiskyAuthorizeUnpinnedRead(ctx, services.UnpinnedReadAuthServers, &ruleCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	servers, nextToken, err := s.authServer.ListAuthServers(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverV2s := make([]*types.ServerV2, 0, len(servers))
	for _, server := range servers {
		v2, ok := server.(*types.ServerV2)
		if !ok {
			s.logger.WarnContext(ctx, "unexpected server type",
				"got_type", logutils.TypeAttr(server),
				"expected_type", "ServerV2",
				"server", server.GetName(),
			)
			continue
		}
		serverV2s = append(serverV2s, v2)
	}

	return presencepb.ListAuthServersResponse_builder{
		Servers:       serverV2s,
		NextPageToken: nextToken,
	}.Build(), nil
}

// ListProxyServers returns a page of proxy servers.
func (s *Service) ListProxyServers(
	ctx context.Context, req *presencepb.ListProxyServersRequest,
) (*presencepb.ListProxyServersResponse, error) {
	authzCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ruleCtx := authzCtx.RuleContext()
	if err := authzCtx.CheckerContext.RiskyAuthorizeUnpinnedRead(ctx, services.UnpinnedReadProxies, &ruleCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	servers, nextToken, err := s.authServer.ListProxyServers(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverV2s := make([]*types.ServerV2, 0, len(servers))
	for _, server := range servers {
		v2, ok := server.(*types.ServerV2)
		if !ok {
			s.logger.WarnContext(ctx, "unexpected server type",
				"got_type", logutils.TypeAttr(server),
				"expected_type", "ServerV2",
				"server", server.GetName(),
			)
			continue
		}
		serverV2s = append(serverV2s, v2)
	}

	return presencepb.ListProxyServersResponse_builder{
		Servers:       serverV2s,
		NextPageToken: nextToken,
	}.Build(), nil
}

// ListReverseTunnels returns a page of reverse tunnels.
func (s *Service) ListReverseTunnels(
	ctx context.Context, req *presencepb.ListReverseTunnelsRequest,
) (*presencepb.ListReverseTunnelsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindReverseTunnel, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	page, nextToken, err := s.authServer.ListReverseTunnels(
		ctx, int(req.GetPageSize()), req.GetPageToken(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert the reverse tunnels to the concrete type
	concretePage := make([]*types.ReverseTunnelV2, 0, len(page))
	for _, rc := range page {
		v3, ok := rc.(*types.ReverseTunnelV2)
		if !ok {
			s.logger.WarnContext(ctx, "unexpected reverse tunnel type",
				"got_type", logutils.TypeAttr(rc),
				"expected_type", "ReverseTunnelV2",
				"reverse_tunnel", rc.GetName(),
			)
			continue
		}
		concretePage = append(concretePage, v3)
	}

	return &presencepb.ListReverseTunnelsResponse{
		ReverseTunnels: concretePage,
		NextPageToken:  nextToken,
	}, nil
}

// UpsertReverseTunnel upserts a reverse tunnel.
func (s *Service) UpsertReverseTunnel(
	ctx context.Context, req *presencepb.UpsertReverseTunnelRequest,
) (*types.ReverseTunnelV2, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindReverseTunnel, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.ReverseTunnel == nil {
		return nil, trace.BadParameter("reverse_tunnel: must not be nil")
	}

	if err := services.ValidateReverseTunnel(req.ReverseTunnel); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := s.authServer.UpsertReverseTunnel(ctx, req.GetReverseTunnel())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	concrete, ok := res.(*types.ReverseTunnelV2)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected reverse tunnel type %T", res)
	}

	return concrete, nil
}

// DeleteReverseTunnel deletes a reverse tunnel.
func (s *Service) DeleteReverseTunnel(
	ctx context.Context, req *presencepb.DeleteReverseTunnelRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindReverseTunnel, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	return nil, trace.Wrap(s.authServer.DeleteReverseTunnel(ctx, req.GetName()))
}

// GetRelayServer implements [presencepb.PresenceServiceServer].
func (s *Service) GetRelayServer(ctx context.Context, req *presencepb.GetRelayServerRequest) (*presencepb.GetRelayServerResponse, error) {
	actx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := actx.CheckAccessToKind(types.KindRelayServer, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	relayServer, err := s.authServer.GetRelayServer(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &presencepb.GetRelayServerResponse{
		RelayServer: relayServer,
	}, nil
}

// ListRelayServers implements [presencepb.PresenceServiceServer].
func (s *Service) ListRelayServers(ctx context.Context, req *presencepb.ListRelayServersRequest) (*presencepb.ListRelayServersResponse, error) {
	actx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := actx.CheckAccessToKind(types.KindRelayServer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	relayServers, nextPageToken, err := s.authServer.ListRelayServers(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &presencepb.ListRelayServersResponse{
		Relays:        relayServers,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteRelayServer implements [presencepb.PresenceServiceServer].
func (s *Service) DeleteRelayServer(ctx context.Context, req *presencepb.DeleteRelayServerRequest) (*presencepb.DeleteRelayServerResponse, error) {
	actx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := actx.CheckAccessToKind(types.KindRelayServer, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteRelayServer(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &presencepb.DeleteRelayServerResponse{}, nil
}

// GetKubeCluster returns the specified kube cluster resource.
func (s *Service) GetKubeCluster(ctx context.Context, req *presencepb.GetKubeClusterRequest) (*presencepb.GetKubeClusterResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verb := scopedaccess.Read
	if req.GetWithSecrets() {
		verb = scopedaccess.Secrets
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindKubernetesCluster, verb); err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := s.authServer.GetKubeCluster(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := authContext.CheckerContext.Decision(ctx, cluster.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindKubernetesCluster, verb); err != nil {
			return err
		}
		return checker.Kube().CanAccessCluster(cluster)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterV3, ok := cluster.(*types.KubernetesClusterV3)
	if !ok {
		return nil, trace.BadParameter("invalid cluster")
	}
	return presencepb.GetKubeClusterResponse_builder{
		Cluster: clusterV3,
	}.Build(), nil
}

// getCursorForKubeCluster wraps [services.GetCursorForKubeCluster] with a signature
// referencing [*types.KubernetesClusterV3] directly. This helps go infer the proper
// typing when using [generic.CollectPageAndCursor].
func getCursorForKubeCluster(cluster *types.KubernetesClusterV3) string {
	return services.GetCursorForKubeCluster(cluster)
}

// ListKubeClusters returns a page of registered kube clusters.
func (s *Service) ListKubeClusters(ctx context.Context, req *presencepb.ListKubeClustersRequest) (*presencepb.ListKubeClustersResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verb := scopedaccess.Read
	if req.GetWithSecrets() {
		verb = scopedaccess.Secrets
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindKubernetesCluster, verb, scopedaccess.List); err != nil {
		return nil, trace.Wrap(err)
	}

	// list method scope filters must use identity-based defaults per RFD 0229i
	req.SetScopeFilter(authContext.CheckerContext.ResolveScopeFilter(req.GetScopeFilter()))

	clusters, nextToken, err := generic.CollectPageAndCursor(
		stream.FilterMap(
			s.authServer.RangeKubeClusters(ctx, req),
			func(cluster types.KubeCluster) (*types.KubernetesClusterV3, bool) {
				// Filter out kube clusters user doesn't have access to.
				if _, err := authContext.CheckerContext.Decision(ctx, cluster.GetScope(), func(checker *services.ScopedAccessChecker) error {
					if err := checker.CheckAccessToRules(&ruleCtx, types.KindKubernetesCluster, verb, scopedaccess.List); err != nil {
						return err
					}
					return checker.Kube().CanAccessCluster(cluster)
				}); err == nil {
					clusterV3, ok := cluster.(*types.KubernetesClusterV3)
					return clusterV3, ok
				}
				return nil, false
			},
		),
		int(req.GetPageSize()),
		getCursorForKubeCluster,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return presencepb.ListKubeClustersResponse_builder{
		Clusters:      clusters,
		NextPageToken: nextToken,
	}.Build(), nil
}

// DeleteKubeCluster removes the specified kube cluster resource.
func (s *Service) DeleteKubeCluster(ctx context.Context, req *presencepb.DeleteKubeClusterRequest) (*presencepb.DeleteKubeClusterResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindKubernetesCluster, scopedaccess.Delete); err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure user has access to the kubernetes cluster before deleting.
	cluster, err := s.authServer.GetKubeCluster(ctx, presencepb.GetKubeClusterRequest_builder{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := authContext.CheckerContext.Decision(ctx, cluster.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindKubernetesCluster, scopedaccess.Delete); err != nil {
			return err
		}
		return checker.Kube().CanAccessCluster(cluster)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteKubeCluster(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.KubernetesClusterDelete{
		Metadata: apievents.Metadata{
			Type: events.KubernetesClusterDeleteEvent,
			Code: events.KubernetesClusterDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:  req.GetName(),
			Scope: req.GetScope(),
		},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit kube cluster delete event", "error", err)
	}
	return presencepb.DeleteKubeClusterResponse_builder{}.Build(), nil
}

// DeleteKubeServer deletes a scoped or unscoped kube server.
func (s *Service) DeleteKubeServer(
	ctx context.Context, req *presencepb.DeleteKubeServerRequest,
) (*presencepb.DeleteKubeServerResponse, error) {
	authzCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case req.GetHostId() == "":
		return nil, trace.BadParameter("host_id: must be specified")
	case req.GetName() == "":
		return nil, trace.BadParameter("name: must be specified")
	}

	ruleCtx := authzCtx.RuleContext()
	if _, err := authzCtx.CheckerContext.Decision(ctx, req.GetScope(), func(checker *services.ScopedAccessChecker) error {
		// Kube agents can delete their own kube servers
		// (builtin RoleKube only grants read on kube_server rules); anyone else
		// needs an app_server delete rule granted in the target scope.
		if err := authzCtx.AgentOwnedResourceAction(req.GetScope(), req.GetHostId(), types.RoleKube); err == nil {
			return nil
		}
		return checker.CheckAccessToRules(&ruleCtx, types.KindKubeServer, scopedaccess.Delete)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteKubeServer(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return presencepb.DeleteKubeServerResponse_builder{}.Build(), nil
}

// GetSSHServer returns the specified node resource.
func (s *Service) GetSSHServer(ctx context.Context, req *presencepb.GetSSHServerRequest) (*presencepb.GetSSHServerResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindNode, scopedaccess.Read); err != nil {
		return nil, trace.Wrap(err)
	}

	node, err := s.authServer.GetSSHServer(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := authContext.CheckerContext.Decision(ctx, node.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindNode, scopedaccess.Read); err != nil {
			return err
		}
		return checker.SSH().CanAccessSSHServer(node)
	}); err != nil {
		if trace.IsAccessDenied(err) {
			return nil, trace.NotFound("not found")
		}
		return nil, trace.Wrap(err)
	}

	nodeV2, ok := node.(*types.ServerV2)
	if !ok {
		return nil, trace.BadParameter("invalid node")
	}
	return presencepb.GetSSHServerResponse_builder{
		Server: nodeV2,
	}.Build(), nil
}

// getCursorForNode wraps [services.GetCursorForNode] with a signature
// referencing [*types.ServerV2] directly. This helps go infer the proper
// typing when using [generic.CollectPageAndCursor].
func getCursorForNode(node *types.ServerV2) string {
	return services.GetCursorForNode(node)
}

// ListSSHServers returns a page of registered nodes.
func (s *Service) ListSSHServers(ctx context.Context, req *presencepb.ListSSHServersRequest) (*presencepb.ListSSHServersResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindNode, scopedaccess.List); err != nil {
		return nil, trace.Wrap(err)
	}

	// list method scope filters must use identity-based defaults per RFD 0229i
	req.SetScopeFilter(authContext.CheckerContext.ResolveScopeFilter(req.GetScopeFilter()))

	servers, nextToken, err := generic.CollectPageAndCursor(
		stream.FilterMap(
			s.authServer.RangeSSHServers(ctx, req),
			func(node types.Server) (*types.ServerV2, bool) {
				// Filter out nodes the user doesn't have access to.
				if _, err := authContext.CheckerContext.Decision(ctx, node.GetScope(), func(checker *services.ScopedAccessChecker) error {
					if err := checker.CheckAccessToRules(&ruleCtx, types.KindNode, scopedaccess.List); err != nil {
						return err
					}
					return checker.SSH().CanAccessSSHServer(node)
				}); err == nil {
					nodeV2, ok := node.(*types.ServerV2)
					return nodeV2, ok
				}
				return nil, false
			},
		),
		int(req.GetPageSize()),
		getCursorForNode,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return presencepb.ListSSHServersResponse_builder{
		Servers:       servers,
		NextPageToken: nextToken,
	}.Build(), nil
}

// DeleteSSHServer removes the specified node resource.
func (s *Service) DeleteSSHServer(ctx context.Context, req *presencepb.DeleteSSHServerRequest) (*presencepb.DeleteSSHServerResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindNode, scopedaccess.Delete); err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure the user has access to the node before deleting it.
	node, err := s.authServer.GetSSHServer(ctx, presencepb.GetSSHServerRequest_builder{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := authContext.CheckerContext.Decision(ctx, node.GetScope(), func(checker *services.ScopedAccessChecker) error {
		return checker.CheckAccessToRules(&ruleCtx, types.KindNode, scopedaccess.Delete)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteSSHServer(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return presencepb.DeleteSSHServerResponse_builder{}.Build(), nil
}

// GetApplication returns the specified application resource.
func (s *Service) GetApplication(ctx context.Context, req *presencepb.GetApplicationRequest) (*presencepb.GetApplicationResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindApp, scopedaccess.Read); err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := s.authServer.GetApplication(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := authContext.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindApp, scopedaccess.Read); err != nil {
			return err
		}
		return checker.App().CanAccessApp(app)
	}); err != nil {
		if trace.IsAccessDenied(err) {
			return nil, trace.NotFound("not found")
		}
		return nil, trace.Wrap(err)
	}

	appV3, ok := app.(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("invalid application")
	}
	return presencepb.GetApplicationResponse_builder{
		Application: appV3,
	}.Build(), nil
}

// getCursorForApp wraps [services.GetCursorForApp] with a signature referencing
// [*types.AppV3] directly, to help go infer the typing for
// [generic.CollectPageAndCursor].
func getCursorForApp(app *types.AppV3) string {
	return services.GetCursorForApp(app)
}

// ListApplications returns a page of registered applications.
func (s *Service) ListApplications(ctx context.Context, req *presencepb.ListApplicationsRequest) (*presencepb.ListApplicationsResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindApp, scopedaccess.List, scopedaccess.Read); err != nil {
		return nil, trace.Wrap(err)
	}

	// list method scope filters must use identity-based defaults per RFD 0229i
	req.SetScopeFilter(authContext.CheckerContext.ResolveScopeFilter(req.GetScopeFilter()))

	apps, nextToken, err := generic.CollectPageAndCursor(
		stream.FilterMap(
			s.authServer.RangeApplications(ctx, req),
			func(app types.Application) (*types.AppV3, bool) {
				// Filter out apps the user doesn't have access to.
				if _, err := authContext.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
					if err := checker.CheckAccessToRules(&ruleCtx, types.KindApp, scopedaccess.List, scopedaccess.Read); err != nil {
						return err
					}
					return checker.App().CanAccessApp(app)
				}); err == nil {
					appV3, ok := app.(*types.AppV3)
					return appV3, ok
				}
				return nil, false
			},
		),
		int(req.GetPageSize()),
		getCursorForApp,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return presencepb.ListApplicationsResponse_builder{
		Applications:  apps,
		NextPageToken: nextToken,
	}.Build(), nil
}

// DeleteApplication removes the specified application resource.
func (s *Service) DeleteApplication(ctx context.Context, req *presencepb.DeleteApplicationRequest) (*presencepb.DeleteApplicationResponse, error) {
	authContext, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindApp, scopedaccess.Delete); err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure the user has access to the app before deleting it.
	app, err := s.authServer.GetApplication(ctx, presencepb.GetApplicationRequest_builder{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := authContext.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindApp, scopedaccess.Delete); err != nil {
			return err
		}
		return checker.App().CanAccessApp(app)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteApplication(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.AppDelete{
		Metadata: apievents.Metadata{
			Type: events.AppDeleteEvent,
			Code: events.AppDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:  req.GetName(),
			Scope: req.GetScope(),
		},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit app delete event", "error", err)
	}

	return presencepb.DeleteApplicationResponse_builder{}.Build(), nil
}
