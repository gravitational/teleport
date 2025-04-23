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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	presencepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Backend is the subset of the backend resources that the Service modifies.
type Backend interface {
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
	ListRemoteClusters(ctx context.Context, pageSize int, nextToken string) ([]types.RemoteCluster, string, error)
	UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error)
	PatchRemoteCluster(ctx context.Context, name string, updateFn func(rc types.RemoteCluster) (types.RemoteCluster, error)) (types.RemoteCluster, error)

	UpsertReverseTunnel(ctx context.Context, tunnel types.ReverseTunnel) (types.ReverseTunnel, error)
	DeleteReverseTunnel(ctx context.Context, tunnelName string) error
}

type Cache interface {
	ListReverseTunnels(ctx context.Context, pageSize int, nextToken string) ([]types.ReverseTunnel, string, error)
}

type AuthServer interface {
	// DeleteRemoteCluster deletes the remote cluster and associated resources
	// like certificate authorities.
	// We need to invoke this directly on auth.Server.
	DeleteRemoteCluster(ctx context.Context, clusterName string) error
}

// ServiceConfig holds configuration options for
// the presence gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	AuthServer AuthServer
	Backend    Backend
	Cache      Cache
	Logger     *slog.Logger
	Emitter    apievents.Emitter
	Reporter   usagereporter.UsageReporter
	Clock      clockwork.Clock
}

// Service implements the teleport.presence.v1.PresenceService RPC service.
type Service struct {
	presencepb.UnimplementedPresenceServiceServer

	authorizer authz.Authorizer
	authServer AuthServer
	backend    Backend
	cache      Cache
	logger     *slog.Logger
	emitter    apievents.Emitter
	reporter   usagereporter.UsageReporter
	clock      clockwork.Clock
}

// NewService returns a new presence gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("auth server is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "presence.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		authServer: cfg.AuthServer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,

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

	rc, err := s.backend.GetRemoteCluster(ctx, req.Name)
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

	page, nextToken, err := s.backend.ListRemoteClusters(
		ctx, int(req.PageSize), req.PageToken,
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
		rc, err := s.backend.UpdateRemoteCluster(ctx, req.RemoteCluster)
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
	rc, err := s.backend.PatchRemoteCluster(ctx, req.RemoteCluster.GetName(), func(rc types.RemoteCluster) (types.RemoteCluster, error) {
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

	page, nextToken, err := s.cache.ListReverseTunnels(
		ctx, int(req.PageSize), req.PageToken,
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

	res, err := s.backend.UpsertReverseTunnel(ctx, req.ReverseTunnel)
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

	return nil, trace.Wrap(s.backend.DeleteReverseTunnel(ctx, req.Name))
}
